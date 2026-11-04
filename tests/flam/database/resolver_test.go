package database

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// seed creates the SQLite database file of the path, with the item table
// holding an item of each name.
func seed(t *testing.T, path string, names ...string) {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	testutil.NoError(t, db.AutoMigrate(&item{}))
	for _, name := range names {
		testutil.NoError(t, db.Create(&item{Name: name}).Error)
	}
	pool, err := db.DB()
	testutil.NoError(t, err)
	testutil.NoError(t, pool.Close())
}

// names returns the names of the items of the SQLite database file of the
// path, in id order.
func names(t *testing.T, path string) []string {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	var got []string
	testutil.NoError(t, db.Model(&item{}).Order("id").Pluck("name", &got).Error)
	pool, err := db.DB()
	testutil.NoError(t, err)
	testutil.NoError(t, pool.Close())
	return got
}

// sqliteDialect returns the configuration of a SQLite dialect over the
// file of the path.
func sqliteDialect(path string) container.Bag {
	return container.Bag{factory.ConfigFieldDriver: sqlite.Driver, sqlite.ConfigFieldPath: path}
}

// resolverFiles are the files of the dialects "main", "replica" and
// "source" of resolverApp.
type resolverFiles struct {
	main    string
	replica string
	source  string
}

// resolverApp returns an app with every database bundle, the dialects
// "main", "replica" and "source", over files seeded with the item table,
// holding "main", "replica" and "source" respectively, the connection
// "main" of the fields, and the values.
func resolverApp(t *testing.T, fields container.Bag, values map[string]any) (*testutil.App, resolverFiles) {
	t.Helper()
	dir := t.TempDir()
	files := resolverFiles{
		main:    filepath.Join(dir, "main.db"),
		replica: filepath.Join(dir, "replica.db"),
		source:  filepath.Join(dir, "source.db"),
	}
	seed(t, files.main, "main")
	seed(t, files.replica, "replica")
	seed(t, files.source, "source")

	all := map[string]any{
		database.DialectConfigPath + ".main":    sqliteDialect(files.main),
		database.DialectConfigPath + ".replica": sqliteDialect(files.replica),
		database.DialectConfigPath + ".source":  sqliteDialect(files.source),
		database.ConnectionConfigPath + ".main": fields,
	}
	for path, value := range values {
		all[path] = value
	}
	return fullApp(t, all), files
}

// closeConnection closes the connection, as its factory does.
func closeConnection(t *testing.T, conn flam.IDatabaseConnection) {
	t.Helper()
	testutil.NoError(t, conn.(io.Closer).Close())
}

