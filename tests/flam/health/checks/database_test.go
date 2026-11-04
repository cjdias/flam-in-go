package checks

import (
	"context"
	"database/sql"
	"testing"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	databasecheck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/database"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// storeDatabase stores in the connection factory of the app the
// connection of the id over a new in-memory SQLite database, and returns
// its pool, closed at the end of the test.
func storeDatabase(t *testing.T, a flam.IApp, id string) *sql.DB {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open("file::memory:"), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	pool, err := db.DB()
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = pool.Close() })

	conn, err := database.NewConnection(id, db)
	testutil.NoError(t, err)
	testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error { return f.Store(id, conn) }))
	return pool
}

// poolessConnection is a flam.IDatabaseConnection whose pool cannot be
// taken.
type poolessConnection struct {
	flam.IDatabaseConnection
}

func (poolessConnection) Id() string           { return "pooless" }
func (poolessConnection) DB() (*sql.DB, error) { return nil, errBoom }
func (poolessConnection) Close() error         { return nil }

func Test_Database_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", databasecheck.Driver, "flam.health.checks.driver.database")
	testutil.Equal(t, "ConfigFieldConnection", databasecheck.ConfigFieldConnection, "connection")
	testutil.Equal(t, "ErrorFieldID", databasecheck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", databasecheck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldConnection", databasecheck.ErrorFieldConnection, "connection")
}

func Test_Database_Bundle(t *testing.T) {
	b := databasecheck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.database")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("main", databasecheck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("main", "other", nil)), false)
}

func Test_Database_Create(t *testing.T) {
	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: "x"}, health.ConfigFieldKind},
			{container.Bag{databasecheck.ConfigFieldConnection: 3}, databasecheck.ConfigFieldConnection},
			{container.Bag{databasecheck.ConfigFieldConnection: ""}, databasecheck.ConfigFieldConnection},
		} {
			a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
			check, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, tc.fields))
			testutil.Error(t, err, databasecheck.ErrInvalidField,
				"unable to create database health check: invalid database health check field",
				map[string]any{databasecheck.ErrorFieldID: "main", databasecheck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should fail without connection factory", func(t *testing.T) {
		a := newApp(t, nil, databasecheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, nil))
		testutil.Error(t, err, databasecheck.ErrNoFactory,
			"unable to create database health check: no database connection factory",
			map[string]any{databasecheck.ErrorFieldID: "main"})
	})

	t.Run("should fail for an unknown connection", func(t *testing.T) {
		a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, container.Bag{
			databasecheck.ConfigFieldConnection: "other",
		}))
		testutil.Error(t, err, databasecheck.ErrUnknownConnection,
			"unable to create database health check: unknown database connection",
			map[string]any{databasecheck.ErrorFieldID: "main", databasecheck.ErrorFieldConnection: "other"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
		storeDatabase(t, a, "main")
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("main", databasecheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "main")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("main", databasecheck.Driver, container.Bag{health.ConfigFieldKind: "both"}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthBoth)
	})

	t.Run("should take the default kind of the configuration", func(t *testing.T) {
		a := newApp(t, map[string]any{health.DefaultKindConfigPath: "liveness"},
			database.NewBundle(), databasecheck.NewBundle())
		storeDatabase(t, a, "main")

		check, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthLiveness)
	})
}

func Test_Database_Check(t *testing.T) {
	t.Run("should ping the database of the connection", func(t *testing.T) {
		a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
		storeDatabase(t, a, "db")

		check, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, container.Bag{
			databasecheck.ConfigFieldConnection: "db",
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should fail when the database does not answer", func(t *testing.T) {
		a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
		pool := storeDatabase(t, a, "main")
		check, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, pool.Close())

		testutil.Error(t, check.Check(context.Background()), databasecheck.ErrPing,
			"database health check failed: unable to ping database: sql: database is closed",
			map[string]any{databasecheck.ErrorFieldID: "main", databasecheck.ErrorFieldConnection: "main"})
	})

	t.Run("should fail when the pool of the connection cannot be taken", func(t *testing.T) {
		a := newApp(t, nil, database.NewBundle(), databasecheck.NewBundle())
		testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error {
			return f.Store("pooless", poolessConnection{})
		}))
		check, err := creatorOf(t, a).Create(checkBag("pooless", databasecheck.Driver, nil))
		testutil.NoError(t, err)

		testutil.Error(t, check.Check(context.Background()), databasecheck.ErrConnection,
			"database health check failed: unable to get database connection: boom",
			map[string]any{databasecheck.ErrorFieldID: "pooless", databasecheck.ErrorFieldConnection: "pooless"})
	})

	t.Run("should fail when the connection cannot be created", func(t *testing.T) {
		a := newApp(t, map[string]any{
			database.ConnectionConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "unknown"},
		}, database.NewBundle(), databasecheck.NewBundle())
		check, err := creatorOf(t, a).Create(checkBag("main", databasecheck.Driver, container.Bag{
			databasecheck.ConfigFieldConnection: "bad",
		}))
		testutil.NoError(t, err)

		wantErr(t, check.Check(context.Background()),
			map[string]any{databasecheck.ErrorFieldID: "main", databasecheck.ErrorFieldConnection: "bad"},
			databasecheck.ErrConnection, factory.ErrNoCreator)
	})

	t.Run("should be run by the readiness probe of the registry", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, map[string]any{
			health.CheckConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: databasecheck.Driver},
		}, database.NewBundle(), b, databasecheck.NewBundle())
		storeDatabase(t, a, "main")
		testutil.NoError(t, b.(flam.IBootableBundle).Boot(a))

		testutil.NoError(t, a.Invoke(func(r flam.IHealthRegistry) {
			testutil.Equal(t, "Readiness()", r.Readiness(context.Background()), flam.HealthReport{
				Status: flam.HealthStatusUp,
				Checks: []flam.HealthCheckReport{{Id: "main", Status: flam.HealthStatusUp}},
			})
			testutil.Equal(t, "Liveness()", r.Liveness(context.Background()).Checks, []flam.HealthCheckReport{})
		}))
	})
}
