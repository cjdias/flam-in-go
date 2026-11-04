package key_generators

import (
	"errors"
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newGenerator returns the key generator "main" of the fields, created by
// the factory of an app with the cache and field bundles.
func newGenerator(t *testing.T, fields container.Bag) (flam.ICacheKeyGenerator, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), cache.NewBundle(), field.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if _, ok := fields[factory.ConfigFieldDriver]; !ok {
		fields[factory.ConfigFieldDriver] = field.Driver
	}
	testutil.NoError(t, bag.Set(cache.KeyGeneratorConfigPath+".main", fields))
	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var g flam.ICacheKeyGenerator
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ICacheKeyGeneratorFactory) { g, err = f.Get("main") }))
	return g, err
}

// generate returns the key of the object with the generator of the fields.
func generate(t *testing.T, fields container.Bag, obj any) (string, error) {
	t.Helper()
	g, err := newGenerator(t, fields)
	testutil.NoError(t, err)
	return g.Generate(obj)
}

type user struct {
	flam.Record
	Name string
}

type labelled int

func (l labelled) String() string { return "label-" + string(rune('0'+l)) }

// pid formats itself with a pointer receiver only.
type pid int

func (p *pid) String() string { return "pid-" + strconv.Itoa(int(*p)) }

type badText struct{}

func (badText) MarshalText() ([]byte, error) { return nil, errors.New("bad") }

type keys struct {
	Id     uint
	Text   string
	Signed int8
	Ptr    *int
	PtrPtr **string
	Label  labelled
	At     time.Time
	Addr   netip.Addr
	Bad    badText
	Float  float64
	Nil    *int
	NilPtr **string
	hidden int // looked up by name, to fail as unexported
	Pid    pid
	Any    any
	AnyPtr any
	NilAny any
	*user
}

