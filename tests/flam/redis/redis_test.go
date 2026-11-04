package redis

import (
	"context"
	"errors"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
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
	a := testutil.NewApp()
	for _, b := range all {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// connectionFactory returns the connection factory of the app.
func connectionFactory(t *testing.T, a flam.IApp) flam.IRedisConnectionFactory {
	t.Helper()
	var f flam.IRedisConnectionFactory
	testutil.NoError(t, a.Invoke(func(provided flam.IRedisConnectionFactory) { f = provided }))
	return f
}

// fakeCreator is a flam.IRedisConnectionCreator of the "fake" driver,
// creating clients of an unreachable server.
type fakeCreator struct{}

func (fakeCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (fakeCreator) Create(bag flam.IBag) (flam.IRedisConnection, error) {
	return redis.NewConnection(bag.String(factory.ConfigFieldId), goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}))
}

// fakeBundle provides the fakeCreator.
type fakeBundle struct{}

func (fakeBundle) Id() string { return "fake" }

func (fakeBundle) Register(a flam.IApp) error {
	return a.Provide(func() flam.IRedisConnectionCreator { return fakeCreator{} }, dig.Group(redis.ConnectionCreatorGroup))
}

// failingConnection is a flam.IRedisConnection failing to close.
type failingConnection struct {
	flam.IRedisConnection
}

var errBoom = errors.New("boom")

func (failingConnection) Id() string   { return "bad" }
func (failingConnection) Close() error { return errBoom }

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ConnectionConfigPath", redis.ConnectionConfigPath, "flam.redis.connections")
	testutil.Equal(t, "ConnectionCreatorGroup", redis.ConnectionCreatorGroup, "flam.redis.connections.creator")
	testutil.Equal(t, "ErrorFieldID", redis.ErrorFieldID, "id")
	for got, want := range map[string]string{
		redis.ConfigFieldUsername:              "username",
		redis.ConfigFieldPassword:              "password",
		redis.ConfigFieldDatabase:              "database",
		redis.ConfigFieldProtocol:              "protocol",
		redis.ConfigFieldClientName:            "client_name",
		redis.ConfigFieldMaxRetries:            "max_retries",
		redis.ConfigFieldMinRetryBackoff:       "min_retry_backoff",
		redis.ConfigFieldMaxRetryBackoff:       "max_retry_backoff",
		redis.ConfigFieldDialTimeout:           "dial_timeout",
		redis.ConfigFieldReadTimeout:           "read_timeout",
		redis.ConfigFieldWriteTimeout:          "write_timeout",
		redis.ConfigFieldContextTimeoutEnabled: "context_timeout_enabled",
		redis.ConfigFieldReadBufferSize:        "read_buffer_size",
		redis.ConfigFieldWriteBufferSize:       "write_buffer_size",
		redis.ConfigFieldPoolFIFO:              "pool_fifo",
		redis.ConfigFieldPoolSize:              "pool_size",
		redis.ConfigFieldPoolTimeout:           "pool_timeout",
		redis.ConfigFieldMinIdleConns:          "min_idle_conns",
		redis.ConfigFieldMaxIdleConns:          "max_idle_conns",
		redis.ConfigFieldMaxActiveConns:        "max_active_conns",
		redis.ConfigFieldConnMaxIdleTime:       "conn_max_idle_time",
		redis.ConfigFieldConnMaxLifetime:       "conn_max_lifetime",
		redis.ConfigFieldDisableIdentity:       "disable_identity",
		redis.ConfigFieldIdentitySuffix:        "identity_suffix",
	} {
		testutil.Equal(t, "ConfigField", got, want)
	}
}

