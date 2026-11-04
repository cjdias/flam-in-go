package database

import (
	"errors"
	"path/filepath"
	"testing"

	"go.uber.org/dig"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newApp returns a testutil.App with the config bundle, the bundles given,
// and a configuration holding their defaults, then the values, at paths.
func newApp(t *testing.T, values map[string]any, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	all := append([]flam.IBundle{config.NewBundle()}, bundles...)

	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	for _, b := range all {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// fullApp returns an app with every database bundle, the SQLite dialect
// "main" over a new file, and the values.
func fullApp(t *testing.T, values map[string]any) *testutil.App {
	t.Helper()
	all := map[string]any{
		database.DialectConfigPath + ".main": container.Bag{
			factory.ConfigFieldDriver: sqlite.Driver,
			sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
		},
	}
	for path, value := range values {
		all[path] = value
	}
	return newApp(t, all, database.NewBundle(), standard.NewBundle(), sqlite.NewBundle())
}

// connections returns the connection factory of the app.
func connections(t *testing.T, app flam.IApp) flam.IDatabaseConnectionFactory {
	t.Helper()
	var f flam.IDatabaseConnectionFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IDatabaseConnectionFactory) { f = provided }))
	return f
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ConfigConfigPath", database.ConfigConfigPath, "flam.database.configs")
	testutil.Equal(t, "ConfigCreatorGroup", database.ConfigCreatorGroup, "flam.database.configs.creator")
	testutil.Equal(t, "DialectConfigPath", database.DialectConfigPath, "flam.database.dialects")
	testutil.Equal(t, "DialectCreatorGroup", database.DialectCreatorGroup, "flam.database.dialects.creator")
	testutil.Equal(t, "ConnectionConfigPath", database.ConnectionConfigPath, "flam.database.connections")
	testutil.Equal(t, "ConnectionCreatorGroup", database.ConnectionCreatorGroup, "flam.database.connections.creator")
	testutil.Equal(t, "Driver", database.Driver, "flam.database.connections.driver.gorm")
	testutil.Equal(t, "ConfigFieldDialect", database.ConfigFieldDialect, "dialect")
	testutil.Equal(t, "ConfigFieldConfig", database.ConfigFieldConfig, "config")
	testutil.Equal(t, "DefaultDialectConfigPath", database.DefaultDialectConfigPath, "flam.database.defaults.connections.dialect")
	testutil.Equal(t, "DefaultConfigConfigPath", database.DefaultConfigConfigPath, "flam.database.defaults.connections.config")
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories and register the creator", func(t *testing.T) {
		b := database.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.database")

		app := newApp(t, nil, b)
		testutil.Equal(t, "calls", app.Calls()[6:], []string{
			"bundle:flam.database", "provider/0", "provider/0", "provider/0", "provider/1", "invoker/0",
		})
		var creators []flam.IDatabaseConnectionCreator
		testutil.NoError(t, app.Invoke(func(args struct {
			dig.In
			Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
		}) {
			creators = args.Creators
		}))
		testutil.Equal(t, "creators", len(creators), 1)
	})

	t.Run("should open the configured connections", func(t *testing.T) {
		app := fullApp(t, map[string]any{
			database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
		})
		f := connections(t, app)
		testutil.Equal(t, "Available()", f.Available(), []string{"main"})

		conn, err := f.Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.NoError(t, conn.AutoMigrate(&item{}))
		testutil.NoError(t, conn.Create(&item{Name: "a"}).Error)
		var got item
		testutil.NoError(t, conn.First(&got).Error)
		testutil.Equal(t, "Name", got.Name, "a")
		testutil.Equal(t, "CreatedAt set", got.CreatedAt.IsZero(), false)
	})

	t.Run("should open the dialect and configuration of the defaults", func(t *testing.T) {
		app := fullApp(t, map[string]any{
			database.DefaultDialectConfigPath:  "main",
			database.ConfigConfigPath + ".dry": container.Bag{factory.ConfigFieldDriver: standard.Driver, standard.ConfigFieldDryRun: true},
			database.DefaultConfigConfigPath:   "dry",
			database.ConnectionConfigPath + ".main": container.Bag{
				factory.ConfigFieldDriver: database.Driver,
			},
		})
		conn, err := connections(t, app).Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "DryRun", conn.Gorm().DryRun, true)
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		app := fullApp(t, map[string]any{
			database.ConnectionConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: "other"},
		})
		_, err := connections(t, app).Get("main")
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})

	t.Run("should close the factories, connections first", func(t *testing.T) {
		b := database.NewBundle()
		app := fullApp(t, map[string]any{
			database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
		})
		conn, err := connections(t, app).Get("main")
		testutil.NoError(t, err)
		pool, err := conn.DB()
		testutil.NoError(t, err)

		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		testutil.Equal(t, "Ping() after Close", pool.Ping() != nil, true)
		var dialects flam.IDatabaseDialectFactory
		var configs flam.IDatabaseConfigFactory
		testutil.NoError(t, app.Invoke(func(d flam.IDatabaseDialectFactory, c flam.IDatabaseConfigFactory) {
			dialects, configs = d, c
		}))
		if _, err := dialects.Get("main"); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("dialects Get() after Close = %v, want factory.ErrClosed", err)
		}
		if _, err := configs.Get(standard.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("configs Get() after Close = %v, want factory.ErrClosed", err)
		}
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
	})

	t.Run("should join the errors of closing", func(t *testing.T) {
		b := database.NewBundle()
		app := newApp(t, nil, b)
		db := openDB(t)
		pool, _ := db.DB()
		db.ConnPool = failingPool{pool}
		db.Statement.ConnPool = db.ConnPool
		conn, err := database.NewConnection("bad", db)
		testutil.NoError(t, err)
		testutil.NoError(t, connections(t, app).Store("bad", conn))

		if err := b.(flam.IClosableBundle).Close(app); !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
	})
}

