package adaptors

import (
	"context"
	"errors"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamredis "github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// user is the object of the tests.
type user struct {
	Id   uint
	Name string
	Tags map[string]bool `json:",omitempty"`
}

// newApp returns a testutil.App with the config, cache and driver bundles,
// the redis ones unless without, a mini connection "main" without retries,
// and the values.
func newApp(t *testing.T, values map[string]any, withRedis bool) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), cache.NewBundle(), json.NewBundle(), field.NewBundle(), redis.NewBundle()}
	if withRedis {
		bundles = append(bundles, flamredis.NewBundle(), mini.NewBundle())
	}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamredis.ConnectionConfigPath+".main", container.Bag{
		factory.ConfigFieldDriver:       mini.Driver,
		flamredis.ConfigFieldMaxRetries: -1,
	}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = cache.NewBundle().(flam.IClosableBundle).Close(app)
		if withRedis {
			_ = flamredis.NewBundle().(flam.IClosableBundle).Close(app)
		}
	})
	return app
}

// adaptor returns the adaptor of the fields, of the redis driver unless
// given, created by the factory of an app with the redis bundles unless
// without, and the values.
func adaptor(t *testing.T, fields container.Bag, values map[string]any, withRedis bool) (flam.ICacheAdaptor, error) {
	t.Helper()
	if _, ok := fields[factory.ConfigFieldDriver]; !ok {
		fields[factory.ConfigFieldDriver] = redis.Driver
	}
	all := map[string]any{cache.AdaptorConfigPath + ".main": fields}
	for path, value := range values {
		all[path] = value
	}
	app := newApp(t, all, withRedis)
	var a flam.ICacheAdaptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ICacheAdaptorFactory) { a, err = f.Get("main") }))
	return a, err
}

// parts returns the key generator, serializer and connection "main" of an
// app with every bundle.
func parts(t *testing.T) (flam.ICacheKeyGenerator, flam.ICacheSerializer, flam.IRedisConnection) {
	t.Helper()
	app := newApp(t, nil, true)
	var g flam.ICacheKeyGenerator
	var s flam.ICacheSerializer
	var c flam.IRedisConnection
	testutil.NoError(t, app.Invoke(func(
		gf flam.ICacheKeyGeneratorFactory, sf flam.ICacheSerializerFactory, cf flam.IRedisConnectionFactory,
	) error {
		var err error
		if g, err = gf.Get(field.Id); err != nil {
			return err
		}
		if s, err = sf.Get(json.Id); err != nil {
			return err
		}
		c, err = cf.Get("main")
		return err
	}))
	return g, s, c
}

// newAdaptor returns the adaptor "main" over the parts of an app, and its
// mini connection.
func newAdaptor(t *testing.T) (flam.ICacheAdaptor, flam.IRedisConnection) {
	t.Helper()
	g, s, c := parts(t)
	a, err := redis.NewAdaptor("main", g, s, c)
	testutil.NoError(t, err)
	return a, c
}

var errBoom = errors.New("boom")

// failingGenerator fails for the objects named "bad".
type failingGenerator struct{ flam.ICacheKeyGenerator }

func (g failingGenerator) Generate(obj any) (string, error) {
	if u, ok := obj.(*user); ok && u.Name == "bad" {
		return "", errBoom
	}
	return g.ICacheKeyGenerator.Generate(obj)
}

// failingSerializer fails for the objects named "bad", and always to
// deserialize.
type failingSerializer struct{ flam.ICacheSerializer }

func (s failingSerializer) Serialize(obj any) (string, error) {
	if u, ok := obj.(*user); ok && u.Name == "bad" {
		return "", errBoom
	}
	return s.ICacheSerializer.Serialize(obj)
}

func (failingSerializer) Deserialize(string, any) error { return errBoom }

