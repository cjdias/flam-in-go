package connections

import (
	"context"
	"errors"
	"math"
	"net"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// connectionFactory returns the connection factory of an app with the
// redis bundles, the connection "main" of the driver and the fields, and
// the values, closed at the end of the test.
func connectionFactory(t *testing.T, driver string, fields container.Bag, values map[string]any) flam.IRedisConnectionFactory {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), redis.NewBundle(), standard.NewBundle(), mini.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	fields[factory.ConfigFieldDriver] = driver
	testutil.NoError(t, bag.Set(redis.ConnectionConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var f flam.IRedisConnectionFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IRedisConnectionFactory) { f = provided }))
	t.Cleanup(func() { _ = f.Close() })
	return f
}

// create returns the connection "main" of the driver and the fields.
func create(t *testing.T, driver string, fields container.Bag, values map[string]any) (flam.IRedisConnection, error) {
	t.Helper()
	return connectionFactory(t, driver, fields, values).Get("main")
}

// options returns the go-redis options of the standard connection of the
// fields.
func options(t *testing.T, fields container.Bag, values map[string]any) *goredis.Options {
	t.Helper()
	conn, err := create(t, standard.Driver, fields, values)
	testutil.NoError(t, err)
	testutil.Equal(t, "Id()", conn.Id(), "main")
	return conn.Options()
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
		{standard.NewBundle(), "flam.redis.connections.standard", map[string]any{
			standard.DefaultNetworkConfigPath: "tcp", standard.DefaultHostConfigPath: "127.0.0.1",
			standard.DefaultPortConfigPath: 6379, standard.DefaultUsernameConfigPath: "",
			standard.DefaultPasswordConfigPath: "", standard.DefaultDatabaseConfigPath: 0,
		}},
		{mini.NewBundle(), "flam.redis.connections.mini", map[string]any{
			mini.DefaultAddressConfigPath: "127.0.0.1:0",
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

	t.Run("should declare the drivers, fields and paths", func(t *testing.T) {
		testutil.Equal(t, "standard.Driver", standard.Driver, "flam.redis.connections.driver.standard")
		testutil.Equal(t, "standard.ConfigFieldNetwork", standard.ConfigFieldNetwork, "network")
		testutil.Equal(t, "standard.ConfigFieldHost", standard.ConfigFieldHost, "host")
		testutil.Equal(t, "standard.ConfigFieldPort", standard.ConfigFieldPort, "port")
		testutil.Equal(t, "standard.DefaultNetworkConfigPath", standard.DefaultNetworkConfigPath, "flam.redis.defaults.connections.standard.network")
		testutil.Equal(t, "standard.DefaultHostConfigPath", standard.DefaultHostConfigPath, "flam.redis.defaults.connections.standard.host")
		testutil.Equal(t, "standard.DefaultPortConfigPath", standard.DefaultPortConfigPath, "flam.redis.defaults.connections.standard.port")
		testutil.Equal(t, "standard.DefaultUsernameConfigPath", standard.DefaultUsernameConfigPath, "flam.redis.defaults.connections.standard.username")
		testutil.Equal(t, "standard.DefaultPasswordConfigPath", standard.DefaultPasswordConfigPath, "flam.redis.defaults.connections.standard.password")
		testutil.Equal(t, "standard.DefaultDatabaseConfigPath", standard.DefaultDatabaseConfigPath, "flam.redis.defaults.connections.standard.database")
		testutil.Equal(t, "standard.ErrorFieldID", standard.ErrorFieldID, "id")
		testutil.Equal(t, "standard.ErrorFieldField", standard.ErrorFieldField, "field")
		testutil.Equal(t, "mini.Driver", mini.Driver, "flam.redis.connections.driver.mini")
		testutil.Equal(t, "mini.ConfigFieldAddress", mini.ConfigFieldAddress, "address")
		testutil.Equal(t, "mini.DefaultAddressConfigPath", mini.DefaultAddressConfigPath, "flam.redis.defaults.connections.mini.address")
		testutil.Equal(t, "mini.ErrorFieldID", mini.ErrorFieldID, "id")
		testutil.Equal(t, "mini.ErrorFieldField", mini.ErrorFieldField, "field")
		testutil.Equal(t, "mini.ErrorFieldAddress", mini.ErrorFieldAddress, "address")
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := create(t, "other", container.Bag{}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Standard(t *testing.T) {
	t.Run("should connect with the defaults", func(t *testing.T) {
		o := options(t, container.Bag{}, nil)
		testutil.Equal(t, "Network", o.Network, "tcp")
		testutil.Equal(t, "Addr", o.Addr, "127.0.0.1:6379")
		testutil.Equal(t, "Username", o.Username, "")
		testutil.Equal(t, "Password", o.Password, "")
		testutil.Equal(t, "DB", o.DB, 0)
	})

	t.Run("should connect to the host and port", func(t *testing.T) {
		o := options(t, container.Bag{
			standard.ConfigFieldHost:  "::1",
			standard.ConfigFieldPort:  " 6380 ",
			redis.ConfigFieldUsername: "admin",
			redis.ConfigFieldPassword: "p@ss:w/rd",
			redis.ConfigFieldDatabase: 3.0,
		}, nil)
		testutil.Equal(t, "Addr", o.Addr, "[::1]:6380")
		testutil.Equal(t, "Username", o.Username, "admin")
		testutil.Equal(t, "Password", o.Password, "p@ss:w/rd")
		testutil.Equal(t, "DB", o.DB, 3)
	})

	t.Run("should connect to the socket of the unix network", func(t *testing.T) {
		o := options(t, container.Bag{standard.ConfigFieldNetwork: "unix", standard.ConfigFieldHost: "/tmp/redis.sock"}, nil)
		testutil.Equal(t, "Network", o.Network, "unix")
		testutil.Equal(t, "Addr", o.Addr, "/tmp/redis.sock")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		o := options(t, container.Bag{standard.ConfigFieldHost: nil}, map[string]any{
			standard.DefaultHostConfigPath:     "cache",
			standard.DefaultPortConfigPath:     "7000",
			standard.DefaultUsernameConfigPath: "user",
			standard.DefaultPasswordConfigPath: "secret",
			standard.DefaultDatabaseConfigPath: uint8(5),
		})
		testutil.Equal(t, "Addr", o.Addr, "cache:7000")
		testutil.Equal(t, "Username", o.Username, "user")
		testutil.Equal(t, "Password", o.Password, "secret")
		testutil.Equal(t, "DB", o.DB, 5)
	})

	t.Run("should set the options of the shared fields", func(t *testing.T) {
		o := options(t, container.Bag{
			redis.ConfigFieldProtocol:              2,
			redis.ConfigFieldClientName:            "app",
			redis.ConfigFieldMaxRetries:            -1,
			redis.ConfigFieldMinRetryBackoff:       "-1s",
			redis.ConfigFieldMaxRetryBackoff:       time.Second,
			redis.ConfigFieldDialTimeout:           " 2s ",
			redis.ConfigFieldReadTimeout:           "-5m",
			redis.ConfigFieldWriteTimeout:          "4s",
			redis.ConfigFieldContextTimeoutEnabled: true,
			redis.ConfigFieldReadBufferSize:        int64(1024),
			redis.ConfigFieldWriteBufferSize:       uint16(2048),
			redis.ConfigFieldPoolFIFO:              true,
			redis.ConfigFieldPoolSize:              int32(20),
			redis.ConfigFieldPoolTimeout:           "5s",
			redis.ConfigFieldMinIdleConns:          float32(2),
			redis.ConfigFieldMaxIdleConns:          int8(8),
			redis.ConfigFieldMaxActiveConns:        uint32(30),
			redis.ConfigFieldConnMaxIdleTime:       "-1ns",
			redis.ConfigFieldConnMaxLifetime:       "1h",
			redis.ConfigFieldDisableIdentity:       true,
			redis.ConfigFieldIdentitySuffix:        "flam",
		}, nil)
		testutil.Equal(t, "Protocol", o.Protocol, 2)
		testutil.Equal(t, "ClientName", o.ClientName, "app")
		testutil.Equal(t, "MaxRetries", o.MaxRetries, 0) // -1, read by go-redis as none
		testutil.Equal(t, "MinRetryBackoff", o.MinRetryBackoff, time.Duration(0))
		testutil.Equal(t, "MaxRetryBackoff", o.MaxRetryBackoff, time.Second)
		testutil.Equal(t, "DialTimeout", o.DialTimeout, 2*time.Second)
		testutil.Equal(t, "ReadTimeout", o.ReadTimeout, time.Duration(0)) // -1, read by go-redis as none
		testutil.Equal(t, "WriteTimeout", o.WriteTimeout, 4*time.Second)
		testutil.Equal(t, "ContextTimeoutEnabled", o.ContextTimeoutEnabled, true)
		testutil.Equal(t, "ReadBufferSize", o.ReadBufferSize, 1024)
		testutil.Equal(t, "WriteBufferSize", o.WriteBufferSize, 2048)
		testutil.Equal(t, "PoolFIFO", o.PoolFIFO, true)
		testutil.Equal(t, "PoolSize", o.PoolSize, 20)
		testutil.Equal(t, "PoolTimeout", o.PoolTimeout, 5*time.Second)
		testutil.Equal(t, "MinIdleConns", o.MinIdleConns, 2)
		testutil.Equal(t, "MaxIdleConns", o.MaxIdleConns, 8)
		testutil.Equal(t, "MaxActiveConns", o.MaxActiveConns, 30)
		testutil.Equal(t, "ConnMaxIdleTime", o.ConnMaxIdleTime, time.Duration(-1))
		testutil.Equal(t, "ConnMaxLifetime", o.ConnMaxLifetime, time.Hour)
		testutil.Equal(t, "DisableIdentity", o.DisableIdentity, true)
		testutil.Equal(t, "IdentitySuffix", o.IdentitySuffix, "flam")
	})

	t.Run("should not reach the server", func(t *testing.T) {
		conn, err := create(t, standard.Driver, container.Bag{standard.ConfigFieldPort: 1, redis.ConfigFieldMaxRetries: -1}, nil)
		testutil.NoError(t, err)
		if err := conn.Ping(context.Background()).Err(); err == nil {
			t.Fatal("Ping() did not fail")
		}
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			standard.ConfigFieldNetwork:            "",
			standard.ConfigFieldHost:               3,
			standard.ConfigFieldPort:               0,
			redis.ConfigFieldUsername:              1,
			redis.ConfigFieldPassword:              true,
			redis.ConfigFieldDatabase:              -1,
			redis.ConfigFieldProtocol:              4,
			redis.ConfigFieldClientName:            []any{},
			redis.ConfigFieldMaxRetries:            "3",
			redis.ConfigFieldMinRetryBackoff:       "soon",
			redis.ConfigFieldMaxRetryBackoff:       5,
			redis.ConfigFieldDialTimeout:           "1x",
			redis.ConfigFieldReadTimeout:           1.5,
			redis.ConfigFieldWriteTimeout:          false,
			redis.ConfigFieldContextTimeoutEnabled: "true",
			redis.ConfigFieldReadBufferSize:        1.5,
			redis.ConfigFieldWriteBufferSize:       uint64(math.MaxUint64),
			redis.ConfigFieldPoolFIFO:              1,
			redis.ConfigFieldPoolSize:              math.Inf(1),
			redis.ConfigFieldPoolTimeout:           "",
			redis.ConfigFieldMinIdleConns:          struct{}{},
			redis.ConfigFieldMaxIdleConns:          "8",
			redis.ConfigFieldMaxActiveConns:        container.Bag{},
			redis.ConfigFieldConnMaxIdleTime:       time.Now(),
			redis.ConfigFieldConnMaxLifetime:       []string{"1h"},
			redis.ConfigFieldDisableIdentity:       "yes",
			redis.ConfigFieldIdentitySuffix:        2,
		} {
			_, err := create(t, standard.Driver, container.Bag{field: val}, nil)
			assertInvalid(t, err, standard.ErrInvalidField, field)
		}
	})

	t.Run("should refuse the negative values go-redis cannot take", func(t *testing.T) {
		for field, val := range map[string]any{
			redis.ConfigFieldMaxRetries:      -2,
			redis.ConfigFieldReadBufferSize:  -1,
			redis.ConfigFieldWriteBufferSize: -1,
			redis.ConfigFieldPoolSize:        -1,
			redis.ConfigFieldMinIdleConns:    -1,
			redis.ConfigFieldMaxIdleConns:    -1,
			redis.ConfigFieldMaxActiveConns:  -1,
			redis.ConfigFieldDialTimeout:     "-1s",
			redis.ConfigFieldPoolTimeout:     -time.Second,
		} {
			for _, driver := range []string{standard.Driver, mini.Driver} {
				_, err := create(t, driver, container.Bag{field: val}, nil)
				sentinel := standard.ErrInvalidField
				if driver == mini.Driver {
					sentinel = mini.ErrInvalidField
				}
				assertInvalid(t, err, sentinel, field)
			}
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		_, err := create(t, standard.Driver, container.Bag{}, map[string]any{standard.DefaultPortConfigPath: 70000})
		assertInvalid(t, err, standard.ErrInvalidField, standard.ConfigFieldPort)
	})

	t.Run("should read the ports of every kind", func(t *testing.T) {
		for _, val := range []any{int8(80), uint(80), float32(80), " 80 "} {
			testutil.Equal(t, "Addr", options(t, container.Bag{standard.ConfigFieldPort: val}, nil).Addr, "127.0.0.1:80")
		}
		for _, val := range []any{"x", 1.5, -1, uint(0), "65536", struct{}{}} {
			_, err := create(t, standard.Driver, container.Bag{standard.ConfigFieldPort: val}, nil)
			assertInvalid(t, err, standard.ErrInvalidField, standard.ConfigFieldPort)
		}
	})

	t.Run("should report the first invalid field", func(t *testing.T) {
		_, err := create(t, standard.Driver, container.Bag{
			standard.ConfigFieldHost:  "",
			redis.ConfigFieldDatabase: -1,
		}, nil)
		assertInvalid(t, err, standard.ErrInvalidField, standard.ConfigFieldHost)
	})
}

func Test_Mini(t *testing.T) {
	ctx := context.Background()

	t.Run("should run the commands on a server of its own", func(t *testing.T) {
		f := connectionFactory(t, mini.Driver, container.Bag{redis.ConfigFieldDatabase: 1}, nil)
		conn, err := f.Get("main")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.NoError(t, conn.Set(ctx, "key", "value", time.Minute).Err())

		server, ok := mini.Server(conn)
		testutil.Equal(t, "Server() ok", ok, true)
		testutil.Equal(t, "Addr", conn.Options().Addr, server.Addr())
		server.Select(1)
		got, err := server.Get("key")
		testutil.NoError(t, err)
		testutil.Equal(t, "server Get()", got, "value")
		server.FastForward(time.Minute)
		if err := conn.Get(ctx, "key").Err(); !errors.Is(err, goredis.Nil) {
			t.Fatalf("Get() after expiry = %v, want redis.Nil", err)
		}

		other, err := f.Generate("main")
		testutil.NoError(t, err)
		defer func() { _ = other.Close() }()
		otherServer, _ := mini.Server(other)
		testutil.Equal(t, "own server", otherServer != server, true)
	})

	t.Run("should stop its server when closed", func(t *testing.T) {
		conn, err := create(t, mini.Driver, container.Bag{}, nil)
		testutil.NoError(t, err)
		server, _ := mini.Server(conn)
		addr := server.Addr()

		testutil.NoError(t, conn.Close())
		if c, err := net.Dial("tcp", addr); err == nil {
			_ = c.Close()
			t.Fatal("Dial() after Close did not fail")
		}
		if err := conn.Close(); !errors.Is(err, goredis.ErrClosed) {
			t.Fatalf("Close() again = %v, want redis.ErrClosed", err)
		}
	})

	t.Run("should require the password", func(t *testing.T) {
		conn, err := create(t, mini.Driver, container.Bag{redis.ConfigFieldPassword: "secret"}, nil)
		testutil.NoError(t, err)
		testutil.NoError(t, conn.Ping(ctx).Err())

		client := goredis.NewClient(&goredis.Options{Addr: conn.Options().Addr})
		defer func() { _ = client.Close() }()
		if err := client.Ping(ctx).Err(); err == nil {
			t.Fatal("Ping() without password did not fail")
		}
	})

	t.Run("should require the username and password", func(t *testing.T) {
		conn, err := create(t, mini.Driver, container.Bag{
			redis.ConfigFieldUsername: "user",
			redis.ConfigFieldPassword: "secret",
		}, nil)
		testutil.NoError(t, err)
		testutil.NoError(t, conn.Ping(ctx).Err())

		client := goredis.NewClient(&goredis.Options{Addr: conn.Options().Addr, Password: "secret"})
		defer func() { _ = client.Close() }()
		if err := client.Ping(ctx).Err(); err == nil {
			t.Fatal("Ping() without username did not fail")
		}
	})

	t.Run("should listen on the address", func(t *testing.T) {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		testutil.NoError(t, err)
		addr := listener.Addr().String()
		testutil.NoError(t, listener.Close())

		conn, err := create(t, mini.Driver, container.Bag{mini.ConfigFieldAddress: addr}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Addr", conn.Options().Addr, addr)
		testutil.NoError(t, conn.Ping(ctx).Err())
	})

	t.Run("should fail when the server cannot listen", func(t *testing.T) {
		_, err := create(t, mini.Driver, container.Bag{}, map[string]any{mini.DefaultAddressConfigPath: "nowhere"})
		var e *flam.Error
		if !errors.Is(err, mini.ErrStart) || !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("Get() = %v, want mini.ErrStart", err)
		}
		testutil.Context(t, e, map[string]any{mini.ErrorFieldID: "main", mini.ErrorFieldAddress: "nowhere"})
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			mini.ConfigFieldAddress:   "",
			redis.ConfigFieldDatabase: "1",
		} {
			_, err := create(t, mini.Driver, container.Bag{field: val}, nil)
			assertInvalid(t, err, mini.ErrInvalidField, field)
		}
	})

	t.Run("should not return the server of other connections", func(t *testing.T) {
		conn, err := create(t, standard.Driver, container.Bag{}, nil)
		testutil.NoError(t, err)
		server, ok := mini.Server(conn)
		testutil.Equal(t, "Server() ok", ok, false)
		testutil.Equal(t, "Server()", server == nil, true)
	})
}