func Test_AggregateBundles(t *testing.T) {
	t.Run("should register every configuration driver", func(t *testing.T) {
		b := configs.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.database.configs")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.database.configs", "bundle:flam.database.configs.standard", "provider/1",
		})
	})

	t.Run("should register every dialect driver", func(t *testing.T) {
		b := dialects.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.database.dialects")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.database.dialects",
			"bundle:flam.database.dialects.sqlite", "provider/1",
			"bundle:flam.database.dialects.mysql", "provider/1",
			"bundle:flam.database.dialects.postgres", "provider/1",
		})
	})

	t.Run("should open a connection in an application", func(t *testing.T) {
		values := container.Bag{}
		testutil.NoError(t, values.Set(database.DialectConfigPath+".main", container.Bag{
			factory.ConfigFieldDriver: sqlite.Driver,
			sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
		}))
		testutil.NoError(t, values.Set(database.ConnectionConfigPath+".main", container.Bag{
			database.ConfigFieldDialect: "main",
		}))
		a := app.New(values)
		for _, b := range []flam.IBundle{database.NewBundle(), configs.NewBundle(), dialects.NewBundle()} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		testutil.NoError(t, a.Boot())

		var pool interface{ Ping() error }
		testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error {
			conn, err := f.Get("main")
			if err != nil {
				return err
			}
			pool, err = conn.DB()
			if err != nil {
				return err
			}
			return conn.AutoMigrate(&item{})
		}))
		testutil.NoError(t, a.Close())
		testutil.Equal(t, "Ping() after Close", pool.Ping() != nil, true)
	})
}

func Test_ConnectionCreator(t *testing.T) {
	get := func(t *testing.T, values map[string]any) error {
		t.Helper()
		_, err := connections(t, fullApp(t, values)).Get("main")
		return err
	}
	conn := func(fields container.Bag) map[string]any {
		return map[string]any{database.ConnectionConfigPath + ".main": fields}
	}

	t.Run("should fail without dialect", func(t *testing.T) {
		testutil.Error(t, unwrapResource(get(t, conn(container.Bag{}))), database.ErrInvalidField,
			"unable to create database connection: invalid database connection field", map[string]any{
				database.ErrorFieldID: "main", database.ErrorFieldField: database.ConfigFieldDialect,
			})
	})

	t.Run("should fail with a dialect that is not an id", func(t *testing.T) {
		testutil.Error(t, unwrapResource(get(t, conn(container.Bag{database.ConfigFieldDialect: 3}))), database.ErrInvalidField,
			"unable to create database connection: invalid database connection field", map[string]any{
				database.ErrorFieldID: "main", database.ErrorFieldField: database.ConfigFieldDialect,
			})
	})

	t.Run("should fail with an empty configuration id", func(t *testing.T) {
		testutil.Error(t, unwrapResource(get(t, conn(container.Bag{database.ConfigFieldDialect: "main", database.ConfigFieldConfig: ""}))),
			database.ErrInvalidField, "unable to create database connection: invalid database connection field", map[string]any{
				database.ErrorFieldID: "main", database.ErrorFieldField: database.ConfigFieldConfig,
			})
	})

	t.Run("should fail with an unknown dialect", func(t *testing.T) {
		err := unwrapResource(get(t, conn(container.Bag{database.ConfigFieldDialect: "nope"})))
		if !errors.Is(err, database.ErrDialect) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrDialect and factory.ErrUnknownID", err)
		}
		testutil.Context(t, asError(t, err), map[string]any{database.ErrorFieldID: "main", database.ErrorFieldDialect: "nope"})
	})

	t.Run("should fail with an unknown configuration", func(t *testing.T) {
		err := unwrapResource(get(t, conn(container.Bag{database.ConfigFieldDialect: "main", database.ConfigFieldConfig: "nope"})))
		if !errors.Is(err, database.ErrConfig) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrConfig and factory.ErrUnknownID", err)
		}
		testutil.Context(t, asError(t, err), map[string]any{database.ErrorFieldID: "main", database.ErrorFieldConfig: "nope"})
	})

	t.Run("should fail when the connection cannot be opened", func(t *testing.T) {
		err := unwrapResource(get(t, map[string]any{
			database.DialectConfigPath + ".bad": container.Bag{
				factory.ConfigFieldDriver: sqlite.Driver,
				sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "missing", "dir", "app.db"),
			},
			database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "bad"},
		}))
		if !errors.Is(err, database.ErrOpen) {
			t.Fatalf("Get() = %v, want ErrOpen", err)
		}
		testutil.Context(t, asError(t, err), map[string]any{
			database.ErrorFieldID: "main", database.ErrorFieldDialect: "bad", database.ErrorFieldConfig: standard.Id,
		})
	})
}