func Test_ConnectionCreator_Replicas(t *testing.T) {
	t.Run("should read from the replica and write to the dialect", func(t *testing.T) {
		app, files := resolverApp(t, container.Bag{
			database.ConfigFieldDialect:  "main",
			database.ConfigFieldReplicas: []any{"replica"},
		}, nil)
		conn, err := connections(t, app).Get("main")
		testutil.NoError(t, err)

		var read []item
		testutil.NoError(t, conn.Order("id").Find(&read).Error)
		testutil.Equal(t, "read", len(read), 1)
		testutil.Equal(t, "read name", read[0].Name, "replica")

		testutil.NoError(t, conn.Create(&item{Name: "written"}).Error)
		var written []item
		testutil.NoError(t, conn.Clauses(dbresolver.Write).Order("id").Find(&written).Error)
		testutil.Equal(t, "read your writes", len(written), 2)
		testutil.Equal(t, "read your writes names", []string{written[0].Name, written[1].Name}, []string{"main", "written"})

		var tx []item
		testutil.NoError(t, conn.Transaction(func(tx2 *gorm.DB) error { return tx2.Order("id").Find(&tx).Error }))
		testutil.Equal(t, "transaction read", len(tx), 2)

		closeConnection(t, conn)
		testutil.Equal(t, "main", names(t, files.main), []string{"main", "written"})
		testutil.Equal(t, "replica", names(t, files.replica), []string{"replica"})
		if err := conn.Find(&read).Error; err == nil || err.Error() != "sql: database is closed" {
			t.Fatalf("Find() after Close = %v, want the replica closed", err)
		}
	})

	t.Run("should write to the dialect and the sources in turn", func(t *testing.T) {
		app, files := resolverApp(t, container.Bag{
			database.ConfigFieldDialect: "main",
			database.ConfigFieldSources: []string{"source"},
			database.ConfigFieldPolicy:  database.PolicyRoundRobin,
		}, nil)
		conn, err := connections(t, app).Get("main")
		testutil.NoError(t, err)

		testutil.NoError(t, conn.Create(&item{Name: "first"}).Error)
		testutil.NoError(t, conn.Create(&item{Name: "second"}).Error)
		var read []item
		testutil.NoError(t, conn.Order("id").Find(&read).Error)
		testutil.Equal(t, "read", len(read), 2)

		closeConnection(t, conn)
		testutil.Equal(t, "main", names(t, files.main), []string{"main", "second"})
		testutil.Equal(t, "source", names(t, files.source), []string{"source", "first"})
	})

	t.Run("should use the pool of the dialect among the sources with prepared statements", func(t *testing.T) {
		app, files := resolverApp(t, container.Bag{
			database.ConfigFieldDialect:  "main",
			database.ConfigFieldConfig:   "prepared",
			database.ConfigFieldReplicas: []any{"replica"},
			database.ConfigFieldSources:  []any{"source"},
		}, map[string]any{
			database.ConfigConfigPath + ".prepared": container.Bag{
				factory.ConfigFieldDriver:       standard.Driver,
				standard.ConfigFieldPrepareStmt: true,
			},
			database.DefaultPolicyConfigPath: database.PolicyRoundRobin,
		})
		conn, err := connections(t, app).Get("main")
		testutil.NoError(t, err)

		for _, name := range []string{"first", "second"} {
			testutil.NoError(t, conn.Create(&item{Name: name}).Error)
		}
		var read []item
		testutil.NoError(t, conn.Order("id").Find(&read).Error)
		testutil.Equal(t, "read", len(read), 1)

		closeConnection(t, conn)
		testutil.Equal(t, "main", names(t, files.main), []string{"main", "second"})
		testutil.Equal(t, "source", names(t, files.source), []string{"source", "first"})
	})

	t.Run("should fail when a replica cannot be opened", func(t *testing.T) {
		app, _ := resolverApp(t, container.Bag{
			database.ConfigFieldDialect:  "main",
			database.ConfigFieldReplicas: []any{"bad"},
		}, map[string]any{
			database.DialectConfigPath + ".bad": sqliteDialect(filepath.Join(t.TempDir(), "missing", "bad.db")),
		})
		_, err := connections(t, app).Get("main")
		err = unwrapResource(err)
		if !errors.Is(err, database.ErrResolver) {
			t.Fatalf("Get() = %v, want ErrResolver", err)
		}
		testutil.Context(t, asError(t, err), map[string]any{database.ErrorFieldID: "main"})
	})

	t.Run("should fail with an unknown replica or source", func(t *testing.T) {
		for _, field := range []string{database.ConfigFieldReplicas, database.ConfigFieldSources} {
			app, _ := resolverApp(t, container.Bag{
				database.ConfigFieldDialect: "main",
				field:                       []any{"replica", "nope"},
			}, nil)
			err := unwrapResource(func() error { _, err := connections(t, app).Get("main"); return err }())
			if !errors.Is(err, database.ErrDialect) || !errors.Is(err, factory.ErrUnknownID) {
				t.Fatalf("%s: Get() = %v, want ErrDialect and factory.ErrUnknownID", field, err)
			}
			testutil.Context(t, asError(t, err), map[string]any{database.ErrorFieldID: "main", database.ErrorFieldDialect: "nope"})
		}
	})
}

func Test_ConnectionCreator_InvalidFields(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		value any
	}{
		{"replicas not a list", database.ConfigFieldReplicas, "replica"},
		{"replicas of an empty id", database.ConfigFieldReplicas, []any{"replica", ""}},
		{"replicas of a number", database.ConfigFieldReplicas, []any{1}},
		{"sources not a list", database.ConfigFieldSources, 3},
		{"sources of an empty id", database.ConfigFieldSources, []string{""}},
		{"unknown policy", database.ConfigFieldPolicy, "nope"},
		{"policy not a string", database.ConfigFieldPolicy, 1},
		{"negative retries", database.ConfigFieldConnectRetries, -1},
		{"fractional retries", database.ConfigFieldConnectRetries, 1.5},
		{"retries not a number", database.ConfigFieldConnectRetries, "1"},
		{"too many retries", database.ConfigFieldConnectRetries, uint64(1) << 40},
		{"negative delay", database.ConfigFieldConnectRetryDelay, "-1s"},
		{"delay not a duration", database.ConfigFieldConnectRetryDelay, 5},
	} {
		t.Run("should fail with "+tc.name, func(t *testing.T) {
			app, _ := resolverApp(t, container.Bag{database.ConfigFieldDialect: "main", tc.field: tc.value}, nil)
			_, err := connections(t, app).Get("main")
			testutil.Error(t, unwrapResource(err), database.ErrInvalidField,
				"unable to create database connection: invalid database connection field", map[string]any{
					database.ErrorFieldID: "main", database.ErrorFieldField: tc.field,
				})
		})
	}

	t.Run("should accept the whole numbers of any kind", func(t *testing.T) {
		for _, retries := range []any{0, uint8(0), 0.0} {
			app, _ := resolverApp(t, container.Bag{
				database.ConfigFieldDialect:        "main",
				database.ConfigFieldConnectRetries: retries,
			}, nil)
			conn, err := connections(t, app).Get("main")
			testutil.NoError(t, err)
			closeConnection(t, conn)
		}
	})

	t.Run("should fail to retry without clock", func(t *testing.T) {
		app, _ := resolverApp(t, container.Bag{database.ConfigFieldDialect: "main"}, map[string]any{
			database.DefaultConnectRetriesConfigPath: 1,
		})
		_, err := connections(t, app).Get("main")
		testutil.Error(t, unwrapResource(err), database.ErrNilClock,
			"unable to create database connection: nil clock", map[string]any{database.ErrorFieldID: "main"})
	})
}