func Test_Bundles(t *testing.T) {
	t.Run("should declare the driver, fields and paths", func(t *testing.T) {
		testutil.Equal(t, "Driver", redis.Driver, "flam.cache.adaptors.driver.redis")
		testutil.Equal(t, "ConfigFieldConnection", redis.ConfigFieldConnection, "connection")
		testutil.Equal(t, "DefaultConnectionConfigPath", redis.DefaultConnectionConfigPath, "flam.cache.defaults.adaptors.redis.connection")
		testutil.Equal(t, "ErrorFieldID", redis.ErrorFieldID, "id")
		testutil.Equal(t, "ErrorFieldField", redis.ErrorFieldField, "field")
		testutil.Equal(t, "ErrorFieldSerializer", redis.ErrorFieldSerializer, "serializer")
		testutil.Equal(t, "ErrorFieldKeyGenerator", redis.ErrorFieldKeyGenerator, "key_generator")
		testutil.Equal(t, "ErrorFieldConnection", redis.ErrorFieldConnection, "connection")
	})

	t.Run("should provide the creator", func(t *testing.T) {
		b := redis.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.adaptors.redis")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.cache.adaptors.redis", "provider/1"})
	})

	t.Run("should register every adaptor driver", func(t *testing.T) {
		b := adaptors.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.adaptors")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.cache.adaptors", "bundle:flam.cache.adaptors.redis", "provider/1",
			"bundle:flam.cache.adaptors.memory", "provider/1",
		})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := adaptor(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, true)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should create the adaptor of the ids, or of their defaults", func(t *testing.T) {
		a, err := adaptor(t, container.Bag{redis.ConfigFieldConnection: "main"}, nil, true)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", a.Id(), "main")

		a, err = adaptor(t, container.Bag{
			cache.ConfigFieldSerializer:   nil,
			cache.ConfigFieldKeyGenerator: "field",
		}, map[string]any{redis.DefaultConnectionConfigPath: "main"}, true)
		testutil.NoError(t, err)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			cache.ConfigFieldSerializer:   {cache.ConfigFieldSerializer: "", redis.ConfigFieldConnection: "main"},
			cache.ConfigFieldKeyGenerator: {cache.ConfigFieldKeyGenerator: 3, redis.ConfigFieldConnection: "main"},
			redis.ConfigFieldConnection:   {},
		} {
			_, err := adaptor(t, fields, nil, true)
			testutil.Error(t, errors.Unwrap(err), redis.ErrInvalidField,
				"unable to create redis cache adaptor: invalid redis cache adaptor field",
				map[string]any{redis.ErrorFieldID: "main", redis.ErrorFieldField: name})
		}
	})

	t.Run("should fail with an unknown serializer, key generator or connection", func(t *testing.T) {
		for _, c := range []struct {
			fields   container.Bag
			sentinel error
			field    string
			value    string
		}{
			{container.Bag{cache.ConfigFieldSerializer: "nope", redis.ConfigFieldConnection: "main"}, redis.ErrSerializer, redis.ErrorFieldSerializer, "nope"},
			{container.Bag{cache.ConfigFieldKeyGenerator: "nope", redis.ConfigFieldConnection: "main"}, redis.ErrKeyGenerator, redis.ErrorFieldKeyGenerator, "nope"},
			{container.Bag{redis.ConfigFieldConnection: "nope"}, redis.ErrConnection, redis.ErrorFieldConnection, "nope"},
		} {
			_, err := adaptor(t, c.fields, nil, true)
			if !errors.Is(err, c.sentinel) || !errors.Is(err, factory.ErrUnknownID) {
				t.Fatalf("Get() = %v, want %v and factory.ErrUnknownID", err, c.sentinel)
			}
			var e *flam.Error
			errors.As(errors.Unwrap(err), &e)
			testutil.Context(t, e, map[string]any{redis.ErrorFieldID: "main", c.field: c.value})
		}
	})

	t.Run("should fail without the redis connection factory", func(t *testing.T) {
		_, err := adaptor(t, container.Bag{redis.ConfigFieldConnection: "main"}, nil, false)
		testutil.Error(t, errors.Unwrap(err), redis.ErrConnection,
			"unable to create redis cache adaptor: unable to get redis cache adaptor connection: no redis connection factory",
			map[string]any{redis.ErrorFieldID: "main", redis.ErrorFieldConnection: "main"})
	})
}

func Test_NewAdaptor(t *testing.T) {
	g, s, c := parts(t)
	for _, tc := range []struct {
		g        flam.ICacheKeyGenerator
		s        flam.ICacheSerializer
		c        flam.IRedisConnection
		sentinel error
		text     string
	}{
		{nil, s, c, redis.ErrNilKeyGenerator, "nil redis cache adaptor key generator"},
		{g, nil, c, redis.ErrNilSerializer, "nil redis cache adaptor serializer"},
		{g, s, nil, redis.ErrNilConnection, "nil redis cache adaptor connection"},
	} {
		a, err := redis.NewAdaptor("main", tc.g, tc.s, tc.c)
		testutil.Error(t, err, tc.sentinel, "unable to create redis cache adaptor: "+tc.text,
			map[string]any{redis.ErrorFieldID: "main"})
		testutil.Equal(t, "adaptor", a, nil)
	}
}