func Test_Bundles(t *testing.T) {
	t.Run("should configure the default key generator", func(t *testing.T) {
		b := field.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.key_generators.field")
		testutil.Equal(t, "Id", field.Id, "field")
		testutil.Equal(t, "Driver", field.Driver, "flam.cache.key_generators.driver.field")
		testutil.Equal(t, "ConfigFieldField", field.ConfigFieldField, "field")
		testutil.Equal(t, "ConfigFieldPrefix", field.ConfigFieldPrefix, "prefix")
		testutil.Equal(t, "ConfigFieldType", field.ConfigFieldType, "type")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "driver", bag.Get(cache.KeyGeneratorConfigPath+".field.driver"), field.Driver)
		testutil.Equal(t, "default", bag.Get(cache.DefaultKeyGeneratorConfigPath), "field")
		for _, broken := range []container.Bag{{"flam": "scalar"}, {"flam": container.Bag{"cache": container.Bag{"defaults": "scalar"}}}} {
			if err := b.(flam.IConfigurableBundle).Config(broken); err == nil {
				t.Fatal("Config() did not fail")
			}
		}

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.cache.key_generators.field", "provider/1"})
	})

	t.Run("should register every key generator driver", func(t *testing.T) {
		b := key_generators.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.key_generators")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.cache.key_generators", "bundle:flam.cache.key_generators.field", "provider/1",
		})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := newGenerator(t, container.Bag{factory.ConfigFieldDriver: "other"})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Field(t *testing.T) {
	t.Run("should key by the type and the promoted id", func(t *testing.T) {
		g, err := newGenerator(t, container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", g.Id(), "main")
		for _, obj := range []any{user{Record: flam.Record{Id: 7}}, &user{Record: flam.Record{Id: 7}}} {
			key, err := g.Generate(obj)
			testutil.NoError(t, err)
			testutil.Equal(t, "key", key, "key_generators.user:7")
		}
		u := &user{Record: flam.Record{Id: 7}}
		key, err := g.Generate(&u)
		testutil.NoError(t, err)
		testutil.Equal(t, "key of a pointer to a pointer", key, "key_generators.user:7")
	})

	t.Run("should key by the field, after the prefix, with or without the type", func(t *testing.T) {
		key, err := generate(t, container.Bag{field.ConfigFieldField: "Name", field.ConfigFieldPrefix: "app:"}, &user{Name: "ann"})
		testutil.NoError(t, err)
		testutil.Equal(t, "key", key, "app:key_generators.user:ann")

		key, err = generate(t, container.Bag{field.ConfigFieldField: "Name", field.ConfigFieldType: false}, &user{Name: "ann"})
		testutil.NoError(t, err)
		testutil.Equal(t, "key", key, "ann")
	})

	t.Run("should format the values of every supported kind", func(t *testing.T) {
		n, s := 42, "deep"
		ps := &s
		obj := &keys{
			Id: 1, Text: "t", Signed: -3, Ptr: &n, PtrPtr: &ps, Label: 2,
			At: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Addr: netip.MustParseAddr("10.0.0.1"),
		}
		for name, want := range map[string]string{
			"Id": "1", "Text": "t", "Signed": "-3", "Ptr": "42", "PtrPtr": "deep", "Label": "label-2",
			"At": "2026-01-02T03:04:05Z", "Addr": "10.0.0.1",
		} {
			key, err := generate(t, container.Bag{field.ConfigFieldField: name, field.ConfigFieldType: false}, obj)
			testutil.NoError(t, err)
			testutil.Equal(t, name, key, want)
		}
	})

	t.Run("should format through pointer receivers and interfaces", func(t *testing.T) {
		n := 8
		obj := &keys{Pid: 5, Any: 7, AnyPtr: &n, hidden: 1}
		for name, want := range map[string]string{"Pid": "pid-5", "Any": "7", "AnyPtr": "8"} {
			key, err := generate(t, container.Bag{field.ConfigFieldField: name, field.ConfigFieldType: false}, obj)
			testutil.NoError(t, err)
			testutil.Equal(t, name, key, want)
		}
		_, err := generate(t, container.Bag{field.ConfigFieldField: "NilAny"}, obj)
		testutil.Error(t, err, cache.ErrKey, "unable to generate field cache key: unable to generate cache key: field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
			map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldType: "*key_generators.keys"})
	})

	t.Run("should key the objects of an unnamed type only untyped", func(t *testing.T) {
		obj := struct{ Id int }{Id: 3}
		_, err := generate(t, container.Bag{}, obj)
		testutil.Error(t, err, cache.ErrKey, "unable to generate field cache key: unable to generate cache key: unnamed type",
			map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldType: "struct { Id int }"})
		key, err := generate(t, container.Bag{field.ConfigFieldType: false}, obj)
		testutil.NoError(t, err)
		testutil.Equal(t, "key", key, "3")
	})

	t.Run("should fail for the values it cannot key", func(t *testing.T) {
		var nilPtr *string
		obj := &keys{NilPtr: &nilPtr}
		for name, cause := range map[string]string{
			"Missing": "no such field", "hidden": "no such field", "Name": "field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
			"Bad":    "field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
			"Float":  "field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
			"Nil":    "field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
			"NilPtr": "field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler",
		} {
			_, err := generate(t, container.Bag{field.ConfigFieldField: name}, obj)
			if name == "Name" { // promoted through the nil *user
				cause = "nil object"
			}
			testutil.Error(t, err, cache.ErrKey, "unable to generate field cache key: unable to generate cache key: "+cause,
				map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldType: "*key_generators.keys"})
		}
	})

	t.Run("should fail for the objects that are not structs", func(t *testing.T) {
		g, err := newGenerator(t, container.Bag{})
		testutil.NoError(t, err)
		var nilUser *user
		for _, c := range []struct {
			obj   any
			typ   string
			cause string
		}{
			{nil, "<nil>", "nil object"},
			{nilUser, "*key_generators.user", "nil object"},
			{&nilUser, "**key_generators.user", "nil object"},
			{7, "int", "not a struct"},
			{map[string]int{}, "map[string]int", "not a struct"},
		} {
			_, err := g.Generate(c.obj)
			testutil.Error(t, err, cache.ErrKey, "unable to generate field cache key: unable to generate cache key: "+c.cause,
				map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldType: c.typ})
		}
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for name, val := range map[string]any{
			field.ConfigFieldField:  "",
			field.ConfigFieldPrefix: 3,
			field.ConfigFieldType:   "yes",
		} {
			_, err := newGenerator(t, container.Bag{name: val})
			var e *flam.Error
			if !errors.Is(err, field.ErrInvalidField) || !errors.As(errors.Unwrap(err), &e) {
				t.Fatalf("%s: Get() = %v, want field.ErrInvalidField", name, err)
			}
			testutil.Context(t, e, map[string]any{field.ErrorFieldID: "main", field.ErrorFieldField: name})
		}
	})
}
