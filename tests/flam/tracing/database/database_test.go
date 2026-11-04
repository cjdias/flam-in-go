package database

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamdatabase "github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/database"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// item is the model of the tests.
type item struct {
	Id   uint `gorm:"primaryKey"`
	Name string
}

// recorded returns a tracer provider recording its spans in the recorder
// returned.
func recorded() (trace.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	return sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)), recorder
}

// openDB returns a gorm database over a new in-memory SQLite database,
// holding the items table, with the plugin of the provider added.
func openDB(t *testing.T, provider trace.TracerProvider) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	sqlDB, err := db.DB()
	testutil.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // a single connection, as each has its own in-memory database
	t.Cleanup(func() { _ = sqlDB.Close() })
	testutil.NoError(t, db.AutoMigrate(&item{}))
	testutil.NoError(t, db.Use(database.NewPlugin(provider)))
	return db
}

// attributes returns the attributes of the span, by key.
func attributes(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

// names returns the names of the spans ended.
func names(recorder *tracetest.SpanRecorder) []string {
	var all []string
	for _, span := range recorder.Ended() {
		all = append(all, span.Name())
	}
	return all
}

// last returns the last span ended.
func last(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("no span ended")
	}
	return spans[len(spans)-1]
}

func Test_Plugin(t *testing.T) {
	t.Run("should be named", func(t *testing.T) {
		testutil.Equal(t, "Name()", database.NewPlugin(nil).Name(), "flam.tracing")
		testutil.Equal(t, "PluginName", database.PluginName, "flam.tracing")
	})

	t.Run("should trace the statements of each operation", func(t *testing.T) {
		provider, recorder := recorded()
		db := openDB(t, provider)

		testutil.NoError(t, db.Create(&item{Name: "a"}).Error)
		var got item
		testutil.NoError(t, db.First(&got).Error)
		testutil.NoError(t, db.Model(&got).Update("name", "b").Error)
		var count int
		testutil.NoError(t, db.Table("items").Select("count(*)").Row().Scan(&count))
		testutil.NoError(t, db.Exec("UPDATE items SET name = ?", "c").Error)
		testutil.NoError(t, db.Delete(&got).Error)

		testutil.Equal(t, "spans", names(recorder), []string{
			"gorm.create items", "gorm.query items", "gorm.update items", "gorm.row items", "gorm.raw", "gorm.delete items",
		})
		spans := recorder.Ended()
		create := spans[0]
		testutil.Equal(t, "kind", create.SpanKind(), trace.SpanKindClient)
		testutil.Equal(t, "scope", create.InstrumentationScope().Name, database.ScopeName)
		testutil.Equal(t, "status", create.Status().Code, codes.Unset)
		testutil.Equal(t, "attributes", attributes(create), map[attribute.Key]attribute.Value{
			"db.system.name":               attribute.StringValue("sqlite"),
			"db.operation.name":            attribute.StringValue(database.OperationCreate),
			"db.collection.name":           attribute.StringValue("items"),
			"db.query.text":                attribute.StringValue("INSERT INTO `items` (`name`) VALUES (?) RETURNING `id`"),
			database.AttributeRowsAffected: attribute.Int64Value(1),
		})
		for i, operation := range []string{
			database.OperationCreate, database.OperationQuery, database.OperationUpdate,
			database.OperationRow, database.OperationRaw, database.OperationDelete,
		} {
			testutil.Equal(t, operation, attributes(spans[i])["db.operation.name"], attribute.StringValue(operation))
		}
		raw := attributes(spans[4])
		testutil.Equal(t, "raw query", raw["db.query.text"], attribute.StringValue("UPDATE items SET name = ?"))
		_, ok := raw["db.collection.name"]
		testutil.Equal(t, "raw has collection", ok, false)
	})

	t.Run("should be the child of the span of the statement context, given back", func(t *testing.T) {
		provider, recorder := recorded()
		db := openDB(t, provider)
		ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
		res := db.WithContext(ctx).Exec("DELETE FROM items")
		testutil.NoError(t, res.Error)
		parent.End()

		testutil.Equal(t, "context given back", res.Statement.Context, ctx)
		span := recorder.Ended()[0]
		testutil.Equal(t, "parent", span.Parent().SpanID(), parent.SpanContext().SpanID())
	})

	t.Run("should record the errors", func(t *testing.T) {
		provider, recorder := recorded()
		db := openDB(t, provider)
		err := db.Exec("SELECT * FROM missing").Error
		if err == nil {
			t.Fatalf("Exec() = nil, want an error")
		}
		span := last(t, recorder)
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: err.Error()})
		testutil.Equal(t, "events", len(span.Events()), 1)
	})

	t.Run("should not record a record not found", func(t *testing.T) {
		provider, recorder := recorded()
		db := openDB(t, provider)
		var got item
		if err := db.First(&got).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("First() = %v, want ErrRecordNotFound", err)
		}
		span := last(t, recorder)
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
		testutil.Equal(t, "rows", attributes(span)[database.AttributeRowsAffected], attribute.Int64Value(0))
	})

	t.Run("should skip the statements it started no span for", func(t *testing.T) {
		provider, recorder := recorded()
		db := openDB(t, provider)
		testutil.NoError(t, db.Callback().Raw().Remove("flam.tracing:before_raw"))
		testutil.NoError(t, db.Exec("DELETE FROM items").Error)
		testutil.Equal(t, "spans", len(recorder.Ended()), 0)
	})

	t.Run("should use the global provider when nil", func(t *testing.T) {
		provider, recorder := recorded()
		global := otel.GetTracerProvider()
		otel.SetTracerProvider(provider)
		defer otel.SetTracerProvider(global)
		db := openDB(t, nil)
		testutil.NoError(t, db.Exec("DELETE FROM items").Error)
		testutil.Equal(t, "spans", names(recorder), []string{"gorm.raw"})
	})

	t.Run("should fail when its callbacks conflict", func(t *testing.T) {
		db, err := gorm.Open(gormsqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
		testutil.NoError(t, err)
		testutil.NoError(t, conflicting(db))
		if err := db.Use(database.NewPlugin(nil)); err == nil {
			t.Errorf("Use() = nil, want an error")
		}
	})
}

