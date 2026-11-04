package container

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type Bag = container.Bag

// named types, which the typed getters must not accept for their base type
type (
	namedInt    int
	namedString string
	namedBool   bool
	namedFloat  float64
)

// named container types, which are not bags nor the slices the slice
// getters accept, and which Clone does not copy
type (
	namedMap         map[string]any
	namedAnySlice    []any
	namedStringSlice []string
)

// emptyKey is a map key type whose values print as an empty string.
type emptyKey struct{}

func (emptyKey) String() string { return "" }

// labelKey is a map key type whose values all print as "label".
type labelKey struct{ n int }

func (labelKey) String() string { return "label" }

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "BagPathSeparator", container.BagPathSeparator, ".")
	testutil.Equal(t, "ErrorFieldPath", container.ErrorFieldPath, "path")
	testutil.Equal(t, "ErrorFieldSegment", container.ErrorFieldSegment, "segment")
	testutil.Equal(t, "ErrorFieldKey", container.ErrorFieldKey, "key")

	sentinels := map[string]error{
		"invalid path":    container.ErrInvalidPath,
		"path conflict":   container.ErrPathConflict,
		"nil value":       container.ErrNil,
		"unsupported bag": container.ErrUnsupportedBag,
	}
	for text, err := range sentinels {
		testutil.Equal(t, "Error()", err.Error(), text)
		for otherText, other := range sentinels {
			if text != otherText && errors.Is(err, other) {
				t.Errorf("%q matches %q", text, otherText)
			}
		}
	}
	testutil.Equal(t, "fmt.Sprint", fmt.Sprint(Bag{"a": 1}), "map[a:1]") // String(path) is not a fmt.Stringer
}

// ---------------------------------------------------------------------------
// Has and Get
// ---------------------------------------------------------------------------

