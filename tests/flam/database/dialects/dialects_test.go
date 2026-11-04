package dialects

import (
	"errors"
	"testing"

	sqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	gormsqlite "gorm.io/driver/sqlite"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/mysql"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/postgres"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// create returns the dialect "main" of the driver and the fields, created
// by the factory of an app with the dialect bundles, and the values.
func create(t *testing.T, driver string, fields container.Bag, values map[string]any) (flam.IDatabaseDialect, error) {
	t.Helper()
	bundles := []flam.IBundle{
		config.NewBundle(), database.NewBundle(), sqlite.NewBundle(), mysql.NewBundle(), postgres.NewBundle(),
	}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	fields[factory.ConfigFieldDriver] = driver
	testutil.NoError(t, bag.Set(database.DialectConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var d flam.IDatabaseDialect
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IDatabaseDialectFactory) {
		d, err = f.Get("main")
	}))
	return d, err
}

// assertInvalid asserts that the creation failed with the sentinel for the
// field.
func assertInvalid(t *testing.T, err, sentinel error, field string) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(errors.Unwrap(err), &e) {
		t.Fatalf("%s: Get() = %v, want %v", field, err, sentinel)
	}
	testutil.Context(t, e, map[string]any{"id": "main", "field": field})
}

func Test_Bundles(t *testing.T) {
	cases := []struct {
		bundle   flam.IBundle
		id       string
		defaults map[string]any
	}{
		{sqlite.NewBundle(), "flam.database.dialects.sqlite", map[string]any{
			sqlite.DefaultPathConfigPath: ":memory:",
		}},
		{mysql.NewBundle(), "flam.database.dialects.mysql", map[string]any{
			mysql.DefaultProtocolConfigPath: "tcp", mysql.DefaultHostConfigPath: "127.0.0.1",
			mysql.DefaultPortConfigPath: 3306, mysql.DefaultSchemaConfigPath: "flam",
			mysql.DefaultUsernameConfigPath: "root", mysql.DefaultPasswordConfigPath: "",
		}},
		{postgres.NewBundle(), "flam.database.dialects.postgres", map[string]any{
			postgres.DefaultHostConfigPath: "127.0.0.1", postgres.DefaultPortConfigPath: 5432,
			postgres.DefaultDatabaseConfigPath: "flam", postgres.DefaultUsernameConfigPath: "postgres",
			postgres.DefaultPasswordConfigPath: "",
		}},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			testutil.Equal(t, "Id()", c.bundle.Id(), c.id)

			bag := container.Bag{}
			testutil.NoError(t, c.bundle.(flam.IConfigurableBundle).Config(bag))
			for path, want := range c.defaults {
				testutil.Equal(t, path, bag.Get(path), want)
			}
			if err := c.bundle.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
				t.Fatal("Config() did not fail")
			}

			app := testutil.NewApp()
			testutil.NoError(t, app.RegisterBundle(c.bundle))
			testutil.Equal(t, "calls", app.Calls(), []string{"bundle:" + c.id, "provider/1"})
		})
	}

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := create(t, "other", container.Bag{}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Sqlite(t *testing.T) {
	dsn := func(t *testing.T, fields container.Bag, values map[string]any) string {
		t.Helper()
		d, err := create(t, sqlite.Driver, fields, values)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", d.Id(), "main")
		return d.Gorm().(*gormsqlite.Dialector).DSN
	}

	t.Run("should open the default path", func(t *testing.T) {
		testutil.Equal(t, "DSN", dsn(t, container.Bag{}, nil), ":memory:")
	})

	t.Run("should open the path with its parameters, in name order", func(t *testing.T) {
		testutil.Equal(t, "DSN", dsn(t, container.Bag{
			sqlite.ConfigFieldPath:   "app.db",
			sqlite.ConfigFieldParams: container.Bag{"_fk": true, "a b": "x&y"},
		}, nil), "app.db?_fk=true&a+b=x%26y")
	})

	t.Run("should append the parameters to a path holding a query", func(t *testing.T) {
		testutil.Equal(t, "DSN", dsn(t, container.Bag{
			sqlite.ConfigFieldPath:   "file::memory:?cache=shared",
			sqlite.ConfigFieldParams: map[string]any{"mode": "memory"},
		}, nil), "file::memory:?cache=shared&mode=memory")
	})

	t.Run("should take the parameters of a map of strings", func(t *testing.T) {
		testutil.Equal(t, "DSN", dsn(t, container.Bag{
			sqlite.ConfigFieldPath:   "app.db",
			sqlite.ConfigFieldParams: map[string]string{"mode": "ro"},
		}, nil), "app.db?mode=ro")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		testutil.Equal(t, "DSN", dsn(t, container.Bag{}, map[string]any{
			sqlite.DefaultPathConfigPath:   "default.db",
			sqlite.DefaultParamsConfigPath: container.Bag{"mode": "ro"},
		}), "default.db?mode=ro")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			sqlite.ConfigFieldPath:   "",
			sqlite.ConfigFieldParams: "a=b",
		} {
			_, err := create(t, sqlite.Driver, container.Bag{field: val}, nil)
			assertInvalid(t, err, sqlite.ErrInvalidField, field)
		}
	})

	t.Run("should fail with nested parameters", func(t *testing.T) {
		for _, val := range []any{container.Bag{"a": container.Bag{"b": 1}}, map[string]any{"a": []any{1}}, map[string]any{"a": nil}} {
			_, err := create(t, sqlite.Driver, container.Bag{sqlite.ConfigFieldParams: val}, nil)
			assertInvalid(t, err, sqlite.ErrInvalidField, sqlite.ConfigFieldParams)
		}
	})
}