// conflicting registers on the database a callback which the callbacks of
// the plugin conflict with.
func conflicting(db *gorm.DB) error {
	return db.Callback().Create().Before("flam.tracing:before_create").After("gorm:create").Register("conflict", func(*gorm.DB) {})
}

// newApp returns a testutil.App with the config, database, standard
// configuration, SQLite dialect and tracing database bundles, the tracer
// provider, and the SQLite connection "main".
func newApp(t *testing.T, provider trace.TracerProvider) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), flamdatabase.NewBundle(), standard.NewBundle(), sqlite.NewBundle(), database.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamdatabase.DialectConfigPath+".main", container.Bag{
		factory.ConfigFieldDriver: sqlite.Driver,
		sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
	}))
	testutil.NoError(t, bag.Set(flamdatabase.ConnectionConfigPath+".main", container.Bag{flamdatabase.ConfigFieldDialect: "main"}))
	app := testutil.NewApp()
	if provider != nil {
		testutil.NoError(t, app.Provide(func() trace.TracerProvider { return provider }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() { _ = flamdatabase.NewBundle().(flam.IClosableBundle).Close(app) })
	return app
}

// connection returns the connection of the id of the connection factory
// of the app, or the error of its creation.
func connection(app *testutil.App, id string) (flam.IDatabaseConnection, error) {
	var conn flam.IDatabaseConnection
	var err error
	if invokeErr := app.Invoke(func(f flam.IDatabaseConnectionFactory) { conn, err = f.Get(id) }); invokeErr != nil {
		return nil, invokeErr
	}
	return conn, err
}

// stub is a connection creator of the tests, returning its connection and
// error.
type stub struct {
	conn flam.IDatabaseConnection
	err  error
}

func (s stub) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "stub"
}

func (s stub) Create(flam.IBag) (flam.IDatabaseConnection, error) {
	return s.conn, s.err
}

// refusing is a connection refusing the plugins with its error, closing
// once when closer.
type refusing struct {
	flam.IDatabaseConnection

	err    error
	closed int
}

func (r *refusing) Use(gorm.Plugin) error {
	return r.err
}

// closingRefusing is a refusing connection implementing io.Closer.
type closingRefusing struct {
	*refusing
}

