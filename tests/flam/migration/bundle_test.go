package migration

import (
	"errors"
	"path/filepath"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
	"github.com/cjdias/flam-in-go/pkg/flam/migration/loggers"
	"github.com/cjdias/flam-in-go/pkg/flam/migration/loggers/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// entries is a flam.ILogger recording the signaled messages.
type entries struct {
	flam.ILogger

	messages []string
}

func (e *entries) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	e.messages = append(e.messages, level.String()+" "+channel+" "+message+" "+ctx[0].String(standard.ContextFieldMigration))
}

// newApp returns a testutil.App with the config, database, standard
// config, SQLite and migration bundles, the bundles given, the SQLite
// dialect and the connection "main" over a new file, the clock, the
// migrations, and a configuration holding their defaults, then the values.
func newApp(t *testing.T, values map[string]any, migrations []flam.IMigration, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	all := append([]flam.IBundle{
		config.NewBundle(), database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(), migration.NewBundle(),
	}, bundles...)

	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	defaults := map[string]any{
		database.DialectConfigPath + ".main": container.Bag{
			factory.ConfigFieldDriver: sqlite.Driver,
			sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
		},
		database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
	}
	for path, value := range defaults {
		testutil.NoError(t, bag.Set(path, value))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	a := testutil.NewApp()
	testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(ts) }))
	for _, m := range migrations {
		testutil.NoError(t, a.Provide(func() flam.IMigration { return m }, dig.Group(migration.MigrationGroup)))
	}
	for _, b := range all {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.IDatabaseConnectionFactory) error { return f.Close() })
	})
	return a
}

// getMigrator returns the migrator of the id of the app, or the error.
func getMigrator(t *testing.T, a flam.IApp, id string) (flam.IMigrator, error) {
	t.Helper()
	var m flam.IMigrator
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.IMigratorFactory) { m, err = f.Get(id) }))
	return m, err
}

// tableIn reports whether the database of the connection "main" of the app
// has the table of the migration id.
func tableIn(t *testing.T, a flam.IApp, id string) bool {
	t.Helper()
	var has bool
	testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error {
		conn, err := f.Get("main")
		has = err == nil && hasTable(conn, id)
		return err
	}))
	return has
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories and register the creator", func(t *testing.T) {
		b := migration.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.migration")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.migration", "provider/0", "provider/0", "provider/1", "provider/0",
		})
	})

	t.Run("should boot nothing by default", func(t *testing.T) {
		b := migration.NewBundle()
		a := newApp(t, map[string]any{
			migration.MigratorConfigPath + ".main": container.Bag{migration.ConfigFieldConnection: "main"},
		}, []flam.IMigration{table("1")})

		testutil.NoError(t, b.(flam.IBootableBundle).Boot(a))
		testutil.Equal(t, "t1", tableIn(t, a, "1"), false)
	})

	t.Run("should apply every pending migration when booting", func(t *testing.T) {
		b := migration.NewBundle()
		a := newApp(t, map[string]any{
			migration.BootConfigPath:               true,
			migration.MigratorConfigPath + ".main": container.Bag{migration.ConfigFieldConnection: "main"},
		}, []flam.IMigration{table("1"), table("2", "1")})

		testutil.NoError(t, b.(flam.IBootableBundle).Boot(a))
		testutil.Equal(t, "t1", tableIn(t, a, "1"), true)
		testutil.Equal(t, "t2", tableIn(t, a, "2"), true)
	})

	t.Run("should fail to boot a failing migrator", func(t *testing.T) {
		b := migration.NewBundle()
		a := newApp(t, map[string]any{
			migration.BootConfigPath:              true,
			migration.MigratorConfigPath + ".bad": container.Bag{},
		}, nil)
		err := b.(flam.IBootableBundle).Boot(a)
		if !errors.Is(err, migration.ErrInvalidField) {
			t.Fatalf("Boot() = %v, want ErrInvalidField", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		testutil.Context(t, e, map[string]any{migration.ErrorFieldID: "bad"})
	})

	t.Run("should fail to boot a failing migration", func(t *testing.T) {
		b := migration.NewBundle()
		failing := migration.NewMigration("1", "", "main", nil, func(flam.IDatabaseConnection) error { return errBoom }, nil)
		a := newApp(t, map[string]any{
			migration.BootConfigPath:               true,
			migration.MigratorConfigPath + ".main": container.Bag{migration.ConfigFieldConnection: "main"},
		}, []flam.IMigration{failing})
		err := b.(flam.IBootableBundle).Boot(a)
		if !errors.Is(err, errBoom) || !errors.Is(err, migration.ErrUp) {
			t.Fatalf("Boot() = %v, want ErrUp and errBoom", err)
		}
	})

	t.Run("should close the factories", func(t *testing.T) {
		b := migration.NewBundle()
		a := newApp(t, nil, nil)
		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
		_, err := getMigrator(t, a, "main")
		if !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Get() after Close = %v, want factory.ErrClosed", err)
		}
		testutil.NoError(t, a.Invoke(func(f flam.IMigrationLoggerAdaptorFactory) {
			if _, err := f.Get(standard.Id); !errors.Is(err, factory.ErrClosed) {
				t.Fatalf("logger Get() after Close = %v, want factory.ErrClosed", err)
			}
		}))
		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
	})

	t.Run("should migrate in an application", func(t *testing.T) {
		values := container.Bag{}
		for path, value := range map[string]any{
			migration.BootConfigPath: true,
			database.DialectConfigPath + ".main": container.Bag{
				factory.ConfigFieldDriver: sqlite.Driver,
				sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
			},
			database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
			migration.MigratorConfigPath + ".main":  container.Bag{migration.ConfigFieldConnection: "main"},
		} {
			testutil.NoError(t, values.Set(path, value))
		}
		a := app.New(values)
		for _, b := range []flam.IBundle{
			database.NewBundle(), configs.NewBundle(), dialects.NewBundle(), migration.NewBundle(), loggers.NewBundle(),
		} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		testutil.NoError(t, a.Provide(func() flam.IMigration { return table("1") }, dig.Group(migration.MigrationGroup)))
		testutil.NoError(t, a.Boot())

		testutil.NoError(t, a.Invoke(func(f flam.IMigratorFactory) error {
			m, err := f.Get("main")
			if err != nil {
				return err
			}
			infos, err := m.List()
			if err == nil && infos[0].InstalledAt == nil {
				t.Fatal("the migration was not applied when booting")
			}
			return err
		}))
		testutil.NoError(t, a.Close())
	})
}