func Test_Mysql(t *testing.T) {
	parse := func(t *testing.T, fields container.Bag) *sqldriver.Config {
		t.Helper()
		d, err := create(t, mysql.Driver, fields, nil)
		testutil.NoError(t, err)
		cfg, err := sqldriver.ParseDSN(d.Gorm().(*gormmysql.Dialector).Config.DSN)
		testutil.NoError(t, err)
		return cfg
	}

	t.Run("should connect with the defaults, parsing times", func(t *testing.T) {
		cfg := parse(t, container.Bag{})
		testutil.Equal(t, "Net", cfg.Net, "tcp")
		testutil.Equal(t, "Addr", cfg.Addr, "127.0.0.1:3306")
		testutil.Equal(t, "DBName", cfg.DBName, "flam")
		testutil.Equal(t, "User", cfg.User, "root")
		testutil.Equal(t, "Passwd", cfg.Passwd, "")
		testutil.Equal(t, "ParseTime", cfg.ParseTime, true)
	})

	t.Run("should keep a password holding the separators", func(t *testing.T) {
		cfg := parse(t, container.Bag{
			mysql.ConfigFieldHost:     "::1",
			mysql.ConfigFieldPort:     "3307",
			mysql.ConfigFieldSchema:   "app",
			mysql.ConfigFieldUsername: "admin",
			mysql.ConfigFieldPassword: "p@ss:w/rd?&",
			mysql.ConfigFieldParams:   container.Bag{"charset": "utf8mb4", "parseTime": false},
		})
		testutil.Equal(t, "Addr", cfg.Addr, "[::1]:3307")
		testutil.Equal(t, "DBName", cfg.DBName, "app")
		testutil.Equal(t, "User", cfg.User, "admin")
		testutil.Equal(t, "Passwd", cfg.Passwd, "p@ss:w/rd?&")
		testutil.Equal(t, "Params", cfg.Params, map[string]string{"charset": "utf8mb4"})
		testutil.Equal(t, "ParseTime", cfg.ParseTime, false)
	})

	t.Run("should connect to the socket of the unix protocol", func(t *testing.T) {
		cfg := parse(t, container.Bag{mysql.ConfigFieldProtocol: "unix", mysql.ConfigFieldHost: "/tmp/mysql.sock"})
		testutil.Equal(t, "Net", cfg.Net, "unix")
		testutil.Equal(t, "Addr", cfg.Addr, "/tmp/mysql.sock")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			mysql.ConfigFieldProtocol: "",
			mysql.ConfigFieldHost:     3,
			mysql.ConfigFieldPort:     0,
			mysql.ConfigFieldSchema:   "",
			mysql.ConfigFieldUsername: "",
			mysql.ConfigFieldPassword: 1234,
			mysql.ConfigFieldParams:   []any{"a"},
		} {
			_, err := create(t, mysql.Driver, container.Bag{field: val}, nil)
			assertInvalid(t, err, mysql.ErrInvalidField, field)
		}
	})
}