func Test_Has(t *testing.T) {
	b := Bag{
		"a":     Bag{"b": nil, "c": 1},
		"m":     map[string]any{"k": "v"},
		"y":     map[any]any{"k": "v"},
		"leaf":  1,
		"empty": Bag(nil),
		"":      "empty key",
		"x.y":   "dotted key",
	}
	tests := []struct {
		path string
		want bool
	}{
		{"", true}, {"a", true}, {"a.b", true}, {"a.c", true}, {"a.x", false}, {"a.b.c", false},
		{"m.k", true}, {"m.x", false}, {"y", true}, {"y.k", false}, {"leaf", true}, {"leaf.x", false},
		{"empty", true}, {"empty.x", false}, {"missing", false}, {"missing.x", false},
		{"x.y", false}, {"a..b", false}, {".a", false}, {"a.", false}, {".", false}, {"..", false},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%q", tc.path), func(t *testing.T) {
			if got := b.Has(tc.path); got != tc.want {
				t.Errorf("Has(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}

	t.Run("should handle a nil bag", func(t *testing.T) {
		if !Bag(nil).Has("") || Bag(nil).Has("a") {
			t.Error("Has() on a nil bag is wrong")
		}
	})

	t.Run("should accept segments made of spaces", func(t *testing.T) {
		if !(Bag{"a": Bag{" ": Bag{"b": 1}}}).Has("a. .b") {
			t.Error(`Has("a. .b") = false`)
		}
	})
}

func Test_Get(t *testing.T) {
	nested := Bag{"k": "v", "n": nil}
	b := Bag{"nested": nested, "a": Bag{"b": "nested b"}, "a.b": "dotted key", "": "empty key"}

	if got := b.Get("nested.k"); got != "v" {
		t.Errorf(`Get("nested.k") = %v`, got)
	}
	if got := b.Get("a.b"); got != "nested b" {
		t.Errorf(`Get("a.b") = %v, want the nested value, not the dotted key`, got)
	}
	if got := b.Get("missing"); got != nil {
		t.Errorf(`Get("missing") = %v, want nil`, got)
	}
	for _, path := range []string{"missing", "a..b", "nested.k.x"} {
		if got := b.Get(path, "def", "ignored"); got != "def" {
			t.Errorf("Get(%q, def) = %v, want the first default", path, got)
		}
	}
	if got := b.Get("nested.n", "def"); got != nil {
		t.Errorf(`Get("nested.n", "def") = %v, want the stored nil`, got)
	}
	if !testutil.SamePointer(b.Get(""), b) || !testutil.SamePointer(b.Get("nested"), nested) {
		t.Error("Get() copied a stored bag, or did not return the bag itself for the empty path")
	}
	if got, ok := Bag(nil).Get("").(Bag); !ok || got != nil {
		t.Errorf(`Get("") on a nil bag = %#v, want a nil Bag`, got)
	}
}

func Test_HasAndGet_Paths(t *testing.T) {
	deep := map[string]any{"d": 42}
	b := Bag{
		"a":        map[string]any{"b": Bag{"c": deep}},
		"named":    namedMap{"k": 1},
		"strMap":   map[string]string{"k": "v"},
		"list":     []any{Bag{"k": 1}},
		"nilMap":   map[string]any(nil),
		"nilValue": nil,
	}

	t.Run("should follow Bag and map[string]any levels at any depth", func(t *testing.T) {
		if !b.Has("a.b.c.d") || b.Get("a.b.c.d") != 42 {
			t.Errorf(`Get("a.b.c.d") = %v, want 42`, b.Get("a.b.c.d"))
		}
		if !testutil.SamePointer(b.Get("a.b.c"), deep) {
			t.Error(`Get("a.b.c") did not return the stored map`)
		}
	})

	t.Run("should not follow values that are not bags", func(t *testing.T) {
		for _, path := range []string{"named.k", "strMap.k", "list.0", "list.0.k", "nilMap.k", "nilValue.k"} {
			if b.Has(path) {
				t.Errorf("Has(%q) = true, want false", path)
			}
			testutil.Equal(t, "Get("+path+", def)", b.Get(path, "def"), "def")
		}
	})

	t.Run("should report the empty slots themselves as stored", func(t *testing.T) {
		for _, path := range []string{"nilMap", "nilValue"} {
			if !b.Has(path) {
				t.Errorf("Has(%q) = false, want true", path)
			}
		}
		if got, ok := b.Get("nilMap", "def").(map[string]any); !ok || got != nil {
			t.Errorf(`Get("nilMap", def) = %#v, want the stored nil map`, got)
		}
	})

	t.Run("should return nil for a missing value without default, ignoring extra defaults", func(t *testing.T) {
		if got := b.Get("a..b"); got != nil {
			t.Errorf(`Get("a..b") = %#v, want nil`, got)
		}
		if got := b.Get("missing", nil, "ignored"); got != nil {
			t.Errorf(`Get("missing", nil, ignored) = %#v, want the first default, nil`, got)
		}
	})

	t.Run("should be case sensitive", func(t *testing.T) {
		if (Bag{"Key": 1}).Has("key") {
			t.Error(`Has("key") = true on the key "Key"`)
		}
	})
}

// ---------------------------------------------------------------------------
// typed getters
// ---------------------------------------------------------------------------

type getterCase struct {
	name   string
	stored any
	def    any
	zero   any
	get    func(b Bag, path string) any
	getDef func(b Bag, path string) any // passes def, then stored as an ignored extra
}

func newGetter[T comparable](name string, stored, def T, fn func(Bag, string, ...T) T) getterCase {
	var zero T
	return getterCase{
		name: name, stored: stored, def: def, zero: zero,
		get:    func(b Bag, path string) any { return fn(b, path) },
		getDef: func(b Bag, path string) any { return fn(b, path, def, stored) },
	}
}

var getterCases = []getterCase{
	newGetter("Bool", true, true, Bag.Bool),
	newGetter("Int", 1, 7, Bag.Int),
	newGetter("Int8", int8(2), int8(8), Bag.Int8),
	newGetter("Int16", int16(3), int16(9), Bag.Int16),
	newGetter("Int32", int32(4), int32(10), Bag.Int32),
	newGetter("Int64", int64(5), int64(11), Bag.Int64),
	newGetter("Uint", uint(6), uint(12), Bag.Uint),
	newGetter("Uint8", uint8(7), uint8(13), Bag.Uint8),
	newGetter("Uint16", uint16(8), uint16(14), Bag.Uint16),
	newGetter("Uint32", uint32(9), uint32(15), Bag.Uint32),
	newGetter("Uint64", uint64(10), uint64(16), Bag.Uint64),
	newGetter("Float32", float32(1.5), float32(7.5), Bag.Float32),
	newGetter("Float64", 2.5, 8.5, Bag.Float64),
	newGetter("Rune", 'x', 'd', Bag.Rune),
	newGetter("String", "s", "d", Bag.String),
	newGetter("Duration", time.Second, time.Minute, Bag.Duration),
}

// otherValues holds a value of every type a getter might be tempted to
// convert from; each getter must reject those of another type.
var otherValues = []any{
	true, 1, int8(1), int16(1), int32(1), int64(1), uint(1), uint8(1), uint16(1), uint32(1), uint64(1),
	float32(1), float64(1), "1", "true", "5s", "x", []byte("x"), time.Duration(1),
	namedInt(1), namedString("s"), namedBool(true), namedFloat(1),
	[]any{1}, []string{"s"}, Bag{}, map[string]any{}, map[any]any{}, struct{}{}, labelKey{1}, nil,
}

func Test_TypedGetters(t *testing.T) {
	for _, g := range getterCases {
		t.Run(g.name, func(t *testing.T) {
			for name, b := range map[string]Bag{
				"top level":           {"v": g.stored},
				"in a Bag":            {"n": Bag{"v": g.stored}},
				"in a map[string]any": {"n": map[string]any{"v": g.stored}},
			} {
				path := "n.v"
				if name == "top level" {
					path = "v"
				}
				if got := g.get(b, path); got != g.stored {
					t.Errorf("%s: %s = %#v, want %#v", name, g.name, got, g.stored)
				}
				if got := g.getDef(b, path); got != g.stored {
					t.Errorf("%s: %s with a default = %#v, want %#v", name, g.name, got, g.stored)
				}
			}

			b := Bag{"leaf": g.stored, "nil": nil, "y": map[any]any{"v": g.stored}}
			for _, path := range []string{"missing", "missing.v", "leaf.v", "a..b", ".", "nil", "y.v"} {
				if got := g.get(b, path); got != g.zero {
					t.Errorf("%s(%q) = %#v, want the zero value", g.name, path, got)
				}
				if got := g.getDef(b, path); got != g.def {
					t.Errorf("%s(%q, def) = %#v, want the first default", g.name, path, got)
				}
			}
			if g.get(Bag(nil), "v") != g.zero || g.getDef(Bag(nil), "v") != g.def {
				t.Errorf("%s on a nil bag did not return the zero value or the default", g.name)
			}

			storedType := fmt.Sprintf("%T", g.stored)
			for _, other := range otherValues {
				if fmt.Sprintf("%T", other) == storedType {
					continue
				}
				b := Bag{"v": other}
				if got := g.get(b, "v"); got != g.zero {
					t.Errorf("%s converted %T(%v) to %#v", g.name, other, other, got)
				}
				if got := g.getDef(b, "v"); got != g.def {
					t.Errorf("%s with a default converted %T(%v) to %#v", g.name, other, other, got)
				}
			}
		})
	}

	t.Run("should treat rune and byte as their aliases", func(t *testing.T) {
		b := Bag{"r": 'x', "i": int32(65), "by": byte(1)}
		if b.Int32("r") != 'x' || b.Rune("i") != 'A' || b.Uint8("by") != 1 {
			t.Error("an alias type was not returned by the aliased getter")
		}
	})
}

func Test_TypedGetters_Boundaries(t *testing.T) {
	// each value is stored, then read with a default that differs from it:
	// the stored value, even a zero one, must win over the default
	tests := []struct {
		name   string
		stored any
		get    func(b Bag) any
	}{
		{"should return false rather than the default", false, func(b Bag) any { return b.Bool("v", true) }},
		{"should return int 0 rather than the default", 0, func(b Bag) any { return b.Int("v", 1) }},
		{"should return math.MinInt", math.MinInt, func(b Bag) any { return b.Int("v", 1) }},
		{"should return math.MaxInt", math.MaxInt, func(b Bag) any { return b.Int("v", 1) }},
		{"should return a negative int", -1, func(b Bag) any { return b.Int("v", 1) }},
		{"should return math.MinInt8", int8(math.MinInt8), func(b Bag) any { return b.Int8("v", 1) }},
		{"should return math.MaxInt8", int8(math.MaxInt8), func(b Bag) any { return b.Int8("v", 1) }},
		{"should return int8 0", int8(0), func(b Bag) any { return b.Int8("v", 1) }},
		{"should return math.MinInt16", int16(math.MinInt16), func(b Bag) any { return b.Int16("v", 1) }},
		{"should return math.MaxInt16", int16(math.MaxInt16), func(b Bag) any { return b.Int16("v", 1) }},
		{"should return math.MinInt32", int32(math.MinInt32), func(b Bag) any { return b.Int32("v", 1) }},
		{"should return math.MaxInt32", int32(math.MaxInt32), func(b Bag) any { return b.Int32("v", 1) }},
		{"should return math.MinInt64", int64(math.MinInt64), func(b Bag) any { return b.Int64("v", 1) }},
		{"should return math.MaxInt64", int64(math.MaxInt64), func(b Bag) any { return b.Int64("v", 1) }},
		{"should return uint 0", uint(0), func(b Bag) any { return b.Uint("v", 1) }},
		{"should return math.MaxUint", uint(math.MaxUint), func(b Bag) any { return b.Uint("v", 1) }},
		{"should return math.MaxUint8", uint8(math.MaxUint8), func(b Bag) any { return b.Uint8("v", 1) }},
		{"should return math.MaxUint16", uint16(math.MaxUint16), func(b Bag) any { return b.Uint16("v", 1) }},
		{"should return math.MaxUint32", uint32(math.MaxUint32), func(b Bag) any { return b.Uint32("v", 1) }},
		{"should return math.MaxUint64", uint64(math.MaxUint64), func(b Bag) any { return b.Uint64("v", 1) }},
		{"should return math.MaxFloat32", float32(math.MaxFloat32), func(b Bag) any { return b.Float32("v", 1) }},
		{"should return -math.MaxFloat32", float32(-math.MaxFloat32), func(b Bag) any { return b.Float32("v", 1) }},
		{"should return the smallest float32", float32(math.SmallestNonzeroFloat32), func(b Bag) any { return b.Float32("v", 1) }},
		{"should return float32 +Inf", float32(math.Inf(1)), func(b Bag) any { return b.Float32("v", 1) }},
		{"should return math.MaxFloat64", math.MaxFloat64, func(b Bag) any { return b.Float64("v", 1) }},
		{"should return the smallest float64", math.SmallestNonzeroFloat64, func(b Bag) any { return b.Float64("v", 1) }},
		{"should return float64 -Inf", math.Inf(-1), func(b Bag) any { return b.Float64("v", 1) }},
		{"should return float64 0", 0.0, func(b Bag) any { return b.Float64("v", 1) }},
		{"should return rune 0", rune(0), func(b Bag) any { return b.Rune("v", 'd') }},
		{"should return unicode.MaxRune", unicode.MaxRune, func(b Bag) any { return b.Rune("v", 'd') }},
		{"should return utf8.RuneError", utf8.RuneError, func(b Bag) any { return b.Rune("v", 'd') }},
		{"should return a negative rune", rune(-1), func(b Bag) any { return b.Rune("v", 'd') }},
		{"should return the empty string", "", func(b Bag) any { return b.String("v", "d") }},
		{"should return a multi-byte string", "ünïcødé ✓", func(b Bag) any { return b.String("v", "d") }},
		{"should return duration 0", time.Duration(0), func(b Bag) any { return b.Duration("v", time.Second) }},
		{"should return a negative duration", -time.Second, func(b Bag) any { return b.Duration("v", time.Second) }},
		{"should return the minimum duration", time.Duration(math.MinInt64), func(b Bag) any { return b.Duration("v", time.Second) }},
		{"should return the maximum duration", time.Duration(math.MaxInt64), func(b Bag) any { return b.Duration("v", time.Second) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Equal(t, "getter", tc.get(Bag{"v": tc.stored}), tc.stored)
		})
	}

	t.Run("should return NaN as stored", func(t *testing.T) {
		b := Bag{"f32": float32(math.NaN()), "f64": math.NaN()}
		if got := b.Float32("f32", 1); !math.IsNaN(float64(got)) {
			t.Errorf("Float32() = %v, want NaN", got)
		}
		if got := b.Float64("f64", 1); !math.IsNaN(got) {
			t.Errorf("Float64() = %v, want NaN", got)
		}
	})

	t.Run("should return negative zero with its sign", func(t *testing.T) {
		if got := (Bag{"v": math.Copysign(0, -1)}).Float64("v", 1); got != 0 || !math.Signbit(got) {
			t.Errorf("Float64() = %v, want -0", got)
		}
	})
}

func Test_TypedGetters_NoConversion(t *testing.T) {
	tests := []struct {
		name   string
		stored any
		get    func(b Bag) any
		def    any
	}{
		{"Bool should not parse the string \"true\"", "true", func(b Bag) any { return b.Bool("v", false) }, false},
		{"Bool should not convert the number 1", 1, func(b Bag) any { return b.Bool("v", false) }, false},
		{"Int should not convert a float64 holding an integer", 1.0, func(b Bag) any { return b.Int("v", 7) }, 7},
		{"Int should not convert an int64", int64(1), func(b Bag) any { return b.Int("v", 7) }, 7},
		{"Int should not parse a numeric string", "1", func(b Bag) any { return b.Int("v", 7) }, 7},
		{"Int8 should not convert an int that fits", 1, func(b Bag) any { return b.Int8("v", 7) }, int8(7)},
		{"Int8 should not convert an int16 that fits", int16(127), func(b Bag) any { return b.Int8("v", 7) }, int8(7)},
		{"Int8 should not truncate an overflowing int", 300, func(b Bag) any { return b.Int8("v", 7) }, int8(7)},
		{"Int64 should not convert a time.Duration", time.Second, func(b Bag) any { return b.Int64("v", 7) }, int64(7)},
		{"Int64 should not wrap a uint64 above math.MaxInt64", uint64(math.MaxUint64), func(b Bag) any { return b.Int64("v", 7) }, int64(7)},
		{"Uint should not convert a non-negative int", 1, func(b Bag) any { return b.Uint("v", 7) }, uint(7)},
		{"Uint should not wrap a negative int", -1, func(b Bag) any { return b.Uint("v", 7) }, uint(7)},
		{"Uint8 should not convert a rune", 'x', func(b Bag) any { return b.Uint8("v", 7) }, uint8(7)},
		{"Uint64 should not convert an int64", int64(math.MaxInt64), func(b Bag) any { return b.Uint64("v", 7) }, uint64(7)},
		{"Float32 should not narrow a float64", 1.5, func(b Bag) any { return b.Float32("v", 7) }, float32(7)},
		{"Float64 should not widen a float32", float32(1.5), func(b Bag) any { return b.Float64("v", 7) }, 7.0},
		{"Float64 should not convert an int", 1, func(b Bag) any { return b.Float64("v", 7) }, 7.0},
		{"Rune should not convert a single character string", "x", func(b Bag) any { return b.Rune("v", 'd') }, 'd'},
		{"Rune should not convert a byte", byte('x'), func(b Bag) any { return b.Rune("v", 'd') }, 'd'},
		{"String should not convert a []byte", []byte("x"), func(b Bag) any { return b.String("v", "d") }, "d"},
		{"String should not convert a rune", 'x', func(b Bag) any { return b.String("v", "d") }, "d"},
		{"String should not call a fmt.Stringer", labelKey{1}, func(b Bag) any { return b.String("v", "d") }, "d"},
		{"Duration should not interpret an int64 as nanoseconds", int64(5), func(b Bag) any { return b.Duration("v", time.Minute) }, time.Minute},
		{"Duration should not interpret an int as nanoseconds", 5, func(b Bag) any { return b.Duration("v", time.Minute) }, time.Minute},
		{"Duration should not parse the string \"5s\"", "5s", func(b Bag) any { return b.Duration("v", time.Minute) }, time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Equal(t, "getter", tc.get(Bag{"v": tc.stored}), tc.def)
		})
	}
}

func Test_TypedGetters_Paths(t *testing.T) {
	b := Bag{
		"a":      map[string]any{"b": Bag{"c": map[string]any{"d": 42}}},
		"named":  namedMap{"k": 42},
		"strMap": map[string]int{"k": 42},
		"nilBag": Bag(nil),
	}

	t.Run("should read through Bag and map[string]any levels at any depth", func(t *testing.T) {
		testutil.Equal(t, "Int", b.Int("a.b.c.d", 7), 42)
	})

	t.Run("should return the default for the empty path, which addresses the bag itself", func(t *testing.T) {
		testutil.Equal(t, "Int", b.Int("", 7), 7)
		testutil.Equal(t, "String", b.String("", "d"), "d")
		testutil.Equal(t, "Bool", b.Bool(""), false)
	})

	t.Run("should return the default through values that are not bags or empty slots", func(t *testing.T) {
		for _, path := range []string{"named.k", "strMap.k", "nilBag.k", "a.b.c.d.e"} {
			testutil.Equal(t, "Int("+path+")", b.Int(path, 7), 7)
		}
	})
}

func Test_TypedGetters_JSON(t *testing.T) {
	var decoded map[string]any
	testutil.NoError(t, json.Unmarshal([]byte(`{
		"server": {"port": 8080, "hosts": ["a", "b"], "tls": {"enabled": true}},
		"mixed": [1, "a"],
		"empty": [],
		"none": null
	}`), &decoded))
	b := Bag(decoded)

	t.Run("should read numbers decoded by encoding/json with Float64 only", func(t *testing.T) {
		testutil.Equal(t, "Float64", b.Float64("server.port"), 8080.0)
		testutil.Equal(t, "Int", b.Int("server.port", -1), -1)
	})

	t.Run("should navigate and read the decoded objects without conversion", func(t *testing.T) {
		testutil.Equal(t, "Bool", b.Bool("server.tls.enabled"), true)
		tls := b.Bag("server.tls")
		if _, ok := tls.(Bag); !ok || !testutil.SamePointer(tls, decoded["server"].(map[string]any)["tls"]) {
			t.Errorf("Bag() = %#v, want a Bag sharing the decoded map", tls)
		}
	})

	t.Run("should read decoded arrays of strings with StringSlice", func(t *testing.T) {
		testutil.Equal(t, "StringSlice", b.StringSlice("server.hosts"), []string{"a", "b"})
		testutil.Equal(t, "StringSlice(empty)", b.StringSlice("empty"), []string{})
		testutil.Equal(t, "StringSlice(mixed)", b.StringSlice("mixed", []string{"d"}), []string{"d"})
		testutil.Equal(t, "Slice(mixed)", b.Slice("mixed"), []any{1.0, "a"})
	})

	t.Run("should report a decoded null as stored, but return the getter defaults for it", func(t *testing.T) {
		if !b.Has("none") || b.Get("none", "def") != nil {
			t.Error("the decoded null is not a stored nil")
		}
		testutil.Equal(t, "String", b.String("none", "d"), "d")
	})

	t.Run("should write into the decoded maps in place", func(t *testing.T) {
		testutil.NoError(t, b.Set("server.tls.cert", "x"))
		testutil.Equal(t, "decoded", decoded["server"].(map[string]any)["tls"].(map[string]any)["cert"], "x")
	})
}

// ---------------------------------------------------------------------------
// Slice and StringSlice
// ---------------------------------------------------------------------------

func Test_Slice(t *testing.T) {
	anys := []any{"a", 1}
	b := Bag{
		"anys": anys, "strs": []string{"a", "b"}, "ints": []int{1}, "nilAnys": []any(nil),
		"nilStrs": []string(nil), "emptyStrs": []string{}, "leaf": 1,
	}
	if got := b.Slice("anys"); !testutil.SamePointer(got, anys) {
		t.Error("Slice() copied the stored []any")
	}
	got := b.Slice("strs")
	testutil.Equal(t, "Slice(strs)", got, []any{"a", "b"})
	got[0] = "changed"
	if b["strs"].([]string)[0] != "a" {
		t.Error("changing the converted slice changed the bag")
	}
	testutil.Equal(t, "Slice(emptyStrs)", b.Slice("emptyStrs"), []any{})
	for _, path := range []string{"nilAnys", "nilStrs"} {
		if got := b.Slice(path, []any{"d"}); got != nil {
			t.Errorf("Slice(%q) = %#v, want the stored nil", path, got)
		}
	}
	for _, path := range []string{"ints", "leaf", "missing", "leaf.x", "a..b"} {
		if got := b.Slice(path); got != nil {
			t.Errorf("Slice(%q) = %#v, want nil", path, got)
		}
		testutil.Equal(t, "Slice(def)", b.Slice(path, []any{"d"}, []any{"x"}), []any{"d"})
	}
}

func Test_StringSlice(t *testing.T) {
	strs := []string{"a", "b"}
	b := Bag{
		"strs": strs, "anys": []any{"a", "b"}, "mixed": []any{"a", 1}, "withNil": []any{"a", nil},
		"empty": []any{}, "ints": []int{1}, "nilStrs": []string(nil), "nilAnys": []any(nil), "leaf": "a",
	}
	if got := b.StringSlice("strs"); !testutil.SamePointer(got, strs) {
		t.Error("StringSlice() copied the stored []string")
	}
	got := b.StringSlice("anys")
	testutil.Equal(t, "StringSlice(anys)", got, []string{"a", "b"})
	got[0] = "changed"
	if b["anys"].([]any)[0] != "a" {
		t.Error("changing the converted slice changed the bag")
	}
	testutil.Equal(t, "StringSlice(empty)", b.StringSlice("empty"), []string{})
	for _, path := range []string{"nilStrs", "nilAnys"} {
		if got := b.StringSlice(path, []string{"d"}); got != nil {
			t.Errorf("StringSlice(%q) = %#v, want the stored nil", path, got)
		}
	}
	for _, path := range []string{"mixed", "withNil", "ints", "leaf", "missing", "leaf.x", "a..b"} {
		if got := b.StringSlice(path); got != nil {
			t.Errorf("StringSlice(%q) = %#v, want nil", path, got)
		}
		testutil.Equal(t, "StringSlice(def)", b.StringSlice(path, []string{"d"}, []string{"x"}), []string{"d"})
	}
}

func Test_Slices_EdgeCases(t *testing.T) {
	b := Bag{
		"namedAnys": namedAnySlice{"a"}, "namedStrs": namedStringSlice{"a"}, "strs": []string{"a"},
		"namedElem": []any{"a", namedString("b")}, "nested": []any{[]string{"a"}},
		"nilElem": []any{nil}, "n": map[string]any{"strs": []string{"x"}, "anys": []any{"y"}},
	}

	t.Run("should return the default for named slice types", func(t *testing.T) {
		for _, path := range []string{"namedAnys", "namedStrs"} {
			testutil.Equal(t, "Slice("+path+")", b.Slice(path, []any{"d"}), []any{"d"})
			testutil.Equal(t, "StringSlice("+path+")", b.StringSlice(path, []string{"d"}), []string{"d"})
		}
	})

	t.Run("should return the default for the empty path, which addresses the bag itself", func(t *testing.T) {
		testutil.Equal(t, "Slice", b.Slice("", []any{"d"}), []any{"d"})
		testutil.Equal(t, "StringSlice", b.StringSlice("", []string{"d"}), []string{"d"})
	})

	t.Run("should read slices nested in a map[string]any", func(t *testing.T) {
		testutil.Equal(t, "Slice", b.Slice("n.strs"), []any{"x"})
		testutil.Equal(t, "StringSlice", b.StringSlice("n.anys"), []string{"y"})
	})

	t.Run("should not convert []any elements that are not exactly strings", func(t *testing.T) {
		for _, path := range []string{"namedElem", "nested", "nilElem"} {
			testutil.Equal(t, "StringSlice("+path+")", b.StringSlice(path, []string{"d"}), []string{"d"})
		}
	})

	t.Run("should return a new []any on every conversion of a []string", func(t *testing.T) {
		if testutil.SamePointer(b.Slice("strs"), b.Slice("strs")) {
			t.Error("Slice() returned the same converted slice twice")
		}
	})

	t.Run("should return a new []string on every conversion of a []any", func(t *testing.T) {
		if testutil.SamePointer(b.StringSlice("n.anys"), b.StringSlice("n.anys")) {
			t.Error("StringSlice() returned the same converted slice twice")
		}
	})

	t.Run("should share a stored []any with the bag, changes being visible", func(t *testing.T) {
		b.Slice("nilElem")[0] = "set"
		testutil.Equal(t, "bag", b["nilElem"], []any{"set"})
	})

	t.Run("should share a stored []string with the bag, changes being visible", func(t *testing.T) {
		b.StringSlice("strs")[0] = "set"
		testutil.Equal(t, "bag", b["strs"], []string{"set"})
	})
}

// ---------------------------------------------------------------------------
// Bag
// ---------------------------------------------------------------------------

func Test_BagGetter(t *testing.T) {
	nested := Bag{"k": "v"}
	m := map[string]any{"k": "v"}
	b := Bag{
		"nested": nested, "m": m, "y": map[any]any{"k": "v"}, "leaf": 1,
		"nilBag": Bag(nil), "nilMap": map[string]any(nil),
	}
	if !testutil.SamePointer(b.Bag("nested"), nested) || !testutil.SamePointer(b.Bag(""), b) {
		t.Error("Bag() copied a stored bag, or did not return the bag itself for the empty path")
	}
	got := b.Bag("m")
	if !testutil.SamePointer(got, m) {
		t.Fatal(`Bag("m") copied the map[string]any`)
	}
	testutil.NoError(t, got.Set("new", 1))
	if m["new"] != 1 {
		t.Error("a change through the result is not visible in the map[string]any")
	}
	for _, path := range []string{"nilBag", "nilMap"} {
		if got := b.Bag(path, Bag{"d": 1}); got != nil {
			t.Errorf("Bag(%q) = %#v, want the stored nil", path, got)
		}
	}
	def := Bag{"d": 1}
	for _, path := range []string{"y", "leaf", "missing", "leaf.x", "a..b"} {
		if got := b.Bag(path); got != nil {
			t.Errorf("Bag(%q) = %#v, want nil", path, got)
		}
		if got := b.Bag(path, def, Bag{"x": 1}); !testutil.SamePointer(got, def) {
			t.Errorf("Bag(%q, def) = %#v, want the first default", path, got)
		}
	}
}

func Test_BagGetter_EdgeCases(t *testing.T) {
	deep := map[string]any{"k": 1}
	b := Bag{
		"named": namedMap{"k": 1}, "strMap": map[string]string{"k": "v"}, "nil": nil,
		"a": map[string]any{"b": Bag{"c": deep}},
	}
	def := Bag{"d": 1}

	t.Run("should return the default for map types other than Bag and map[string]any", func(t *testing.T) {
		for _, path := range []string{"named", "strMap"} {
			if got := b.Bag(path, def); !testutil.SamePointer(got, def) {
				t.Errorf("Bag(%q, def) = %#v, want the default", path, got)
			}
		}
	})

	t.Run("should return the default for a stored untyped nil, which is not a bag", func(t *testing.T) {
		if got := b.Bag("nil", def); !testutil.SamePointer(got, def) {
			t.Errorf(`Bag("nil", def) = %#v, want the default`, got)
		}
	})

	t.Run("should return a Bag sharing a map[string]any found at any depth", func(t *testing.T) {
		got := b.Bag("a.b.c")
		if _, ok := got.(Bag); !ok || !testutil.SamePointer(got, deep) {
			t.Errorf(`Bag("a.b.c") = %#v, want a Bag sharing the stored map`, got)
		}
	})

	t.Run("should return a nil flam.IBag, not the default, for the empty path of a nil bag", func(t *testing.T) {
		if got := Bag(nil).Bag("", def); got != nil {
			t.Errorf(`Bag("", def) on a nil bag = %#v, want a nil interface`, got)
		}
	})

	t.Run("should let Delete and Merge on the result change the parent bag", func(t *testing.T) {
		parent := Bag{"m": map[string]any{"k": 1}}
		child := parent.Bag("m")
		testutil.NoError(t, child.Delete("k"))
		_, err := child.Merge("", Bag{"n": 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "parent", parent, Bag{"m": map[string]any{"n": 2}})
	})
}

// ---------------------------------------------------------------------------
// Set
// ---------------------------------------------------------------------------

func Test_Set(t *testing.T) {
	t.Run("should store values, creating the intermediate bags", func(t *testing.T) {
		b := Bag{"a": Bag{"x": 1}, "y": 2}
		testutil.NoError(t, b.Set("top", 1))
		testutil.NoError(t, b.Set("a.z", 3))
		testutil.NoError(t, b.Set("new.deep.path", 4))
		testutil.Equal(t, "bag", b, Bag{
			"a": Bag{"x": 1, "z": 3}, "y": 2, "top": 1, "new": Bag{"deep": Bag{"path": 4}},
		})
	})

	t.Run("should replace any value, including a bag, and store nil", func(t *testing.T) {
		b := Bag{"a": Bag{"b": 1}, "c": 1, "n": 1}
		testutil.NoError(t, b.Set("a", 2))
		testutil.NoError(t, b.Set("c", Bag{"d": 3}))
		testutil.NoError(t, b.Set("n", nil))
		testutil.Equal(t, "bag", b, Bag{"a": 2, "c": Bag{"d": 3}, "n": nil})
	})

	t.Run("should store the value as is, without copying or converting it", func(t *testing.T) {
		list, y := []any{1}, map[any]any{"k": 1}
		b := Bag{}
		testutil.NoError(t, b.Set("list", list))
		testutil.NoError(t, b.Set("y", y))
		if !testutil.SamePointer(b["list"], list) || !testutil.SamePointer(b["y"], y) {
			t.Error("Set() copied or converted the value")
		}
	})

	t.Run("should replace empty slots on the path", func(t *testing.T) {
		for name, slot := range map[string]any{"nil": nil, "nil Bag": Bag(nil), "nil map": map[string]any(nil)} {
			b := Bag{"a": slot}
			testutil.NoError(t, b.Set("a.b.c", 1))
			testutil.Equal(t, name, b, Bag{"a": Bag{"b": Bag{"c": 1}}})
		}
	})

	t.Run("should write into a map[string]any in place", func(t *testing.T) {
		m := map[string]any{}
		b := Bag{"m": m}
		testutil.NoError(t, b.Set("m.k", 1))
		if m["k"] != 1 {
			t.Error("Set() did not write into the existing map[string]any")
		}
		if _, ok := b["m"].(map[string]any); !ok {
			t.Errorf("the map[string]any was replaced by a %T", b["m"])
		}
	})

	t.Run("should fail, unchanged, on an invalid path", func(t *testing.T) {
		for _, path := range []string{"", "a..b", ".a", "a.", ".", ".."} {
			b := Bag{"a": 1}
			testutil.Error(t, b.Set(path, 1), container.ErrInvalidPath,
				"unable to set bag value: invalid path", map[string]any{"path": path})
			testutil.Equal(t, "bag", b, Bag{"a": 1})
		}
	})

	t.Run("should fail on a nil bag, after checking the path", func(t *testing.T) {
		testutil.Error(t, Bag(nil).Set("a.b", 1), container.ErrNil,
			"unable to set bag value: nil value", map[string]any{"path": "a.b"})
		testutil.Error(t, Bag(nil).Set("", 1), container.ErrInvalidPath,
			"unable to set bag value: invalid path", map[string]any{"path": ""})
	})

	t.Run("should fail, unchanged, on a segment holding a value that is not a bag", func(t *testing.T) {
		b := Bag{"a": Bag{"b": 1}, "y": map[any]any{"k": 1}, "nilY": map[any]any(nil)}
		before := b.Clone()
		for _, tc := range []struct{ path, segment string }{
			{"a.b.c", "a.b"}, {"a.b.c.d", "a.b"}, {"y.k", "y"}, {"nilY.k", "nilY"},
		} {
			testutil.Error(t, b.Set(tc.path, 1), container.ErrPathConflict,
				"unable to set bag value: path conflict", map[string]any{"path": tc.path, "segment": tc.segment})
		}
		testutil.Equal(t, "bag", b, before)
	})

	t.Run("should report as conflicts every intermediate value that is not a bag", func(t *testing.T) {
		newBag := func() Bag {
			return Bag{
				"leaf": "s", "list": []any{Bag{}}, "named": namedMap{}, "strMap": map[string]string{},
				"m": map[string]any{"k": 1, "b": Bag{"leaf": 1}},
			}
		}
		for _, tc := range []struct{ name, path, segment string }{
			{"a string at the first segment", "leaf.x", "leaf"},
			{"a []any", "list.0", "list"},
			{"a map type based on map[string]any", "named.k", "named"},
			{"a map[string]string", "strMap.k", "strMap"},
			{"a leaf of a map[string]any", "m.k.x", "m.k"},
			{"a leaf deep below a map[string]any", "m.b.leaf.x.y", "m.b.leaf"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				b := newBag()
				testutil.Error(t, b.Set(tc.path, 1), container.ErrPathConflict,
					"unable to set bag value: path conflict", map[string]any{"path": tc.path, "segment": tc.segment})
				testutil.Equal(t, "bag", b, newBag())
			})
		}
	})

	t.Run("should check the path before the nil bag", func(t *testing.T) {
		for _, path := range []string{"a..b", ".a", "a."} {
			testutil.Error(t, Bag(nil).Set(path, 1), container.ErrInvalidPath,
				"unable to set bag value: invalid path", map[string]any{"path": path})
		}
	})

	t.Run("should create intermediate bags of type Bag below a map[string]any", func(t *testing.T) {
		m := map[string]any{}
		b := Bag{"m": m}
		testutil.NoError(t, b.Set("m.a.b", 1))
		testutil.Equal(t, "map", m, map[string]any{"a": Bag{"b": 1}})
	})

	t.Run("should not address a key containing the separator", func(t *testing.T) {
		b := Bag{"a.b": 1}
		testutil.NoError(t, b.Set("a.b", 2))
		testutil.Equal(t, "bag", b, Bag{"a.b": 1, "a": Bag{"b": 2}})
	})

	t.Run("should store a nil bag as an empty slot, replaced by a later nested Set", func(t *testing.T) {
		b := Bag{}
		testutil.NoError(t, b.Set("a", Bag(nil)))
		if !b.Has("a") || b.Has("a.b") {
			t.Fatal("the stored nil bag is not an empty slot")
		}
		testutil.NoError(t, b.Set("a.b", 1))
		testutil.Equal(t, "bag", b, Bag{"a": Bag{"b": 1}})
	})

	t.Run("should keep sibling values when creating intermediate bags", func(t *testing.T) {
		b := Bag{"a": Bag{"x": 1, "b": nil}}
		testutil.NoError(t, b.Set("a.b.c", 2))
		testutil.Equal(t, "bag", b, Bag{"a": Bag{"x": 1, "b": Bag{"c": 2}}})
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func Test_Delete(t *testing.T) {
	t.Run("should remove values, keeping emptied parents", func(t *testing.T) {
		b := Bag{"a": Bag{"b": Bag{"c": 1}}, "x": 1, "n": nil, "w": Bag{"k": 1}}
		testutil.NoError(t, b.Delete("x"))
		testutil.NoError(t, b.Delete("a.b.c"))
		testutil.NoError(t, b.Delete("n"))
		testutil.NoError(t, b.Delete("w"))
		testutil.Equal(t, "bag", b, Bag{"a": Bag{"b": Bag{}}})
	})

	t.Run("should delete from a map[string]any in place", func(t *testing.T) {
		m := map[string]any{"k": 1}
		testutil.NoError(t, Bag{"m": m}.Delete("m.k"))
		testutil.Equal(t, "map", m, map[string]any{})
	})

	t.Run("should ignore missing values, including under non-bags", func(t *testing.T) {
		y := map[any]any{"k": 1}
		b := Bag{"a": 1, "n": nil, "e": Bag(nil), "y": y}
		for _, path := range []string{"x", "x.y", "a.b", "a.b.c", "n.x", "e.x", "y.k"} {
			testutil.NoError(t, b.Delete(path))
		}
		testutil.Equal(t, "bag", b, Bag{"a": 1, "n": nil, "e": Bag(nil), "y": map[any]any{"k": 1}})
		testutil.NoError(t, Bag(nil).Delete("a"))
		testutil.NoError(t, Bag(nil).Delete("a.b"))
	})

	t.Run("should fail, unchanged, on an invalid path", func(t *testing.T) {
		for _, path := range []string{"", "a..b", ".a", "a.", "."} {
			b := Bag{"a": Bag{"b": 1}}
			testutil.Error(t, b.Delete(path), container.ErrInvalidPath,
				"unable to delete bag value: invalid path", map[string]any{"path": path})
			testutil.Equal(t, "bag", b, Bag{"a": Bag{"b": 1}})
		}
	})

	t.Run("should fail on an invalid path of a nil bag", func(t *testing.T) {
		for _, path := range []string{"", "a..b"} {
			testutil.Error(t, Bag(nil).Delete(path), container.ErrInvalidPath,
				"unable to delete bag value: invalid path", map[string]any{"path": path})
		}
	})

	t.Run("should not address a key containing the separator", func(t *testing.T) {
		b := Bag{"a.b": 1}
		testutil.NoError(t, b.Delete("a.b"))
		testutil.Equal(t, "bag", b, Bag{"a.b": 1})
	})

	t.Run("should ignore values under map types other than Bag and map[string]any", func(t *testing.T) {
		b := Bag{"named": namedMap{"k": 1}, "strMap": map[string]string{"k": "v"}}
		testutil.NoError(t, b.Delete("named.k"))
		testutil.NoError(t, b.Delete("strMap.k"))
		testutil.Equal(t, "bag", b, Bag{"named": namedMap{"k": 1}, "strMap": map[string]string{"k": "v"}})
	})

	t.Run("should delete at any depth through Bag and map[string]any levels", func(t *testing.T) {
		inner := Bag{"c": 1, "keep": 2}
		b := Bag{"a": map[string]any{"b": inner}}
		testutil.NoError(t, b.Delete("a.b.c"))
		testutil.Equal(t, "inner", inner, Bag{"keep": 2})
	})

	t.Run("should remove a stored nil, so the key no longer exists", func(t *testing.T) {
		b := Bag{"n": nil, "e": Bag(nil)}
		testutil.NoError(t, b.Delete("n"))
		testutil.NoError(t, b.Delete("e"))
		if b.Has("n") || b.Has("e") {
			t.Error("a deleted nil value is still reported by Has()")
		}
	})
}

// ---------------------------------------------------------------------------
// Merge
// ---------------------------------------------------------------------------

// ptrBag is a flam.IBag implementation used through a nil pointer.
type ptrBag struct{ flam.IBag }

func Test_Merge(t *testing.T) {
	t.Run("should deep merge, src winning on leaves and kinds", func(t *testing.T) {
		b := Bag{"a": Bag{"x": 1, "y": 1}, "keep": 1, "list": []any{1}, "bag": Bag{"k": 1}, "leaf": 1, "value": 1}
		got, err := b.Merge("", Bag{
			"a": Bag{"y": 2, "z": 2}, "list": []any{2}, "bag": 2, "leaf": Bag{"k": 2}, "value": nil,
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{
			"a": Bag{"x": 1, "y": 2, "z": 2}, "keep": 1, "list": []any{2}, "bag": 2, "leaf": Bag{"k": 2}, "value": nil,
		})
		if !testutil.SamePointer(got, b) {
			t.Error("Merge() did not return the receiver")
		}
	})

	t.Run("should merge at a path, creating it when missing", func(t *testing.T) {
		got, err := Bag{"a": Bag{"b": Bag{"x": 1}}}.Merge("a.b", Bag{"y": 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": Bag{"b": Bag{"x": 1, "y": 2}}})
		got, err = Bag{}.Merge("a.b", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": Bag{"b": Bag{}}})
	})

	t.Run("should leave the bag unchanged on a nil or empty src at the root", func(t *testing.T) {
		for _, src := range []Bag{nil, {}} {
			got, err := Bag{"a": 1}.Merge("", src)
			testutil.NoError(t, err)
			testutil.Equal(t, "bag", got, Bag{"a": 1})
		}
	})

	t.Run("should create a new bag on a nil receiver, but return it unchanged when failing", func(t *testing.T) {
		got, err := Bag(nil).Merge("a", Bag{"k": 1})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": Bag{"k": 1}})
		for path, src := range map[string]flam.IBag{"a..b": nil, "a": Bag{"x..y": 1}} {
			got, err = Bag(nil).Merge(path, src)
			if err == nil || got.(Bag) != nil {
				t.Errorf("Merge() on a nil receiver failing = (%#v, %v), want the nil bag and an error", got, err)
			}
		}
	})

	t.Run("should merge nothing from a nil IBag pointer", func(t *testing.T) {
		got, err := Bag{"a": 1}.Merge("", (*ptrBag)(nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": 1})
	})

	t.Run("should keep the values of NaN keys", func(t *testing.T) {
		got, err := Bag{}.Merge("", Bag{"y": map[any]any{math.NaN(): "v"}})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"y": Bag{"NaN": "v"}})
	})

	t.Run("should replace destination empty slots", func(t *testing.T) {
		for name, slot := range map[string]any{"nil": nil, "nil Bag": Bag(nil), "nil map": map[string]any(nil)} {
			for _, tc := range []struct {
				path string
				src  Bag
				want Bag
			}{
				{"a", Bag{"b": 1}, Bag{"a": Bag{"b": 1}}},
				{"a.b", Bag{"c": 1}, Bag{"a": Bag{"b": Bag{"c": 1}}}},
				{"", Bag{"a": Bag{"b": 1}}, Bag{"a": Bag{"b": 1}}},
			} {
				got, err := Bag{"a": slot}.Merge(tc.path, tc.src)
				testutil.NoError(t, err)
				testutil.Equal(t, name+" at "+tc.path, got, tc.want)
			}
		}
	})

	t.Run("should not let nil or empty src bags erase destination bags", func(t *testing.T) {
		got, err := Bag{"a": Bag{"k": 1}, "b": Bag{"k": 1}, "m": Bag{"k": 1}}.
			Merge("", Bag{"a": Bag(nil), "b": Bag{}, "m": map[string]any(nil), "c": Bag(nil)})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": Bag{"k": 1}, "b": Bag{"k": 1}, "m": Bag{"k": 1}, "c": Bag(nil)})
	})

	t.Run("should merge into a map[string]any in place, keeping src types", func(t *testing.T) {
		m := map[string]any{"k": 1}
		got, err := Bag{"m": m}.Merge("", Bag{
			"m": Bag{"n": 2}, "s": map[string]any{"b": Bag{"x": 1}, "nil": map[string]any(nil)},
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "map", m, map[string]any{"k": 1, "n": 2})
		testutil.Equal(t, "s", got.(Bag)["s"], map[string]any{"b": Bag{"x": 1}, "nil": map[string]any(nil)})
	})

	t.Run("should treat a destination map[any]any as a plain value", func(t *testing.T) {
		got, err := Bag{"y": map[any]any{"k": 1}}.Merge("", Bag{"y": Bag{"n": 1}})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"y": Bag{"n": 1}})
	})

	t.Run("should expand dotted keys, in sorted order, the last one winning", func(t *testing.T) {
		tests := []struct {
			src, want Bag
		}{
			{Bag{"server.port": 8080, "server": Bag{"host": "h"}, "db": Bag{"primary.port": 1}},
				Bag{"server": Bag{"port": 8080, "host": "h"}, "db": Bag{"primary": Bag{"port": 1}}}},
			{Bag{"a": 1, "a.b": 2}, Bag{"a": Bag{"b": 2}}},
			{Bag{"a.b": 1, "a": Bag{"b": 2}}, Bag{"a": Bag{"b": 1}}},
			{Bag{"a.b": Bag{"x": 1}, "a": Bag{"b": Bag{"y": 2}}}, Bag{"a": Bag{"b": Bag{"x": 1, "y": 2}}}},
			{Bag{"a.b.c": Bag{"x": 1}, "a": Bag{"b.c.y": 2}}, Bag{"a": Bag{"b": Bag{"c": Bag{"x": 1, "y": 2}}}}},
			{Bag{"a": nil, "a.b": 2}, Bag{"a": Bag{"b": 2}}},
		}
		for _, tc := range tests {
			for range 20 { // map order must not matter
				got, err := Bag{}.Merge("", tc.src)
				testutil.NoError(t, err)
				testutil.Equal(t, fmt.Sprint(tc.src), got, tc.want)
			}
		}
	})

	t.Run("should convert map[any]any bags of src, resolving keys in a fixed order", func(t *testing.T) {
		for range 50 {
			got, err := Bag{}.Merge("", Bag{
				"y":     map[any]any{"k": 1, 2: "two", "a.b": 3, 1.5: "float", "in": map[any]any{true: "yes"}},
				"nil":   map[any]any(nil),
				"types": map[any]any{1.0: "float64", 1: "int", "1": "string"},
				"same":  map[any]any{labelKey{2}: "second", labelKey{1}: "first"},
			})
			testutil.NoError(t, err)
			testutil.Equal(t, "bag", got, Bag{
				"y":     Bag{"k": 1, "2": "two", "a": Bag{"b": 3}, "1": Bag{"5": "float"}, "in": Bag{"true": "yes"}},
				"nil":   Bag(nil),
				"types": Bag{"1": "string"},
				"same":  Bag{"label": "second"},
			})
		}
	})

	t.Run("should fail, unchanged, on invalid keys", func(t *testing.T) {
		for _, tc := range []struct {
			src Bag
			key string
		}{
			{Bag{"": 1}, ""}, {Bag{".a": 1}, ".a"}, {Bag{"n": Bag{"": 1}}, "n."},
			{Bag{"n": Bag{"a..b": 1}}, "n.a..b"}, {Bag{"n.m": map[string]any{"a.": 1}}, "n.m.a."},
			{Bag{"y": map[any]any{emptyKey{}: 1}}, "y."},
		} {
			b := Bag{"keep": 1}
			_, err := b.Merge("", tc.src)
			testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path",
				map[string]any{"path": "", "key": tc.key})
			testutil.Equal(t, "bag", b, Bag{"keep": 1})
		}
	})

	t.Run("should fail, unchanged, on path conflicts and invalid paths", func(t *testing.T) {
		for _, tc := range []struct{ path, segment string }{
			{"a", "a"}, {"a.b", "a"}, {"a.b.c", "a"}, {"y", "y"}, {"y.k", "y"},
		} {
			b := Bag{"a": 1, "y": map[any]any{"k": 1}}
			_, err := b.Merge(tc.path, Bag{"n": 1})
			testutil.Error(t, err, container.ErrPathConflict, "unable to merge bag: path conflict",
				map[string]any{"path": tc.path, "segment": tc.segment})
			testutil.Equal(t, "bag", b, Bag{"a": 1, "y": map[any]any{"k": 1}})
		}
		for _, path := range []string{"a..b", ".a", "a.", "."} {
			_, err := Bag{}.Merge(path, Bag{"n": 1})
			testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path",
				map[string]any{"path": path})
		}
	})

	t.Run("should report errors in a fixed order", func(t *testing.T) {
		for _, tc := range []struct {
			path     string
			src      Bag
			sentinel error
			ctx      map[string]any
		}{
			{"a..b", Bag{"": 1}, container.ErrInvalidPath, map[string]any{"path": "a..b"}},
			{"leaf", Bag{"": 1}, container.ErrInvalidPath, map[string]any{"path": "leaf", "key": ""}},
			{"", Bag{"b..c": 1, "a..b": 2, "c.": 3}, container.ErrInvalidPath, map[string]any{"path": "", "key": "a..b"}},
		} {
			for range 20 {
				_, err := Bag{"leaf": 1}.Merge(tc.path, tc.src)
				testutil.Error(t, err, tc.sentinel, "unable to merge bag: "+tc.sentinel.Error(), tc.ctx)
			}
		}
	})

	t.Run("should copy src, sharing the values of other types", func(t *testing.T) {
		nested, inner, y := Bag{"k": "v"}, map[string]any{"k": "v"}, map[any]any{"k": "v"}
		anys, strs := []any{"v", Bag{"k": "v"}}, []string{"v"}
		ptr, ints, bags := &struct{}{}, []int{1}, []Bag{{"k": "v"}}
		src := Bag{"nested": nested, "inner": inner, "y": y, "anys": anys, "strs": strs, "p": ptr, "ints": ints, "bags": bags}
		got, err := Bag{}.Merge("", src)
		testutil.NoError(t, err)

		nested["k"], inner["k"], y["k"] = "changed", "changed", "changed"
		anys[0], anys[1].(Bag)["k"], strs[0] = "changed", "changed", "changed"
		src["added"] = 1

		testutil.Equal(t, "nested", got.(Bag)["nested"], Bag{"k": "v"})
		testutil.Equal(t, "inner", got.(Bag)["inner"], map[string]any{"k": "v"})
		testutil.Equal(t, "y", got.(Bag)["y"], Bag{"k": "v"})
		testutil.Equal(t, "anys", got.(Bag)["anys"], []any{"v", Bag{"k": "v"}})
		testutil.Equal(t, "strs", got.(Bag)["strs"], []string{"v"})
		if got.(Bag)["p"] != any(ptr) || !testutil.SamePointer(got.(Bag)["ints"], ints) || !testutil.SamePointer(got.(Bag)["bags"], bags) {
			t.Error("Merge() copied a value of a type it does not copy")
		}
		if _, added := got.(Bag)["added"]; added {
			t.Error("a key added to src after Merge reached the bag")
		}
	})

	t.Run("should be deterministic when src overlaps the destination", func(t *testing.T) {
		for range 50 {
			s := Bag{"a": Bag{"x": 1}, "b": 2}
			got, err := s.Merge("a", s)
			testutil.NoError(t, err)
			testutil.Equal(t, "bag", got, Bag{"a": Bag{"x": 1, "a": Bag{"x": 1}, "b": 2}, "b": 2})
		}
		b := Bag{"a": Bag{"x": 1}, "l": []any{1}}
		got, err := b.Merge("", b)
		testutil.NoError(t, err)
		testutil.Equal(t, "self merge", got, Bag{"a": Bag{"x": 1}, "l": []any{1}})
	})

	t.Run("should copy a bag referenced twice into two independent bags", func(t *testing.T) {
		shared := Bag{"k": 1}
		got, err := Bag{}.Merge("", Bag{"a": shared, "b": shared})
		testutil.NoError(t, err)
		if testutil.SamePointer(got.(Bag)["a"], got.(Bag)["b"]) || testutil.SamePointer(got.(Bag)["a"], shared) {
			t.Error("the copies are not independent bags")
		}
	})
}

func Test_Merge_EdgeCases(t *testing.T) {
	t.Run("should return the receiver, unchanged, along with every kind of error", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			path     string
			src      flam.IBag
			sentinel error
		}{
			{"invalid path", "a..b", Bag{"n": 1}, container.ErrInvalidPath},
			{"unsupported src", "", foreignBag{whole: 1}, container.ErrUnsupportedBag},
			{"invalid key", "", Bag{"a..b": 1}, container.ErrInvalidPath},
			{"path conflict", "leaf", Bag{"n": 1}, container.ErrPathConflict},
		} {
			t.Run(tc.name, func(t *testing.T) {
				b := Bag{"leaf": 1}
				got, err := b.Merge(tc.path, tc.src)
				if !errors.Is(err, tc.sentinel) {
					t.Fatalf("error = %v, want it to wrap %v", err, tc.sentinel)
				}
				if !testutil.SamePointer(got, b) {
					t.Error("Merge() did not return the receiver")
				}
				testutil.Equal(t, "bag", b, Bag{"leaf": 1})
			})
		}
	})

	t.Run("should report as conflicts every value on the path that is not a bag", func(t *testing.T) {
		for _, tc := range []struct{ name, path, segment string }{
			{"a []any", "list", "list"},
			{"a map type based on map[string]any", "named", "named"},
			{"a map[string]string", "strMap.k", "strMap"},
			{"a leaf of a map[string]any", "m.k", "m.k"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				b := Bag{"list": []any{}, "named": namedMap{}, "strMap": map[string]string{}, "m": map[string]any{"k": 1}}
				_, err := b.Merge(tc.path, Bag{"n": 1})
				testutil.Error(t, err, container.ErrPathConflict, "unable to merge bag: path conflict",
					map[string]any{"path": tc.path, "segment": tc.segment})
			})
		}
	})

	t.Run("should keep the destination Bag type when merging a src map[string]any into it", func(t *testing.T) {
		got, err := Bag{"a": Bag{"x": 1}}.Merge("", Bag{"a": map[string]any{"y": 2}})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": Bag{"x": 1, "y": 2}})
	})

	t.Run("should add nil and empty src bags when the key is missing, keeping their type", func(t *testing.T) {
		src := Bag{"nilBag": Bag(nil), "nilMap": map[string]any(nil), "emptyBag": Bag{}, "emptyMap": map[string]any{}}
		got, err := Bag{}.Merge("", src)
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, src)
		if testutil.SamePointer(got.(Bag)["emptyBag"], src["emptyBag"]) {
			t.Error("the empty src bag was added without copying")
		}
	})

	t.Run("should replace a destination value that is not a bag with a nil src bag", func(t *testing.T) {
		got, err := Bag{"leaf": 1}.Merge("", Bag{"leaf": Bag(nil)})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"leaf": Bag(nil)})
	})

	t.Run("should keep an existing bag at the path on a nil or empty src", func(t *testing.T) {
		for _, src := range []flam.IBag{nil, Bag(nil), Bag{}} {
			inner := Bag{"k": 1}
			b := Bag{"a": inner}
			_, err := b.Merge("a", src)
			testutil.NoError(t, err)
			if !testutil.SamePointer(b["a"], inner) {
				t.Error("the existing bag at the path was replaced")
			}
			testutil.Equal(t, "bag", b, Bag{"a": Bag{"k": 1}})
		}
	})

	t.Run("should create the missing intermediate bags as Bag and merge into a map[string]any in place", func(t *testing.T) {
		m := map[string]any{"k": 1}
		b := Bag{"m": m}
		_, err := b.Merge("m", Bag{"n": 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "map", m, map[string]any{"k": 1, "n": 2})

		_, err = b.Merge("m.x.y", Bag{"z": 3})
		testutil.NoError(t, err)
		testutil.Equal(t, "map", m["x"], Bag{"y": Bag{"z": 3}})
	})

	t.Run("should expand dotted keys of a src map[string]any, keeping its type", func(t *testing.T) {
		got, err := Bag{}.Merge("", Bag{"m": map[string]any{"x.y": 1}})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"m": map[string]any{"x": Bag{"y": 1}}})
	})

	t.Run("should not let later changes to the bag reach src", func(t *testing.T) {
		src := Bag{"a": Bag{"k": 1}, "l": []any{1}, "m": map[string]any{"k": 1}}
		got, err := Bag{}.Merge("", src)
		testutil.NoError(t, err)
		got.(Bag)["a"].(Bag)["k"] = 2
		got.(Bag)["l"].([]any)[0] = 2
		got.(Bag)["m"].(map[string]any)["k"] = 2
		testutil.Equal(t, "src", src, Bag{"a": Bag{"k": 1}, "l": []any{1}, "m": map[string]any{"k": 1}})
	})

	t.Run("should report the full key path of an invalid key nested in converted bags", func(t *testing.T) {
		for _, tc := range []struct {
			src Bag
			key string
		}{
			{Bag{"m": map[string]any{"y": map[any]any{"a..b": 1}}}, "m.y.a..b"},
			{Bag{"y": map[any]any{"z": map[any]any{"": 1}}}, "y.z."},
			{Bag{".": 1}, "."},
			{Bag{"a.b": Bag{"c.": 1}}, "a.b.c."},
		} {
			_, err := Bag{}.Merge("", tc.src)
			testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path",
				map[string]any{"path": "", "key": tc.key})
		}
	})

	t.Run("should add no values from a foreign bag whose Get returns a nil map[string]any", func(t *testing.T) {
		got, err := Bag{"keep": 1}.Merge("p", foreignBag{whole: map[string]any(nil)})
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"keep": 1, "p": Bag{}})
	})

	t.Run("should reject a foreign bag whose Get returns a named map type", func(t *testing.T) {
		_, err := Bag{}.Merge("", foreignBag{whole: namedMap{"k": 1}})
		testutil.Error(t, err, container.ErrUnsupportedBag, "unable to merge bag: unsupported bag",
			map[string]any{"path": ""})
	})

	t.Run("should copy the content of a foreign bag", func(t *testing.T) {
		whole := map[string]any{"a": map[string]any{"k": 1}}
		got, err := Bag{}.Merge("", foreignBag{whole: whole})
		testutil.NoError(t, err)
		whole["a"].(map[string]any)["k"] = 2
		testutil.Equal(t, "bag", got, Bag{"a": map[string]any{"k": 1}})
	})

	t.Run("should report a src key error before a path conflict", func(t *testing.T) {
		_, err := Bag{"leaf": 1}.Merge("leaf.x", Bag{"a.": 1})
		testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path",
			map[string]any{"path": "leaf.x", "key": "a."})
	})

	t.Run("should report an unsupported src before its keys and a path conflict", func(t *testing.T) {
		_, err := Bag{"leaf": 1}.Merge("leaf", foreignBag{whole: []any{}})
		testutil.Error(t, err, container.ErrUnsupportedBag, "unable to merge bag: unsupported bag",
			map[string]any{"path": "leaf"})
	})
}

