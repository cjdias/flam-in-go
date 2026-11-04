package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newApp returns a testutil.App with the config and cache bundles.
func newApp(t *testing.T) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(cache.NewBundle()))
	return a
}

var errBoom = errors.New("boom")

// failing is a serializer and a key generator failing to close.
type failing struct{}

func (failing) Id() string                    { return "bad" }
func (failing) Close() error                  { return errBoom }
func (failing) Serialize(any) (string, error) { return "", nil }
func (failing) Deserialize(string, any) error { return nil }
func (failing) Generate(any) (string, error)  { return "", nil }

// failingAdaptor adapts failing to flam.ICacheAdaptor.
type failingAdaptor struct{ flam.ICacheAdaptor }

func (failingAdaptor) Id() string   { return "bad" }
func (failingAdaptor) Close() error { return errBoom }

type user struct {
	flam.Record
	Name string
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "SerializerConfigPath", cache.SerializerConfigPath, "flam.cache.serializers")
	testutil.Equal(t, "SerializerCreatorGroup", cache.SerializerCreatorGroup, "flam.cache.serializers.creator")
	testutil.Equal(t, "KeyGeneratorConfigPath", cache.KeyGeneratorConfigPath, "flam.cache.key_generators")
	testutil.Equal(t, "KeyGeneratorCreatorGroup", cache.KeyGeneratorCreatorGroup, "flam.cache.key_generators.creator")
	testutil.Equal(t, "AdaptorConfigPath", cache.AdaptorConfigPath, "flam.cache.adaptors")
	testutil.Equal(t, "AdaptorCreatorGroup", cache.AdaptorCreatorGroup, "flam.cache.adaptors.creator")
	testutil.Equal(t, "ConfigFieldSerializer", cache.ConfigFieldSerializer, "serializer")
	testutil.Equal(t, "ConfigFieldKeyGenerator", cache.ConfigFieldKeyGenerator, "key_generator")
	testutil.Equal(t, "DefaultSerializerConfigPath", cache.DefaultSerializerConfigPath, "flam.cache.defaults.adaptors.serializer")
	testutil.Equal(t, "DefaultKeyGeneratorConfigPath", cache.DefaultKeyGeneratorConfigPath, "flam.cache.defaults.adaptors.key_generator")
	testutil.Equal(t, "ErrorFieldID", cache.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldType", cache.ErrorFieldType, "type")
	testutil.Equal(t, "ErrorFieldKey", cache.ErrorFieldKey, "key")
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories", func(t *testing.T) {
		b := cache.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache")

		a := newApp(t)
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-4:], []string{"bundle:flam.cache", "provider/0", "provider/0", "provider/0"})
		testutil.NoError(t, a.Invoke(func(
			s flam.ICacheSerializerFactory, g flam.ICacheKeyGeneratorFactory, ad flam.ICacheAdaptorFactory,
		) {
			testutil.Equal(t, "serializers", s.Available(), []string{})
			testutil.Equal(t, "key generators", g.Available(), []string{})
			testutil.Equal(t, "adaptors", ad.Available(), []string{})
		}))
	})

	t.Run("should close every factory, joining their errors", func(t *testing.T) {
		a := newApp(t)
		var s flam.ICacheSerializerFactory
		var g flam.ICacheKeyGeneratorFactory
		var ad flam.ICacheAdaptorFactory
		testutil.NoError(t, a.Invoke(func(
			sf flam.ICacheSerializerFactory, gf flam.ICacheKeyGeneratorFactory, af flam.ICacheAdaptorFactory,
		) {
			s, g, ad = sf, gf, af
		}))
		testutil.NoError(t, s.Store("bad", failing{}))
		testutil.NoError(t, g.Store("bad", failing{}))
		testutil.NoError(t, ad.Store("bad", failingAdaptor{}))

		err := cache.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
		testutil.Equal(t, "errors", len(err.(interface{ Unwrap() []error }).Unwrap()), 3)
		for name, f := range map[string]interface{ Has(string) bool }{"serializers": s, "key generators": g, "adaptors": ad} {
			testutil.Equal(t, name+" closed", f.Has("bad"), false)
		}
		testutil.NoError(t, cache.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(cache.NewBundle()))
		if err := cache.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

func Test_Application(t *testing.T) {
	values := container.Bag{}
	testutil.NoError(t, values.Set(redis.ConnectionConfigPath+".main", container.Bag{factory.ConfigFieldDriver: mini.Driver}))
	testutil.NoError(t, values.Set(cache.AdaptorConfigPath+".main", container.Bag{
		factory.ConfigFieldDriver: "flam.cache.adaptors.driver.redis",
		"connection":              "main",
	}))
	a := app.New(values)
	for _, b := range []flam.IBundle{
		redis.NewBundle(), connections.NewBundle(),
		cache.NewBundle(), serializers.NewBundle(), key_generators.NewBundle(), adaptors.NewBundle(),
	} {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Boot())

	testutil.NoError(t, a.Invoke(func(f flam.ICacheAdaptorFactory) error {
		adaptor, err := f.Get("main")
		if err != nil {
			return err
		}
		if err := adaptor.Set(context.Background(), &user{Record: flam.Record{Id: 7}, Name: "ann"}); err != nil {
			return err
		}
		got := &user{Record: flam.Record{Id: 7}}
		found, err := adaptor.Get(context.Background(), got)
		testutil.Equal(t, "Get()", found, []any{got})
		testutil.Equal(t, "Name", got.Name, "ann")
		return err
	}))
	testutil.NoError(t, a.Close())
}