func Test_Postgres(t *testing.T) {
	parse := func(t *testing.T, fields container.Bag) *pgconn.Config {
		t.Helper()
		d, err := create(t, postgres.Driver, fields, nil)
		testutil.NoError(t, err)
		cfg, err := pgconn.ParseConfig(d.Gorm().(*gormpostgres.Dialector).Config.DSN)
		testutil.NoError(t, err)
		return cfg
	}

	t.Run("should connect with the defaults", func(t *testing.T) {
		cfg := parse(t, container.Bag{})
		testutil.Equal(t, "Host", cfg.Host, "127.0.0.1")
		testutil.Equal(t, "Port", cfg.Port, uint16(5432))
		testutil.Equal(t, "Database", cfg.Database, "flam")
		testutil.Equal(t, "User", cfg.User, "postgres")
		testutil.Equal(t, "Password", cfg.Password, "")
	})

	t.Run("should keep values holding spaces, quotes and backslashes", func(t *testing.T) {
		cfg := parse(t, container.Bag{
			postgres.ConfigFieldHost:     "db.local",
			postgres.ConfigFieldPort:     6543.0,
			postgres.ConfigFieldDatabase: "my app",
			postgres.ConfigFieldUsername: "o'brien",
			postgres.ConfigFieldPassword: `p a\ss'`,
			postgres.ConfigFieldParams:   container.Bag{"application_name": "flam app", "host": "ignored"},
		})
		testutil.Equal(t, "Host", cfg.Host, "db.local")
		testutil.Equal(t, "Port", cfg.Port, uint16(6543))
		testutil.Equal(t, "Database", cfg.Database, "my app")
		testutil.Equal(t, "User", cfg.User, "o'brien")
		testutil.Equal(t, "Password", cfg.Password, `p a\ss'`)
		testutil.Equal(t, "application_name", cfg.RuntimeParams["application_name"], "flam app")
	})

	t.Run("should give the same string for the same configuration", func(t *testing.T) {
		fields := func() container.Bag {
			return container.Bag{postgres.ConfigFieldParams: container.Bag{"b": 1, "a": 2, "c": 3}}
		}
		first, err := create(t, postgres.Driver, fields(), nil)
		testutil.NoError(t, err)
		for range 5 {
			again, err := create(t, postgres.Driver, fields(), nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "DSN", again.Gorm().(*gormpostgres.Dialector).Config.DSN,
				first.Gorm().(*gormpostgres.Dialector).Config.DSN)
		}
		testutil.Equal(t, "DSN", first.Gorm().(*gormpostgres.Dialector).Config.DSN,
			"a='2' b='1' c='3' dbname='flam' host='127.0.0.1' password='' port='5432' user='postgres'")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			postgres.ConfigFieldHost:     "",
			postgres.ConfigFieldPort:     70000,
			postgres.ConfigFieldDatabase: "",
			postgres.ConfigFieldUsername: true,
			postgres.ConfigFieldPassword: nil,
			postgres.ConfigFieldParams:   "sslmode=disable",
		} {
			_, err := create(t, postgres.Driver, container.Bag{field: val}, nil)
			assertInvalid(t, err, postgres.ErrInvalidField, field)
		}
	})

	t.Run("should read the ports of every kind", func(t *testing.T) {
		for _, val := range []any{int8(80), uint(80), float32(80), " 80 "} {
			testutil.Equal(t, "Port", parse(t, container.Bag{postgres.ConfigFieldPort: val}).Port, uint16(80))
		}
		for _, val := range []any{"x", 1.5, -1, uint(0), struct{}{}} {
			_, err := create(t, postgres.Driver, container.Bag{postgres.ConfigFieldPort: val}, nil)
			assertInvalid(t, err, postgres.ErrInvalidField, postgres.ConfigFieldPort)
		}
	})
}