// ---------------------------------------------------------------------------
// Clone
// ---------------------------------------------------------------------------

func Test_Clone(t *testing.T) {
	newOriginal := func() Bag {
		return Bag{
			"bag": Bag{"k": "v"}, "map": map[string]any{"k": "v"}, "slice": []any{"v", Bag{"k": "v"}, nil},
			"strs": []string{"v"}, "leaf": 1,
		}
	}

	t.Run("should copy every supported container at any depth, both ways", func(t *testing.T) {
		b := newOriginal()
		c := b.Clone()
		testutil.Equal(t, "clone", c, b)

		c.(Bag)["bag"].(Bag)["k"] = "x"
		c.(Bag)["map"].(map[string]any)["k"] = "x"
		c.(Bag)["slice"].([]any)[0] = "x"
		c.(Bag)["slice"].([]any)[1].(Bag)["k"] = "x"
		c.(Bag)["strs"].([]string)[0] = "x"
		c.(Bag)["added"] = 1
		testutil.Equal(t, "original", b, newOriginal())

		c = b.Clone()
		b["bag"].(Bag)["k"] = "x"
		b["added"] = 1
		testutil.Equal(t, "clone", c, newOriginal())
	})

	t.Run("should share values of other types", func(t *testing.T) {
		ptr, ints, strMap := &struct{}{}, []int{1}, map[string]string{"k": "v"}
		y, bags, maps := map[any]any{"k": "v"}, []Bag{{"k": "v"}}, []map[string]any{{"k": "v"}}
		c := Bag{"p": ptr, "ints": ints, "strMap": strMap, "y": y, "bags": bags, "maps": maps}.Clone()
		if c.(Bag)["p"] != any(ptr) || !testutil.SamePointer(c.(Bag)["ints"], ints) || !testutil.SamePointer(c.(Bag)["strMap"], strMap) ||
			!testutil.SamePointer(c.(Bag)["y"], y) || !testutil.SamePointer(c.(Bag)["bags"], bags) || !testutil.SamePointer(c.(Bag)["maps"], maps) {
			t.Error("Clone() copied a value of a type it does not copy")
		}
	})

	t.Run("should keep nil and empty containers", func(t *testing.T) {
		if Bag(nil).Clone() != nil {
			t.Error("Clone() of a nil bag is not nil")
		}
		testutil.Equal(t, "empty", Bag{}.Clone(), Bag{})
		b := Bag{
			"nilBag": Bag(nil), "nilMap": map[string]any(nil), "nilSlice": []any(nil), "nilStrs": []string(nil),
			"nil": nil, "emptySlice": []any{}, "emptyStrs": []string{},
		}
		testutil.Equal(t, "clone", b.Clone(), b)
	})

	t.Run("should copy a bag referenced twice into two independent bags", func(t *testing.T) {
		shared := Bag{"k": 1}
		c := Bag{"a": shared, "b": shared}.Clone()
		if testutil.SamePointer(c.(Bag)["a"], c.(Bag)["b"]) || testutil.SamePointer(c.(Bag)["a"], shared) {
			t.Error("the copies are not independent bags")
		}
	})

	t.Run("should return a new bag, even for an empty one", func(t *testing.T) {
		for _, b := range []Bag{{}, {"k": 1}} {
			if testutil.SamePointer(b.Clone(), b) {
				t.Error("Clone() returned the receiver")
			}
		}
	})

	t.Run("should share named map and slice types", func(t *testing.T) {
		named, anys, strs := namedMap{"k": "v"}, namedAnySlice{"v"}, namedStringSlice{"v"}
		c := Bag{"named": named, "anys": anys, "strs": strs}.Clone().(Bag)
		if !testutil.SamePointer(c["named"], named) || !testutil.SamePointer(c["anys"], anys) ||
			!testutil.SamePointer(c["strs"], strs) {
			t.Error("Clone() copied a named container type")
		}
	})

	t.Run("should copy containers nested in slices and maps at any depth", func(t *testing.T) {
		deepest := []string{"v"}
		b := Bag{"l": []any{map[string]any{"b": Bag{"s": deepest}}}}
		c := b.Clone()
		deepest[0] = "changed"
		testutil.Equal(t, "clone", c, Bag{"l": []any{map[string]any{"b": Bag{"s": []string{"v"}}}}})
	})

	t.Run("should keep the type and nil-ness of nested nil containers", func(t *testing.T) {
		b := Bag{"l": []any{Bag(nil), map[string]any(nil), []any(nil), []string(nil), nil}, "m": map[string]any{"n": Bag(nil)}}
		testutil.Equal(t, "clone", b.Clone(), b)
	})
}