func (c closingRefusing) Close() error {
	c.closed++
	return nil
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the decorator", func(t *testing.T) {
		b := database.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing.database")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.tracing.database", "decorator/0"})
	})

	t.Run("should trace the statements of the connections", func(t *testing.T) {
		provider, recorder := recorded()
		app := newApp(t, provider)
		conn, err := connection(app, "main")
		testutil.NoError(t, err)
		testutil.NoError(t, conn.AutoMigrate(&item{}))
		testutil.NoError(t, conn.Create(&item{Name: "a"}).Error)
		testutil.Equal(t, "last span", last(t, recorder).Name(), "gorm.create items")
	})

	t.Run("should trace with the provider of the tracing bundle", func(t *testing.T) {
		app := newApp(t, nil)
		testutil.NoError(t, app.RegisterBundle(tracing.NewBundle()))
		conn, err := connection(app, "main")
		testutil.NoError(t, err)
		testutil.NoError(t, conn.Exec("SELECT 1").Error)
	})

	t.Run("should fail without tracer provider", func(t *testing.T) {
		app := newApp(t, nil)
		if _, err := connection(app, "main"); err == nil {
			t.Errorf("connection() = nil error, want one")
		}
	})

	t.Run("should return the creation failures as they are", func(t *testing.T) {
		provider, _ := recorded()
		for name, s := range map[string]stub{
			"error": {err: errBoom},
			"nil":   {},
		} {
			app := testutil.NewApp()
			testutil.NoError(t, app.Provide(func() trace.TracerProvider { return provider }))
			testutil.NoError(t, app.Provide(func() flam.IDatabaseConnectionCreator { return s },
				dig.Group(flamdatabase.ConnectionCreatorGroup)))
			testutil.NoError(t, app.RegisterBundle(database.NewBundle()))
			testutil.NoError(t, app.Invoke(func(args struct {
				dig.In
				Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
			}) {
				conn, err := args.Creators[0].Create(container.Bag{})
				testutil.Equal(t, name+" conn", conn, s.conn)
				testutil.Equal(t, name+" err", err, s.err)
			}))
		}
	})

	t.Run("should keep a connection having the plugin already", func(t *testing.T) {
		provider, _ := recorded()
		conn := &refusing{err: gorm.ErrRegistered}
		created := decorated(t, provider, conn)
		got, err := created(container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "conn", got, flam.IDatabaseConnection(conn))
	})

	t.Run("should fail, closing the connection, when the plugin cannot be added", func(t *testing.T) {
		provider, _ := recorded()
		conn := closingRefusing{&refusing{err: errBoom}}
		created := decorated(t, provider, conn)
		got, err := created(container.Bag{factory.ConfigFieldId: "main"})
		testutil.Equal(t, "conn", got, nil)
		testutil.Error(t, err, database.ErrPlugin,
			"unable to create traced database connection: unable to add tracing plugin to database connection: boom",
			map[string]any{database.ErrorFieldID: "main"})
		if !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want it to wrap errBoom", err)
		}
		testutil.Equal(t, "closed", conn.closed, 1)
	})

	t.Run("should fail when the plugin cannot be added to a connection not closer", func(t *testing.T) {
		provider, _ := recorded()
		created := decorated(t, provider, &refusing{err: errBoom})
		if _, err := created(container.Bag{}); !errors.Is(err, database.ErrPlugin) {
			t.Errorf("error = %v, want it to wrap ErrPlugin", err)
		}
	})

	t.Run("should fail when the callbacks of the connection conflict", func(t *testing.T) {
		db, err := gorm.Open(gormsqlite.Open(":memory:"), &gorm.Config{Logger: logger.Discard})
		testutil.NoError(t, err)
		testutil.NoError(t, conflicting(db))
		conn, err := flamdatabase.NewConnection("main", db)
		testutil.NoError(t, err)
		provider, _ := recorded()
		created := decorated(t, provider, conn)
		if _, err := created(container.Bag{factory.ConfigFieldId: "main"}); !errors.Is(err, database.ErrPlugin) {
			t.Errorf("error = %v, want it to wrap ErrPlugin", err)
		}
		if err := db.Exec("SELECT 1").Error; err == nil {
			t.Errorf("Exec() = nil, want the error of the closed connection")
		}
	})
}

// decorated returns the Create method of the stub creator of the
// connection, decorated by the bundle with the provider.
func decorated(t *testing.T, provider trace.TracerProvider, conn flam.IDatabaseConnection) func(flam.IBag) (flam.IDatabaseConnection, error) {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() trace.TracerProvider { return provider }))
	testutil.NoError(t, app.Provide(func() flam.IDatabaseConnectionCreator { return stub{conn: conn} },
		dig.Group(flamdatabase.ConnectionCreatorGroup)))
	testutil.NoError(t, app.RegisterBundle(database.NewBundle()))
	var creator flam.IDatabaseConnectionCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.IDatabaseConnectionCreator `group:"flam.database.connections.creator"`
	}) {
		creator = args.Creators[0]
	}))
	testutil.Equal(t, "Accept()", creator.Accept(container.Bag{factory.ConfigFieldDriver: "stub"}), true)
	return creator.Create
}