func Test_MigratorCreator(t *testing.T) {
	migrator := func(fields container.Bag) map[string]any {
		return map[string]any{migration.MigratorConfigPath + ".main": fields}
	}

	t.Run("should create the migrator of the group of its id", func(t *testing.T) {
		other := migration.NewMigration("9", "", "other", nil, nil, nil)
		a := newApp(t, migrator(container.Bag{migration.ConfigFieldConnection: "main"}), []flam.IMigration{table("1"), other})
		m, err := getMigrator(t, a, "main")
		testutil.NoError(t, err)

		testutil.Equal(t, "Id()", m.Id(), "main")
		infos, err := m.List()
		testutil.NoError(t, err)
		testutil.Equal(t, "List()", ids(infos), []string{"1"})
	})

	t.Run("should take the group, the connection and the logger of the configuration", func(t *testing.T) {
		logged := &entries{}
		other := migration.NewMigration("9", "", "other", nil, nil, nil)
		a := newApp(t, map[string]any{
			migration.DefaultConnectionConfigPath: "main",
			migration.MigratorConfigPath + ".main": container.Bag{
				factory.ConfigFieldDriver: migration.Driver, migration.ConfigFieldGroup: "other",
			},
		}, []flam.IMigration{table("1"), other}, standard.NewBundle())
		testutil.NoError(t, a.Provide(func() flam.ILogger { return logged }))
		m, err := getMigrator(t, a, "main")
		testutil.NoError(t, err)

		_, err = m.UpAll()
		testutil.NoError(t, err)
		testutil.Equal(t, "logged", logged.messages, []string{
			"info flam.migration migration up started 9", "info flam.migration migration up done 9",
		})
	})

	t.Run("should leave the migrator without logger for an empty id", func(t *testing.T) {
		a := newApp(t, migrator(container.Bag{migration.ConfigFieldConnection: "main", migration.ConfigFieldLogger: ""}),
			[]flam.IMigration{table("1")}, standard.NewBundle())
		m, err := getMigrator(t, a, "main")
		testutil.NoError(t, err)
		_, err = m.UpAll()
		testutil.NoError(t, err)
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, fields := range map[string]container.Bag{
			migration.ConfigFieldConnection: {},
			migration.ConfigFieldGroup:      {migration.ConfigFieldConnection: "main", migration.ConfigFieldGroup: ""},
			migration.ConfigFieldLogger:     {migration.ConfigFieldConnection: "main", migration.ConfigFieldLogger: 1},
		} {
			_, err := getMigrator(t, newApp(t, migrator(fields), nil), "main")
			testutil.Error(t, errors.Unwrap(err), migration.ErrInvalidField, "unable to create migrator: invalid migrator field",
				map[string]any{migration.ErrorFieldID: "main", migration.ErrorFieldField: field})
		}
	})

	t.Run("should fail with an unknown connection or logger", func(t *testing.T) {
		_, err := getMigrator(t, newApp(t, migrator(container.Bag{migration.ConfigFieldConnection: "nope"}), nil), "main")
		if !errors.Is(err, migration.ErrConnection) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrConnection", err)
		}
		_, err = getMigrator(t, newApp(t, migrator(container.Bag{
			migration.ConfigFieldConnection: "main", migration.ConfigFieldLogger: "nope",
		}), nil), "main")
		if !errors.Is(err, migration.ErrLogger) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrLogger", err)
		}
	})

	t.Run("should fail with a nil migration", func(t *testing.T) {
		_, err := getMigrator(t, newApp(t, migrator(container.Bag{migration.ConfigFieldConnection: "main"}),
			[]flam.IMigration{nil}), "main")
		if !errors.Is(err, migration.ErrNilMigration) {
			t.Fatalf("Get() = %v, want ErrNilMigration", err)
		}
	})

	t.Run("should fail with migrations that cannot be ordered", func(t *testing.T) {
		_, err := getMigrator(t, newApp(t, migrator(container.Bag{migration.ConfigFieldConnection: "main"}),
			[]flam.IMigration{table("1", "1")}), "main")
		if !errors.Is(err, migration.ErrDependencyCycle) {
			t.Fatalf("Get() = %v, want ErrDependencyCycle", err)
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := getMigrator(t, newApp(t, migrator(container.Bag{factory.ConfigFieldDriver: "other"}), nil), "main")
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