// ---------------------------------------------------------------------------
// All
// ---------------------------------------------------------------------------

// entry is a key and value yielded by All.
type entry struct {
	key   string
	value any
}

// collect returns the entries yielded by All, in order.
func collect(b flam.IBag) []entry {
	var entries []entry
	for key, value := range b.All() {
		entries = append(entries, entry{key, value})
	}
	return entries
}

func Test_All(t *testing.T) {
	t.Run("should yield the top level entries in key order", func(t *testing.T) {
		b := Bag{"c": 3, "a": 1, "b": 2, "B": 0, "": "empty", "a.b": "dotted"}
		testutil.Equal(t, "entries", collect(b), []entry{
			{"", "empty"}, {"B", 0}, {"a", 1}, {"a.b", "dotted"}, {"b", 2}, {"c", 3},
		})
	})

	t.Run("should yield nothing for an empty or nil bag", func(t *testing.T) {
		testutil.Equal(t, "empty", collect(Bag{}), []entry(nil))
		testutil.Equal(t, "nil", collect(Bag(nil)), []entry(nil))
	})

	t.Run("should yield nested bags and nil values as single values", func(t *testing.T) {
		nested := Bag{"k": "v"}
		m := map[string]any{"k": "v"}
		b := Bag{"bag": nested, "map": m, "nil": nil}
		testutil.Equal(t, "entries", collect(b), []entry{
			{"bag", nested}, {"map", m}, {"nil", nil},
		})
	})

	t.Run("should yield the values as stored, without copying", func(t *testing.T) {
		b := Bag{"bag": Bag{"k": "v"}, "slice": []any{"v"}}
		for key, value := range b.All() {
			switch v := value.(type) {
			case Bag:
				v["k"] = "changed"
			case []any:
				v[0] = "changed"
			default:
				t.Fatalf("unexpected value at %q: %#v", key, value)
			}
		}
		testutil.Equal(t, "bag.k", b.String("bag.k"), "changed")
		testutil.Equal(t, "slice", b.Slice("slice"), []any{"changed"})
	})

	t.Run("should stop when the loop breaks", func(t *testing.T) {
		var keys []string
		for key := range (Bag{"a": 1, "b": 2, "c": 3}).All() {
			keys = append(keys, key)
			if key == "b" {
				break
			}
		}
		testutil.Equal(t, "keys", keys, []string{"a", "b"})
	})

	t.Run("should be iterable more than once", func(t *testing.T) {
		seq := Bag{"a": 1, "b": 2}.All()
		var first, second []string
		for key := range seq {
			first = append(first, key)
		}
		for key := range seq {
			second = append(second, key)
		}
		testutil.Equal(t, "first", first, []string{"a", "b"})
		testutil.Equal(t, "second", second, []string{"a", "b"})
	})

	t.Run("should let the loop modify the bag", func(t *testing.T) {
		b := Bag{"a": 1, "b": 2, "c": 3}
		var entries []entry
		for key, value := range b.All() {
			entries = append(entries, entry{key, value})
			if key == "a" {
				testutil.NoError(t, b.Delete("b"))
				testutil.NoError(t, b.Set("c", 30))
				testutil.NoError(t, b.Set("d", 4))
			}
		}
		testutil.Equal(t, "entries", entries, []entry{{"a", 1}, {"c", 30}})
		testutil.Equal(t, "bag", b, Bag{"a": 1, "c": 30, "d": 4})
	})

	t.Run("should be reachable through flam.IBag and nested bags", func(t *testing.T) {
		var b flam.IBag = Bag{"db": Bag{"port": 5432, "host": "localhost"}}
		testutil.Equal(t, "entries", collect(b.Bag("db")), []entry{
			{"host", "localhost"}, {"port", 5432},
		})
	})
}

