package mongo

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	gomongo "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo/connections"
	"github.com/cjdias/flam-in-go/pkg/flam/mongo/connections/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// unreachable is the URI of a deployment that cannot be reached, its
// servers being selected for a short time only.
const unreachable = "mongodb://127.0.0.1:1/?directConnection=true&serverSelectionTimeoutMS=100&connectTimeoutMS=100"

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
	a := testutil.NewApp()
	for _, b := range all {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// connectionFactory returns the connection factory of the app.
func connectionFactory(t *testing.T, a flam.IApp) flam.IMongoConnectionFactory {
	t.Helper()
	var f flam.IMongoConnectionFactory
	testutil.NoError(t, a.Invoke(func(provided flam.IMongoConnectionFactory) { f = provided }))
	return f
}

// newClient returns a client of the unreachable deployment, disconnected
// at the end of the test.
func newClient(t *testing.T) *gomongo.Client {
	t.Helper()
	client, err := gomongo.Connect(options.Client().ApplyURI(unreachable))
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	return client
}

// fakeCreator is a flam.IMongoConnectionCreator of the "fake" driver,
// creating connections of the unreachable deployment.
type fakeCreator struct{}

func (fakeCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (fakeCreator) Create(bag flam.IBag) (flam.IMongoConnection, error) {
	client, err := gomongo.Connect(options.Client().ApplyURI(unreachable))
	if err != nil {
		return nil, err
	}
	return mongo.NewConnection(bag.String(factory.ConfigFieldId), client, "app")
}

// fakeBundle provides the fakeCreator.
type fakeBundle struct{}

func (fakeBundle) Id() string { return "fake" }

func (fakeBundle) Register(a flam.IApp) error {
	return a.Provide(func() flam.IMongoConnectionCreator { return fakeCreator{} }, dig.Group(mongo.ConnectionCreatorGroup))
}

// failingConnection is a flam.IMongoConnection failing to close.
type failingConnection struct {
	flam.IMongoConnection
}

var errBoom = errors.New("boom")

func (failingConnection) Id() string   { return "bad" }
func (failingConnection) Close() error { return errBoom }

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ConnectionConfigPath", mongo.ConnectionConfigPath, "flam.mongo.connections")
	testutil.Equal(t, "ConnectionCreatorGroup", mongo.ConnectionCreatorGroup, "flam.mongo.connections.creator")
	testutil.Equal(t, "ErrorFieldID", mongo.ErrorFieldID, "id")
}

func Test_NewConnection(t *testing.T) {
	t.Run("should fail with a nil client", func(t *testing.T) {
		conn, err := mongo.NewConnection("main", nil, "app")
		testutil.Error(t, err, mongo.ErrNilClient, "unable to create mongo connection: nil mongo client",
			map[string]any{mongo.ErrorFieldID: "main"})
		testutil.Equal(t, "conn", conn, nil)
	})

	t.Run("should fail with an empty database name", func(t *testing.T) {
		conn, err := mongo.NewConnection("main", newClient(t), "")
		testutil.Error(t, err, mongo.ErrEmptyDatabase, "unable to create mongo connection: empty mongo database name",
			map[string]any{mongo.ErrorFieldID: "main"})
		testutil.Equal(t, "conn", conn, nil)
	})

	t.Run("should hold the client and its database", func(t *testing.T) {
		client := newClient(t)
		conn, err := mongo.NewConnection("main", client, "app")
		testutil.NoError(t, err)

		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.Equal(t, "Client()", conn.Client() == client, true)
		testutil.Equal(t, "Database().Name()", conn.Database().Name(), "app")
		testutil.Equal(t, "Database().Client()", conn.Database().Client() == client, true)
	})

	t.Run("should fail to ping an unreachable deployment", func(t *testing.T) {
		conn, err := mongo.NewConnection("main", newClient(t), "app")
		testutil.NoError(t, err)

		start := time.Now()
		err = conn.Ping(context.Background())
		if !gomongo.IsTimeout(err) || !strings.Contains(err.Error(), "server selection error") {
			t.Fatalf("Ping() = %v, want a server selection timeout", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Ping() = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{mongo.ErrorFieldID: "main"})
		testutil.Equal(t, "message prefix", err.Error()[:len("unable to ping mongo connection: ")], "unable to ping mongo connection: ")
		testutil.Equal(t, "within the server selection timeout", time.Since(start) < 10*time.Second, true)
	})

	t.Run("should stop pinging when the context ends", func(t *testing.T) {
		conn, err := mongo.NewConnection("main", newClient(t), "app")
		testutil.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := conn.Ping(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("Ping() = %v, want context.Canceled", err)
		}
	})

	t.Run("should disconnect the client when closed, once", func(t *testing.T) {
		conn, err := mongo.NewConnection("main", newClient(t), "app")
		testutil.NoError(t, err)

		testutil.NoError(t, conn.Close())
		err = conn.Close()
		testutil.Error(t, err, gomongo.ErrClientDisconnected,
			"unable to close mongo connection: client is disconnected", map[string]any{mongo.ErrorFieldID: "main"})
		if err := conn.Ping(context.Background()); !errors.Is(err, gomongo.ErrClientDisconnected) {
			t.Fatalf("Ping() after Close = %v, want ErrClientDisconnected", err)
		}
	})
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factory", func(t *testing.T) {
		b := mongo.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.mongo")

		a := newApp(t, nil, b)
		testutil.Equal(t, "calls", a.Calls()[len(a.Calls())-3:], []string{
			"bundle:flam.mongo", "provider/0", "invoker/0",
		})
		testutil.Equal(t, "Available()", connectionFactory(t, a).Available(), []string{})
	})

	t.Run("should create the configured connections with the creators of the group", func(t *testing.T) {
		a := newApp(t, map[string]any{
			mongo.ConnectionConfigPath + ".main":  container.Bag{factory.ConfigFieldDriver: "fake"},
			mongo.ConnectionConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
		}, mongo.NewBundle(), fakeBundle{})
		f := connectionFactory(t, a)
		t.Cleanup(func() { _ = f.Close() })
		testutil.Equal(t, "Available()", f.Available(), []string{"main", "other"})

		conn, err := f.Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		if _, err := f.Get("other"); !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})

	t.Run("should close the factory, and so disconnect its connections", func(t *testing.T) {
		b := mongo.NewBundle()
		a := newApp(t, map[string]any{
			mongo.ConnectionConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: "fake"},
		}, b, fakeBundle{})
		f := connectionFactory(t, a)
		conn, err := f.Get("main")
		testutil.NoError(t, err)

		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
		if err := conn.Ping(context.Background()); !errors.Is(err, gomongo.ErrClientDisconnected) {
			t.Fatalf("Ping() after Close = %v, want ErrClientDisconnected", err)
		}
		if _, err := f.Get("main"); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Get() after Close = %v, want factory.ErrClosed", err)
		}
		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
	})

	t.Run("should return the errors of closing", func(t *testing.T) {
		b := mongo.NewBundle()
		a := newApp(t, nil, b)
		testutil.NoError(t, connectionFactory(t, a).Store("bad", failingConnection{}))

		if err := b.(flam.IClosableBundle).Close(a); !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		b := mongo.NewBundle()
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		if err := b.(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

func Test_AggregateBundle(t *testing.T) {
	t.Run("should register every connection driver", func(t *testing.T) {
		b := connections.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.mongo.connections")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.mongo.connections",
			"bundle:flam.mongo.connections.standard", "provider/1",
		})
	})

	t.Run("should create a connection in an application, disconnected on close", func(t *testing.T) {
		values := container.Bag{}
		testutil.NoError(t, values.Set(mongo.ConnectionConfigPath+".main", container.Bag{
			factory.ConfigFieldDriver:    standard.Driver,
			standard.ConfigFieldURI:      unreachable,
			standard.ConfigFieldDatabase: "app",
		}))
		a := app.New(values)
		for _, b := range []flam.IBundle{mongo.NewBundle(), connections.NewBundle()} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		testutil.NoError(t, a.Boot())

		var conn flam.IMongoConnection
		testutil.NoError(t, a.Invoke(func(f flam.IMongoConnectionFactory) error {
			var err error
			conn, err = f.Get("main")
			return err
		}))
		testutil.Equal(t, "Database().Name()", conn.Database().Name(), "app")

		testutil.NoError(t, a.Close())
		if err := conn.Ping(context.Background()); !errors.Is(err, gomongo.ErrClientDisconnected) {
			t.Fatalf("Ping() after Close = %v, want ErrClientDisconnected", err)
		}
	})
}