func Test_NewConnection(t *testing.T) {
	t.Run("should fail with a nil client", func(t *testing.T) {
		conn, err := redis.NewConnection("main", nil)
		testutil.Error(t, err, redis.ErrNilClient, "unable to create redis connection: nil redis client",
			map[string]any{redis.ErrorFieldID: "main"})
		testutil.Equal(t, "conn", conn, nil)
	})

	t.Run("should run the commands of the client, and close it", func(t *testing.T) {
		client := goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"})
		conn, err := redis.NewConnection("main", client)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.Equal(t, "Options()", conn.Options(), client.Options())

		testutil.NoError(t, conn.Close())
		if err := client.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
			t.Fatalf("Ping() after Close = %v, want redis.ErrClosed", err)
		}
	})
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factory", func(t *testing.T) {
		b := redis.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.redis")

		a := newApp(t, nil, b)
		testutil.Equal(t, "calls", a.Calls()[len(a.Calls())-3:], []string{
			"bundle:flam.redis", "provider/0", "invoker/0",
		})
		testutil.Equal(t, "Available()", connectionFactory(t, a).Available(), []string{})
	})

	t.Run("should create the configured connections with the creators of the group", func(t *testing.T) {
		a := newApp(t, map[string]any{
			redis.ConnectionConfigPath + ".main":  container.Bag{factory.ConfigFieldDriver: "fake"},
			redis.ConnectionConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
		}, redis.NewBundle(), fakeBundle{})
		f := connectionFactory(t, a)
		testutil.Equal(t, "Available()", f.Available(), []string{"main", "other"})

		conn, err := f.Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		if _, err := f.Get("other"); !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})

	t.Run("should close the factory, and so its connections", func(t *testing.T) {
		b := redis.NewBundle()
		a := newApp(t, map[string]any{
			redis.ConnectionConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: "fake"},
		}, b, fakeBundle{})
		f := connectionFactory(t, a)
		conn, err := f.Get("main")
		testutil.NoError(t, err)

		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
		if err := conn.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
			t.Fatalf("Ping() after Close = %v, want redis.ErrClosed", err)
		}
		if _, err := f.Get("main"); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Get() after Close = %v, want factory.ErrClosed", err)
		}
		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
	})

	t.Run("should return the errors of closing", func(t *testing.T) {
		b := redis.NewBundle()
		a := newApp(t, nil, b)
		testutil.NoError(t, connectionFactory(t, a).Store("bad", failingConnection{}))

		if err := b.(flam.IClosableBundle).Close(a); !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		b := redis.NewBundle()
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
		testutil.Equal(t, "Id()", b.Id(), "flam.redis.connections")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{
			"bundle:flam.redis.connections",
			"bundle:flam.redis.connections.standard", "provider/1",
			"bundle:flam.redis.connections.mini", "provider/1",
		})
	})

	t.Run("should run commands in an application", func(t *testing.T) {
		values := container.Bag{}
		testutil.NoError(t, values.Set(redis.ConnectionConfigPath+".main", container.Bag{
			factory.ConfigFieldDriver: mini.Driver,
			redis.ConfigFieldDatabase: 2,
		}))
		a := app.New(values)
		for _, b := range []flam.IBundle{redis.NewBundle(), connections.NewBundle()} {
			testutil.NoError(t, a.RegisterBundle(b))
		}
		testutil.NoError(t, a.Boot())

		var conn flam.IRedisConnection
		testutil.NoError(t, a.Invoke(func(f flam.IRedisConnectionFactory) error {
			var err error
			if conn, err = f.Get("main"); err != nil {
				return err
			}
			ctx := context.Background()
			if err := conn.Set(ctx, "key", "value", 0).Err(); err != nil {
				return err
			}
			got, err := conn.Get(ctx, "key").Result()
			testutil.Equal(t, "Get()", got, "value")
			return err
		}))
		server, ok := mini.Server(conn)
		testutil.Equal(t, "Server() ok", ok, true)
		server.Select(2)
		testutil.Equal(t, "Keys() of the database", server.Keys(), []string{"key"})

		testutil.NoError(t, a.Close())
		if err := conn.Ping(context.Background()).Err(); !errors.Is(err, goredis.ErrClosed) {
			t.Fatalf("Ping() after Close = %v, want redis.ErrClosed", err)
		}
	})
}