// namedPlugin is a gorm plugin of a name, doing nothing.
type namedPlugin string

func (p namedPlugin) Name() string              { return string(p) }
func (p namedPlugin) Initialize(*gorm.DB) error { return nil }

func Test_ConnectionCreator_SharedConfig(t *testing.T) {
	t.Run("should not share the plugins of the configuration", func(t *testing.T) {
		app := fullApp(t, map[string]any{
			database.ConnectionConfigPath + ".one": container.Bag{database.ConfigFieldDialect: "main", database.ConfigFieldConfig: "plug"},
			database.ConnectionConfigPath + ".two": container.Bag{database.ConfigFieldDialect: "main", database.ConfigFieldConfig: "plug"},
		})
		plugins := map[string]gorm.Plugin{}
		testutil.NoError(t, app.Invoke(func(f flam.IDatabaseConfigFactory) error {
			return f.Store("plug", flam.NewDatabaseConfig("plug", gorm.Config{Plugins: plugins}))
		}))
		one, err := connections(t, app).Get("one")
		testutil.NoError(t, err)
		two, err := connections(t, app).Get("two")
		testutil.NoError(t, err)

		testutil.NoError(t, one.Use(namedPlugin("p")))
		testutil.Equal(t, "configuration plugins", len(plugins), 0)
		testutil.Equal(t, "other connection plugins", len(two.Gorm().Config.Plugins), 0)
		testutil.Equal(t, "connection plugins", len(one.Gorm().Config.Plugins), 1)
	})
}

func Test_NewDialect(t *testing.T) {
	t.Run("should fail with a nil dialector", func(t *testing.T) {
		for _, open := range []func() gorm.Dialector{
			nil,
			func() gorm.Dialector { return nil },
			func() gorm.Dialector { return (*gormsqlite.Dialector)(nil) },
		} {
			_, err := database.NewDialect("main", open)
			testutil.Error(t, err, database.ErrNilDialector, "unable to create database dialect: nil database dialector", map[string]any{
				database.ErrorFieldID: "main",
			})
		}
	})

	t.Run("should hold the id and give a new dialector of the driver each time", func(t *testing.T) {
		calls := 0
		d, err := database.NewDialect("main", func() gorm.Dialector {
			calls++
			return gormsqlite.Open(":memory:")
		})
		testutil.NoError(t, err)

		testutil.Equal(t, "Id()", d.Id(), "main")
		testutil.Equal(t, "Name()", d.Name(), "sqlite")
		first, second := d.Gorm(), d.Gorm()
		testutil.Equal(t, "Gorm()", first, gormsqlite.Open(":memory:"))
		testutil.Equal(t, "new each time", first != second, true)
		testutil.Equal(t, "calls", calls, 3)
	})
}

// unwrapResource returns the error the factory wraps with the resource id.
func unwrapResource(err error) error {
	if u := errors.Unwrap(err); u != nil {
		return u
	}
	return err
}

// asError returns the error as a *flam.Error.
func asError(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v is not a *flam.Error", err)
	}
	return e
}