// ---------------------------------------------------------------------------
// allocations and concurrency
// ---------------------------------------------------------------------------

func Test_LookupAllocations(t *testing.T) {
	b := Bag{"a": Bag{"b": map[string]any{"c": 1}}}
	for _, path := range []string{"", "a", "a.b.c", "a.missing", "a..b"} {
		if allocs := testing.AllocsPerRun(100, func() { _ = b.Has(path) }); allocs != 0 {
			t.Errorf("Has(%q) allocated %v times per call, want 0", path, allocs)
		}
	}
}

func Test_ConcurrentReads(t *testing.T) {
	b := Bag{"a": Bag{"b": 1}, "s": []any{"x"}, "m": map[string]any{"k": "v"}}
	before := b.Clone()
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if b.Int("a.b") != 1 || b.String("m.k") != "v" || !b.Has("s") {
				t.Error("a concurrent read returned a wrong value")
			}
			_, _ = b.Get("m"), b.Slice("s")
			_ = b.Clone()
			if _, err := (Bag{}).Merge("", b); err != nil {
				t.Errorf("concurrent Merge() from the bag failed: %v", err)
			}
		})
	}
	wg.Wait()
	testutil.Equal(t, "bag", b, before)
}

// ---------------------------------------------------------------------------
// flam.IBag results and arguments
// ---------------------------------------------------------------------------