// waitTimer waits for the clock to hold a timer, as the creator sleeping.
func waitTimer(t *testing.T, clock *testutil.Clock) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for clock.Timers() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("no timer started")
		}
		time.Sleep(time.Millisecond)
	}
}

// retryApp returns an app with every database bundle, the clock, and the
// connection "main" over the dialect "late", whose file is in a directory
// that does not exist yet, which it returns.
func retryApp(t *testing.T, clock *testutil.Clock, fields container.Bag, values map[string]any) (*testutil.App, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "late")
	fields[database.ConfigFieldDialect] = "late"
	all := map[string]any{
		database.DialectConfigPath + ".late":    sqliteDialect(filepath.Join(dir, "app.db")),
		database.ConnectionConfigPath + ".main": fields,
	}
	for path, value := range values {
		all[path] = value
	}
	app := fullApp(t, all)
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	return app, dir
}

func Test_ConnectionCreator_Retries(t *testing.T) {
	type result struct {
		conn flam.IDatabaseConnection
		err  error
	}
	get := func(f flam.IDatabaseConnectionFactory) <-chan result {
		done := make(chan result, 1)
		go func() {
			conn, err := f.Get("main")
			done <- result{conn, err}
		}()
		return done
	}

	t.Run("should retry the opening after a doubling delay", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		app, dir := retryApp(t, clock, container.Bag{
			database.ConfigFieldConnectRetries:    3,
			database.ConfigFieldConnectRetryDelay: "1s",
		}, nil)
		done := get(connections(t, app))

		waitTimer(t, clock)
		clock.Advance(time.Second)
		waitTimer(t, clock)
		clock.Advance(2*time.Second - time.Nanosecond)
		testutil.Equal(t, "timers before the doubled delay", clock.Timers(), 1)
		testutil.NoError(t, os.MkdirAll(dir, 0o750))
		clock.Advance(time.Nanosecond)

		res := <-done
		testutil.NoError(t, res.err)
		testutil.Equal(t, "Id()", res.conn.Id(), "main")
		closeConnection(t, res.conn)
	})

	t.Run("should fail once the retries are exhausted, waiting at most the limit", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		app, _ := retryApp(t, clock, container.Bag{}, map[string]any{
			database.DefaultConnectRetriesConfigPath:    2,
			database.DefaultConnectRetryDelayConfigPath: "20s",
		})
		done := get(connections(t, app))

		waitTimer(t, clock)
		clock.Advance(20 * time.Second)
		waitTimer(t, clock)
		clock.Advance(database.ConnectRetryMaxDelay)

		res := <-done
		err := unwrapResource(res.err)
		if !errors.Is(err, database.ErrOpen) {
			t.Fatalf("Get() = %v, want ErrOpen", err)
		}
		testutil.Context(t, asError(t, err), map[string]any{
			database.ErrorFieldID: "main", database.ErrorFieldDialect: "late", database.ErrorFieldConfig: standard.Id,
		})
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should not retry by default", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		app, _ := retryApp(t, clock, container.Bag{}, nil)
		_, err := connections(t, app).Get("main")
		if !errors.Is(err, database.ErrOpen) {
			t.Fatalf("Get() = %v, want ErrOpen", err)
		}
	})
}

func Test_ResolverDeclarations(t *testing.T) {
	for got, want := range map[string]string{
		database.ConfigFieldReplicas:                "replicas",
		database.ConfigFieldSources:                 "sources",
		database.ConfigFieldPolicy:                  "policy",
		database.ConfigFieldConnectRetries:          "connect_retries",
		database.ConfigFieldConnectRetryDelay:       "connect_retry_delay",
		database.PolicyRandom:                       "random",
		database.PolicyRoundRobin:                   "round_robin",
		database.DefaultPolicyConfigPath:            "flam.database.defaults.connections.policy",
		database.DefaultConnectRetriesConfigPath:    "flam.database.defaults.connections.connect_retries",
		database.DefaultConnectRetryDelayConfigPath: "flam.database.defaults.connections.connect_retry_delay",
		database.ErrorFieldAttempts:                 "attempts",
	} {
		testutil.Equal(t, "declaration", got, want)
	}
	testutil.Equal(t, "ConnectRetryMaxDelay", database.ConnectRetryMaxDelay, 30*time.Second)
	testutil.Equal(t, "DefaultRetryAttempts", database.DefaultRetryAttempts, 3)
	testutil.Equal(t, "DefaultRetryDelay", database.DefaultRetryDelay, 50*time.Millisecond)
	testutil.Equal(t, "DefaultRetryMaxDelay", database.DefaultRetryMaxDelay, time.Second)
}
