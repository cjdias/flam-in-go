package flam

import (
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// compile time check that container.Bag implements IBag
var _ flam.IBag = container.Bag{}

func Test_IBag(t *testing.T) {
	t.Run("should read and change the bag it holds", func(t *testing.T) {
		b := container.Bag{}
		var i flam.IBag = b

		testutil.NoError(t, i.Set("a.b", 1))
		testutil.NoError(t, i.Set("s", "v"))
		testutil.NoError(t, i.Set("d", time.Second))
		testutil.NoError(t, i.Set("tmp", true))
		testutil.NoError(t, i.Delete("tmp"))
		merged, err := i.Merge("m", container.Bag{"x.y": 2})
		testutil.NoError(t, err)

		testutil.Equal(t, "bag", b, container.Bag{
			"a": container.Bag{"b": 1}, "s": "v", "d": time.Second,
			"m": container.Bag{"x": container.Bag{"y": 2}},
		})
		if !testutil.SamePointer(merged, b) {
			t.Error("Merge() through IBag did not return the bag")
		}
		if !i.Has("a.b") || i.Get("a.b") != 1 || i.Int("a.b") != 1 || i.String("s") != "v" ||
			i.Duration("d") != time.Second || i.Int("m.x.y") != 2 || !i.Bool("missing", true) {
			t.Error("a getter through IBag did not return the stored value")
		}
		if !testutil.SamePointer(i.Bag("a"), b["a"]) {
			t.Error("Bag() through IBag copied the nested bag")
		}
		testutil.Equal(t, "Clone()", i.Clone(), b)
	})

	t.Run("should expose every typed getter", func(t *testing.T) {
		var i flam.IBag = container.Bag{
			"i8": int8(1), "i16": int16(2), "i32": int32(3), "i64": int64(4),
			"u": uint(5), "u8": uint8(6), "u16": uint16(7), "u32": uint32(8), "u64": uint64(9),
			"f32": float32(1.5), "f64": 2.5, "r": 'x', "sl": []any{1}, "ss": []string{"a"},
		}
		if i.Int8("i8") != 1 || i.Int16("i16") != 2 || i.Int32("i32") != 3 || i.Int64("i64") != 4 ||
			i.Uint("u") != 5 || i.Uint8("u8") != 6 || i.Uint16("u16") != 7 || i.Uint32("u32") != 8 ||
			i.Uint64("u64") != 9 || i.Float32("f32") != 1.5 || i.Float64("f64") != 2.5 || i.Rune("r") != 'x' {
			t.Error("a numeric getter through IBag did not return the stored value")
		}
		testutil.Equal(t, "Slice()", i.Slice("sl"), []any{1})
		testutil.Equal(t, "StringSlice()", i.StringSlice("ss"), []string{"a"})
	})

	t.Run("should keep the nil bag behaviour", func(t *testing.T) {
		var i flam.IBag = container.Bag(nil)
		testutil.Error(t, i.Set("a", 1), container.ErrNil, "unable to set bag value: nil value", map[string]any{"path": "a"})
		got, err := i.Merge("", container.Bag{"a": 1})
		testutil.NoError(t, err)
		testutil.Equal(t, "Merge()", got, container.Bag{"a": 1})
	})
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

// configOnly, bootOnly, runOnly and closeOnly implement a single optional
// bundle interface each, without being an IBundle.
type (
	configOnly struct{}
	bootOnly   struct{}
	runOnly    struct{}
	closeOnly  struct{}
)

func (configOnly) Config(flam.IBag) error { return nil }
func (bootOnly) Boot(flam.IApp) error     { return nil }
func (runOnly) Run(flam.IApp) error       { return nil }
func (closeOnly) Close(flam.IApp) error   { return nil }

// compile time checks that the optional bundle interfaces do not embed
// IBundle, so that a bundle opts in to each of them on its own
var (
	_ flam.IConfigurableBundle = configOnly{}
	_ flam.IBootableBundle     = bootOnly{}
	_ flam.IRunnableBundle     = runOnly{}
	_ flam.IClosableBundle     = closeOnly{}
)

func Test_Declarations(t *testing.T) {
	t.Run("should declare IDiskCreator as the IResourceCreator of IDisk", func(t *testing.T) {
		testutil.Equal(t, "IDiskCreator", reflect.TypeFor[flam.IDiskCreator](),
			reflect.TypeFor[flam.IResourceCreator[flam.IDisk]]())
	})

	t.Run("should declare IDiskFactory as the IFactory of IDisk", func(t *testing.T) {
		testutil.Equal(t, "IDiskFactory", reflect.TypeFor[flam.IDiskFactory](),
			reflect.TypeFor[flam.IFactory[flam.IDisk]]())
	})

	t.Run("should declare the config creators and factories as aliases", func(t *testing.T) {
		for name, pair := range map[string][2]reflect.Type{
			"IConfigParserCreator": {reflect.TypeFor[flam.IConfigParserCreator](),
				reflect.TypeFor[flam.IResourceCreator[flam.IConfigParser]]()},
			"IConfigParserFactory": {reflect.TypeFor[flam.IConfigParserFactory](),
				reflect.TypeFor[flam.IFactory[flam.IConfigParser]]()},
			"IConfigSourceCreator": {reflect.TypeFor[flam.IConfigSourceCreator](),
				reflect.TypeFor[flam.IResourceCreator[flam.IConfigSource]]()},
			"IConfigSourceFactory": {reflect.TypeFor[flam.IConfigSourceFactory](),
				reflect.TypeFor[flam.IFactory[flam.IConfigSource]]()},
		} {
			testutil.Equal(t, name, pair[0], pair[1])
		}
	})

	t.Run("should declare a reloadable source as a source", func(t *testing.T) {
		var _ flam.IConfigSource = flam.IConfigReloadableSource(nil)
	})

	t.Run("should accept an *http.Client as an IConfigRestClient", func(t *testing.T) {
		var _ flam.IConfigRestClient = (*http.Client)(nil)
	})

	t.Run("should declare IDisk as an afero.Fs and an IResource", func(t *testing.T) {
		disk := reflect.TypeFor[flam.IDisk]()
		for _, embedded := range []reflect.Type{reflect.TypeFor[afero.Fs](), reflect.TypeFor[flam.IResource]()} {
			if !disk.Implements(embedded) {
				t.Errorf("IDisk does not implement %v", embedded)
			}
		}
		testutil.Equal(t, "IDisk methods", disk.NumMethod(), reflect.TypeFor[afero.Fs]().NumMethod()+1)
	})

	t.Run("should declare IResource with Id only, closing being optional", func(t *testing.T) {
		resource := reflect.TypeFor[flam.IResource]()
		testutil.Equal(t, "IResource methods", resource.NumMethod(), 1)
		if resource.Implements(reflect.TypeFor[io.Closer]()) {
			t.Error("IResource requires io.Closer")
		}
	})

	t.Run("should declare ConfigObserver as a function of the old and new values", func(t *testing.T) {
		observer := reflect.TypeFor[flam.ConfigObserver]()
		testutil.Equal(t, "kind", observer.Kind(), reflect.Func)
		testutil.Equal(t, "inputs", []reflect.Type{observer.In(0), observer.In(1)},
			[]reflect.Type{reflect.TypeFor[any](), reflect.TypeFor[any]()})
		testutil.Equal(t, "input count", observer.NumIn(), 2)
		testutil.Equal(t, "output count", observer.NumOut(), 0)
	})

	t.Run("should declare IBundle with Id and Register only", func(t *testing.T) {
		b := reflect.TypeFor[flam.IBundle]()
		testutil.Equal(t, "IBundle methods", b.NumMethod(), 2)
		for _, optional := range []reflect.Type{
			reflect.TypeFor[flam.IConfigurableBundle](), reflect.TypeFor[flam.IBootableBundle](),
			reflect.TypeFor[flam.IRunnableBundle](), reflect.TypeFor[flam.IClosableBundle](),
		} {
			if b.Implements(optional) {
				t.Errorf("IBundle requires %v", optional)
			}
		}
	})
}