// compile time check that Bag implements flam.IBag
var _ flam.IBag = Bag(nil)

// foreignBag is a flam.IBag implementation other than Bag, whose Get("")
// returns whole; its other methods are not implemented.
type foreignBag struct {
	flam.IBag
	whole any
}

func (f foreignBag) Get(string, ...any) any { return f.whole }

func Test_IBagResults(t *testing.T) {
	t.Run("should return a nil flam.IBag for a nil bag", func(t *testing.T) {
		b := Bag{"nilBag": Bag(nil), "nilMap": map[string]any(nil)}
		for _, path := range []string{"nilBag", "nilMap"} {
			if got := b.Bag(path); got != nil {
				t.Errorf("Bag(%q) = %#v, want a nil interface", path, got)
			}
		}
		if got := Bag(nil).Clone(); got != nil {
			t.Errorf("Clone() of a nil bag = %#v, want a nil interface", got)
		}
	})

	t.Run("should return Bag values", func(t *testing.T) {
		b := Bag{"m": map[string]any{"k": 1}}
		for name, got := range map[string]flam.IBag{"Bag": b.Bag("m"), "Clone": b.Clone()} {
			if _, ok := got.(Bag); !ok {
				t.Errorf("%s() holds a %T, want a Bag", name, got)
			}
		}
		got, err := b.Merge("", nil)
		testutil.NoError(t, err)
		if _, ok := got.(Bag); !ok {
			t.Errorf("Merge() holds a %T, want a Bag", got)
		}
	})

	t.Run("should return a default of any flam.IBag implementation as is", func(t *testing.T) {
		def := foreignBag{whole: "marker"} // comparable, unlike a struct holding a map
		if got := (Bag{}).Bag("missing", def); got != flam.IBag(def) {
			t.Errorf("Bag(missing, def) = %#v, want the default", got)
		}
	})

	t.Run("should merge a nil flam.IBag as an empty src", func(t *testing.T) {
		got, err := Bag{"a": 1}.Merge("p", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "bag", got, Bag{"a": 1, "p": Bag{}})
	})

	t.Run("should merge another flam.IBag implementation through Get", func(t *testing.T) {
		for name, whole := range map[string]any{
			"Bag":            Bag{"x.y": 1},
			"map[string]any": map[string]any{"x.y": 1},
		} {
			got, err := Bag{"keep": 1}.Merge("", foreignBag{whole: whole})
			testutil.NoError(t, err)
			testutil.Equal(t, name, got, Bag{"keep": 1, "x": Bag{"y": 1}})
		}
		got, err := Bag{"keep": 1}.Merge("", foreignBag{whole: Bag(nil)})
		testutil.NoError(t, err)
		testutil.Equal(t, "nil Bag", got, Bag{"keep": 1})
	})

	t.Run("should fail, unchanged, on an unsupported flam.IBag implementation", func(t *testing.T) {
		for _, whole := range []any{nil, 1, "bag", map[any]any{"k": 1}, []any{Bag{}}} {
			b := Bag{"keep": 1}
			_, err := b.Merge("p", foreignBag{whole: whole})
			testutil.Error(t, err, container.ErrUnsupportedBag, "unable to merge bag: unsupported bag",
				map[string]any{"path": "p"})
			testutil.Equal(t, "bag", b, Bag{"keep": 1})
		}
	})

	t.Run("should check the path before an unsupported src", func(t *testing.T) {
		_, err := Bag{}.Merge("a..b", foreignBag{whole: 1})
		testutil.Error(t, err, container.ErrInvalidPath, "unable to merge bag: invalid path", map[string]any{"path": "a..b"})
	})
}

