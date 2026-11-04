package serializers

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// user is the object of the tests.
type user struct {
	Id     uint
	Name   string
	secret string
}

// newSerializer returns the serializer of the id of an app with the cache
// and JSON bundles, and the values.
func newSerializer(t *testing.T, id string, values map[string]any) (flam.ICacheSerializer, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), cache.NewBundle(), json.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var s flam.ICacheSerializer
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ICacheSerializerFactory) { s, err = f.Get(id) }))
	return s, err
}

func Test_Bundles(t *testing.T) {
	t.Run("should configure the default serializer", func(t *testing.T) {
		b := json.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.serializers.json")
		testutil.Equal(t, "Id", json.Id, "json")
		testutil.Equal(t, "Driver", json.Driver, "flam.cache.serializers.driver.json")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "driver", bag.Get(cache.SerializerConfigPath+".json.driver"), json.Driver)
		testutil.Equal(t, "default", bag.Get(cache.DefaultSerializerConfigPath), "json")
		for _, broken := range []container.Bag{{"flam": "scalar"}, {"flam": container.Bag{"cache": container.Bag{"defaults": "scalar"}}}} {
			if err := b.(flam.IConfigurableBundle).Config(broken); err == nil {
				t.Fatal("Config() did not fail")
			}
		}

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.cache.serializers.json", "provider/1"})
	})

	t.Run("should register every serializer driver", func(t *testing.T) {
		b := serializers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.serializers")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.cache.serializers", "bundle:flam.cache.serializers.json", "provider/1",
		})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := newSerializer(t, "other", map[string]any{
			cache.SerializerConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
		})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_JSON(t *testing.T) {
	s, err := newSerializer(t, json.Id, nil)
	testutil.NoError(t, err)
	testutil.Equal(t, "Id()", s.Id(), "json")

	t.Run("should serialize the exported fields", func(t *testing.T) {
		data, err := s.Serialize(&user{Id: 7, Name: "ann", secret: "x"})
		testutil.NoError(t, err)
		testutil.Equal(t, "data", data, `{"Id":7,"Name":"ann"}`)
	})

	t.Run("should deserialize into the pointer", func(t *testing.T) {
		got := &user{Id: 7, secret: "kept"}
		testutil.NoError(t, s.Deserialize(`{"Id":7,"Name":"ann"}`, got))
		testutil.Equal(t, "object", got, &user{Id: 7, Name: "ann", secret: "kept"})
	})

	t.Run("should fail to serialize", func(t *testing.T) {
		_, err := s.Serialize(make(chan int))
		testutil.Error(t, err, cache.ErrSerialize,
			"unable to serialize json cache value: unable to serialize cache value: json: unsupported type: chan int",
			map[string]any{cache.ErrorFieldID: "json", cache.ErrorFieldType: "chan int"})
	})

	t.Run("should fail to deserialize", func(t *testing.T) {
		testutil.Error(t, s.Deserialize(`{"Id":"x"}`, &user{}), cache.ErrDeserialize,
			"unable to deserialize json cache value: unable to deserialize cache value: json: cannot unmarshal string into Go struct field user.Id of type uint",
			map[string]any{cache.ErrorFieldID: "json", cache.ErrorFieldType: "*serializers.user"})
	})

	t.Run("should fail to deserialize into a non pointer", func(t *testing.T) {
		for obj, typ := range map[any]string{user{}: "serializers.user", (*user)(nil): "*serializers.user", 3: "int"} {
			testutil.Error(t, s.Deserialize(`{}`, obj), cache.ErrDeserialize,
				"unable to deserialize json cache value: unable to deserialize cache value: not a non-nil pointer",
				map[string]any{cache.ErrorFieldID: "json", cache.ErrorFieldType: typ})
		}
		testutil.Error(t, s.Deserialize(`{}`, nil), cache.ErrDeserialize,
			"unable to deserialize json cache value: unable to deserialize cache value: not a non-nil pointer",
			map[string]any{cache.ErrorFieldID: "json", cache.ErrorFieldType: "<nil>"})
	})
}