func Test_Adaptor(t *testing.T) {
	t.Run("should set, check, get and delete the objects", func(t *testing.T) {
		a, c := newAdaptor(t)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}, &user{Id: 2, Name: "bob"}))
		server, _ := mini.Server(c)
		got, err := server.Get("adaptors.user:1")
		testutil.NoError(t, err)
		testutil.Equal(t, "stored", got, `{"Id":1,"Name":"ann"}`)

		has, err := a.Has(context.Background(), &user{Id: 1}, &user{Id: 3}, &user{Id: 1})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", has, []bool{true, false, true})

		first, missing := &user{Id: 1}, &user{Id: 3, Name: "kept"}
		found, err := a.Get(context.Background(), first, missing)
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", found, []any{first, nil})
		testutil.Equal(t, "filled", first, &user{Id: 1, Name: "ann"})
		testutil.Equal(t, "unchanged", missing, &user{Id: 3, Name: "kept"})

		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}, &user{Id: 3}))
		has, err = a.Has(context.Background(), &user{Id: 1}, &user{Id: 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has() after Del", has, []bool{false, true})
	})

	t.Run("should replace the objects, instead of merging in them", func(t *testing.T) {
		a, _ := newAdaptor(t)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Tags: map[string]bool{"new": true}}))

		got := &user{Id: 1, Name: "stale", Tags: map[string]bool{"stale": true}}
		_, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "object", got, &user{Id: 1, Tags: map[string]bool{"new": true}})
	})

	t.Run("should change no object when one fails to decode", func(t *testing.T) {
		a, c := newAdaptor(t)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}))
		server, _ := mini.Server(c)
		testutil.NoError(t, server.Set("adaptors.user:2", "{broken"))

		first, second := &user{Id: 1}, &user{Id: 2, Name: "kept"}
		_, err := a.Get(context.Background(), first, second)
		if !errors.Is(err, cache.ErrDeserialize) {
			t.Fatalf("Get() = %v, want cache.ErrDeserialize", err)
		}
		testutil.Equal(t, "first", first, &user{Id: 1})
		testutil.Equal(t, "second", second, &user{Id: 2, Name: "kept"})
	})

	t.Run("should fail for an object that is not a pointer", func(t *testing.T) {
		a, _ := newAdaptor(t)
		testutil.NoError(t, a.Set(context.Background(), user{Id: 1}))
		_, err := a.Get(context.Background(), user{Id: 1})
		if !errors.Is(err, cache.ErrDeserialize) {
			t.Fatalf("Get() = %v, want cache.ErrDeserialize", err)
		}
	})

	t.Run("should expire the ephemeral objects", func(t *testing.T) {
		a, c := newAdaptor(t)
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 1, Name: "ann"}, &user{Id: 2}))
		server, _ := mini.Server(c)
		testutil.Equal(t, "TTL", server.TTL("adaptors.user:1"), time.Minute)
		testutil.Equal(t, "TTL", server.TTL("adaptors.user:2"), time.Minute)

		server.FastForward(time.Minute)
		has, err := a.Has(context.Background(), &user{Id: 1}, &user{Id: 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has() after expiry", has, []bool{false, false})
	})

	t.Run("should do nothing for no objects", func(t *testing.T) {
		a, _ := newAdaptor(t)
		has, err := a.Has(context.Background())
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", has, []bool{})
		found, err := a.Get(context.Background())
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", found, []any{})
		testutil.NoError(t, a.Set(context.Background()))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Second))
		testutil.NoError(t, a.Del(context.Background()))
	})

	t.Run("should replace a nil context with context.Background", func(t *testing.T) {
		a, _ := newAdaptor(t)
		var ctx context.Context
		testutil.NoError(t, a.Set(ctx, &user{Id: 1, Name: "ann"}))
		testutil.NoError(t, a.SetEphemeral(ctx, time.Minute, &user{Id: 2}))
		has, err := a.Has(ctx, &user{Id: 1}, &user{Id: 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", has, []bool{true, true})
		got := &user{Id: 1}
		_, err = a.Get(ctx, got)
		testutil.NoError(t, err)
		testutil.Equal(t, "Name", got.Name, "ann")
		testutil.NoError(t, a.Del(ctx, &user{Id: 1}))
	})

	t.Run("should fail with a cancelled context", func(t *testing.T) {
		a, _ := newAdaptor(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, hasErr := a.Has(ctx, &user{Id: 1})
		_, getErr := a.Get(ctx, &user{Id: 1})
		for message, err := range map[string]error{
			"Has()":          hasErr,
			"Get()":          getErr,
			"Set()":          a.Set(ctx, &user{Id: 1}),
			"SetEphemeral()": a.SetEphemeral(ctx, time.Second, &user{Id: 1}),
			"Del()":          a.Del(ctx, &user{Id: 1}),
		} {
			if !errors.Is(err, cache.ErrStorage) || !errors.Is(err, context.Canceled) {
				t.Fatalf("%s = %v, want cache.ErrStorage and context.Canceled", message, err)
			}
		}
	})

	t.Run("should refuse a ttl that is not positive", func(t *testing.T) {
		a, _ := newAdaptor(t)
		for _, ttl := range []time.Duration{0, -time.Second} {
			testutil.Error(t, a.SetEphemeral(context.Background(), ttl, &user{Id: 1}), redis.ErrInvalidTTL,
				"unable to set ephemeral in redis cache: invalid redis cache adaptor ttl",
				map[string]any{redis.ErrorFieldID: "main"})
		}
	})

	t.Run("should fail with the errors of the key generator", func(t *testing.T) {
		g, s, c := parts(t)
		a, err := redis.NewAdaptor("main", failingGenerator{g}, s, c)
		testutil.NoError(t, err)
		bad := &user{Name: "bad"}
		_, hasErr := a.Has(context.Background(), bad)
		_, getErr := a.Get(context.Background(), bad)
		for message, err := range map[string]error{
			"unable to check redis cache":            hasErr,
			"unable to get from redis cache":         getErr,
			"unable to set in redis cache":           a.Set(context.Background(), &user{Id: 1}, bad),
			"unable to set ephemeral in redis cache": a.SetEphemeral(context.Background(), time.Second, bad),
			"unable to delete from redis cache":      a.Del(context.Background(), bad),
		} {
			testutil.Error(t, err, errBoom, message+": boom", map[string]any{redis.ErrorFieldID: "main"})
		}
	})

	t.Run("should fail with the errors of the serializer, with the key", func(t *testing.T) {
		g, s, c := parts(t)
		a, err := redis.NewAdaptor("main", g, failingSerializer{s}, c)
		testutil.NoError(t, err)
		bad := &user{Id: 9, Name: "bad"}
		testutil.Error(t, a.Set(context.Background(), bad), errBoom, "unable to set in redis cache: boom",
			map[string]any{redis.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:9"})
		testutil.Error(t, a.SetEphemeral(context.Background(), time.Second, bad), errBoom, "unable to set ephemeral in redis cache: boom",
			map[string]any{redis.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:9"})

		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		_, err = a.Get(context.Background(), &user{Id: 1})
		testutil.Error(t, err, errBoom, "unable to get from redis cache: boom",
			map[string]any{redis.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:1"})
	})

	t.Run("should fail with the errors of the store", func(t *testing.T) {
		g, s, _ := parts(t)
		c, err := flamredis.NewConnection("dead", goredis.NewClient(&goredis.Options{
			Addr: "127.0.0.1:1", MaxRetries: -1, DialerRetries: 1,
		}))
		testutil.NoError(t, err)
		defer func() { _ = c.Close() }()
		a, err := redis.NewAdaptor("main", g, s, c)
		testutil.NoError(t, err)

		_, hasErr := a.Has(context.Background(), &user{Id: 1})
		_, getErr := a.Get(context.Background(), &user{Id: 1})
		for message, err := range map[string]error{
			"unable to check redis cache":            hasErr,
			"unable to get from redis cache":         getErr,
			"unable to set in redis cache":           a.Set(context.Background(), &user{Id: 1}),
			"unable to set ephemeral in redis cache": a.SetEphemeral(context.Background(), time.Second, &user{Id: 1}),
			"unable to delete from redis cache":      a.Del(context.Background(), &user{Id: 1}),
		} {
			if !errors.Is(err, cache.ErrStorage) {
				t.Fatalf("%s: error = %v, want cache.ErrStorage", message, err)
			}
			var e *flam.Error
			errors.As(err, &e)
			testutil.Equal(t, "message", e.Error()[:len(message)], message)
			testutil.Context(t, e, map[string]any{redis.ErrorFieldID: "main"})
		}
	})
}