func Test_IBagContract(t *testing.T) {
	var b flam.IBag = Bag{"a": Bag{"b": 1}}

	t.Run("should expose every documented method through flam.IBag", func(t *testing.T) {
		testutil.NoError(t, b.Set("v.bool", true))
		testutil.NoError(t, b.Set("v.int", 1))
		testutil.NoError(t, b.Set("v.int8", int8(2)))
		testutil.NoError(t, b.Set("v.int16", int16(3)))
		testutil.NoError(t, b.Set("v.int32", int32(4)))
		testutil.NoError(t, b.Set("v.int64", int64(5)))
		testutil.NoError(t, b.Set("v.uint", uint(6)))
		testutil.NoError(t, b.Set("v.uint8", uint8(7)))
		testutil.NoError(t, b.Set("v.uint16", uint16(8)))
		testutil.NoError(t, b.Set("v.uint32", uint32(9)))
		testutil.NoError(t, b.Set("v.uint64", uint64(10)))
		testutil.NoError(t, b.Set("v.float32", float32(1.5)))
		testutil.NoError(t, b.Set("v.float64", 2.5))
		testutil.NoError(t, b.Set("v.rune", 'r'))
		testutil.NoError(t, b.Set("v.string", "s"))
		testutil.NoError(t, b.Set("v.slice", []any{1}))
		testutil.NoError(t, b.Set("v.strings", []string{"s"}))
		testutil.NoError(t, b.Set("v.duration", time.Second))

		if !b.Has("a.b") || b.Get("a.b") != 1 || !b.Bool("v.bool") || b.Int("v.int") != 1 || b.Int8("v.int8") != 2 ||
			b.Int16("v.int16") != 3 || b.Int32("v.int32") != 4 || b.Int64("v.int64") != 5 || b.Uint("v.uint") != 6 ||
			b.Uint8("v.uint8") != 7 || b.Uint16("v.uint16") != 8 || b.Uint32("v.uint32") != 9 ||
			b.Uint64("v.uint64") != 10 || b.Float32("v.float32") != 1.5 || b.Float64("v.float64") != 2.5 ||
			b.Rune("v.rune") != 'r' || b.String("v.string") != "s" || b.Duration("v.duration") != time.Second {
			t.Error("a getter returned a wrong value through flam.IBag")
		}
		testutil.Equal(t, "Slice", b.Slice("v.slice"), []any{1})
		testutil.Equal(t, "StringSlice", b.StringSlice("v.strings"), []string{"s"})
		testutil.Equal(t, "Bag", b.Bag("a"), flam.IBag(Bag{"b": 1}))

		clone := b.Clone()
		testutil.Equal(t, "Clone", clone, b)
		testutil.NoError(t, b.Delete("v"))
		if b.Has("v") || !clone.Has("v") {
			t.Error("Delete() through flam.IBag did not delete, or reached the clone")
		}

		merged, err := b.Merge("a", Bag{"c": 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "Merge", merged, flam.IBag(Bag{"a": Bag{"b": 1, "c": 2}}))
	})

	t.Run("should return the defaults of every getter through flam.IBag", func(t *testing.T) {
		var e flam.IBag = Bag{}
		def := Bag{"d": 1}
		if e.Get("x", 1) != 1 || !e.Bool("x", true) || e.Int("x", 1) != 1 || e.Int8("x", 1) != 1 ||
			e.Int16("x", 1) != 1 || e.Int32("x", 1) != 1 || e.Int64("x", 1) != 1 || e.Uint("x", 1) != 1 ||
			e.Uint8("x", 1) != 1 || e.Uint16("x", 1) != 1 || e.Uint32("x", 1) != 1 || e.Uint64("x", 1) != 1 ||
			e.Float32("x", 1) != 1 || e.Float64("x", 1) != 1 || e.Rune("x", 'd') != 'd' || e.String("x", "d") != "d" ||
			e.Duration("x", time.Second) != time.Second || !testutil.SamePointer(e.Bag("x", def), def) {
			t.Error("a getter did not return its default through flam.IBag")
		}
		testutil.Equal(t, "Slice", e.Slice("x", []any{1}), []any{1})
		testutil.Equal(t, "StringSlice", e.StringSlice("x", []string{"s"}), []string{"s"})
	})

	t.Run("should let a nil Bag in a flam.IBag be merged into, using the result", func(t *testing.T) {
		var cfg flam.IBag = Bag(nil)
		cfg, err := cfg.Merge("", Bag{"a": 1})
		testutil.NoError(t, err)
		cfg, err = cfg.Merge("db", Bag{"host": "h"})
		testutil.NoError(t, err)
		testutil.Equal(t, "cfg", cfg, flam.IBag(Bag{"a": 1, "db": Bag{"host": "h"}}))
	})
}

// ---------------------------------------------------------------------------
// errors
// ---------------------------------------------------------------------------

func Test_Errors(t *testing.T) {
	failures := map[string]func() error{
		"Set invalid path": func() error { return Bag{}.Set("", 1) },
		"Set nil bag":      func() error { return Bag(nil).Set("a", 1) },
		"Set conflict":     func() error { return Bag{"a": 1}.Set("a.b", 1) },
		"Delete invalid":   func() error { return Bag{}.Delete("") },
		"Merge invalid": func() error {
			_, err := Bag{}.Merge(".", nil)
			return err
		},
		"Merge unsupported": func() error {
			_, err := Bag{}.Merge("", foreignBag{whole: 1})
			return err
		},
	}
	sentinels := map[string]error{
		"Set invalid path": container.ErrInvalidPath, "Set nil bag": container.ErrNil,
		"Set conflict": container.ErrPathConflict, "Delete invalid": container.ErrInvalidPath,
		"Merge invalid": container.ErrInvalidPath, "Merge unsupported": container.ErrUnsupportedBag,
	}

	for name, fail := range failures {
		t.Run(name, func(t *testing.T) {
			t.Run("should return a *flam.Error unwrapping directly to the sentinel", func(t *testing.T) {
				err := fail()
				e, ok := err.(*flam.Error)
				if !ok {
					t.Fatalf("error = %T, want a *flam.Error", err)
				}
				if e.Unwrap() != sentinels[name] {
					t.Errorf("Unwrap() = %v, want %v", e.Unwrap(), sentinels[name])
				}
			})

			t.Run("should return a new error on every call", func(t *testing.T) {
				a, b := fail(), fail()
				if a == b || errors.Is(a, b) {
					t.Error("two failures returned the same error")
				}
			})
		})
	}
}

func Test_Merge_Slices(t *testing.T) {
	t.Run("should not traverse the bags held by a slice of the source", func(t *testing.T) {
		inner := map[any]any{"k": 1}
		src := Bag{"list": []any{Bag{"a.b": 1}, map[string]any{"c.d": 2}, inner}}

		got, err := Bag{}.Merge("", src)
		testutil.NoError(t, err)
		testutil.Equal(t, "list", got.Get("list"), []any{Bag{"a.b": 1}, map[string]any{"c.d": 2}, inner})
		if _, isAnyMap := got.Get("list").([]any)[2].(map[any]any); !isAnyMap {
			t.Fatal("the map[any]any in the slice was converted")
		}
	})
}

func Test_ValidPath(t *testing.T) {
	for path, want := range map[string]bool{
		"": true, "a": true, "a.b": true, "a.b.c": true,
		".": false, ".a": false, "a.": false, "a..b": false,
	} {
		testutil.Equal(t, path, container.ValidPath(path), want)
	}
}
