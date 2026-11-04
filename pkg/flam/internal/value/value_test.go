package value

import (
	"errors"
	"math"
	"testing"
	"unsafe"
)

// The test lives next to the package, unlike the other tests of flam,
// since an internal package cannot be imported from the tests directory.

type ptrError struct{}

func (*ptrError) Error() string { return "" }

// named types based on the nillable kinds
type (
	namedMap   map[string]any
	namedSlice []any
	namedFunc  func()
	namedChan  chan int
)

func Test_IsNil(t *testing.T) {
	var (
		nilPtr        *int
		nilMap        map[string]any
		nilSlice      []any
		nilFunc       func()
		nilChan       chan int
		nilErr        *ptrError
		nilIface      error
		nilNamedMap   namedMap
		nilNamedSlice namedSlice
		nilNamedFunc  namedFunc
		nilNamedChan  namedChan
	)

	t.Run("should report nil for the untyped nil", func(t *testing.T) {
		if !IsNil(nil) {
			t.Error("IsNil(nil) = false, want true")
		}
	})

	t.Run("should report nil for a nil interface stored in an any", func(t *testing.T) {
		if !IsNil(nilIface) {
			t.Error("IsNil(nil error) = false, want true")
		}
	})

	t.Run("should report nil for a nil value of every nillable kind held by an interface", func(t *testing.T) {
		for name, v := range map[string]any{
			"nil pointer":                 nilPtr,
			"nil map":                     nilMap,
			"nil slice":                   nilSlice,
			"nil func":                    nilFunc,
			"nil chan":                    nilChan,
			"nil pointer as an error":     error(nilErr),
			"nil map of a named type":     nilNamedMap,
			"nil slice of a named type":   nilNamedSlice,
			"nil func of a named type":    nilNamedFunc,
			"nil chan of a named type":    nilNamedChan,
			"nil pointer to a pointer":    (**int)(nil),
			"nil pointer to an interface": (*error)(nil),
			"nil unsafe pointer":          unsafe.Pointer(nil),
		} {
			t.Run(name, func(t *testing.T) {
				if !IsNil(v) {
					t.Errorf("IsNil(%#v) = false, want true", v)
				}
			})
		}
	})

	t.Run("should report non-nil for non-nil values of the nillable kinds", func(t *testing.T) {
		for name, v := range map[string]any{
			"pointer":                     new(int),
			"unsafe pointer":              unsafe.Pointer(new(int)),
			"empty map":                   map[string]any{},
			"empty slice":                 []any{},
			"zero length made slice":      make([]any, 0),
			"func":                        func() {},
			"chan":                        make(chan int),
			"error":                       errors.New("e"),
			"pointer to a nil pointer":    &nilPtr,
			"pointer to a nil interface":  &nilIface,
			"empty map of a named type":   namedMap{},
			"slice holding a nil element": []any{nil},
		} {
			t.Run(name, func(t *testing.T) {
				if IsNil(v) {
					t.Errorf("IsNil(%#v) = true, want false", v)
				}
			})
		}
	})

	t.Run("should report non-nil for values of kinds that cannot be nil", func(t *testing.T) {
		for name, v := range map[string]any{
			"zero int":          0,
			"zero float":        0.0,
			"false":             false,
			"empty string":      "",
			"zero struct":       struct{}{},
			"struct with a nil": struct{ p *int }{},
			"zero array":        [2]*int{},
			"empty array":       [0]int{},
			"zero rune":         rune(0),
		} {
			t.Run(name, func(t *testing.T) {
				if IsNil(v) {
					t.Errorf("IsNil(%#v) = true, want false", v)
				}
			})
		}
	})
}

func Test_Equal(t *testing.T) {
	nan := math.NaN()
	type pair struct{ A, B any }
	type point struct{ X, Y int }

	t.Run("should report equal values", func(t *testing.T) {
		for name, p := range map[string]pair{
			"nils":                     {nil, nil},
			"ints":                     {1, 1},
			"strings":                  {"a", "a"},
			"floats":                   {0.5, 0.5},
			"zeros of both signs":      {0.0, math.Copysign(0, -1)},
			"NaNs":                     {nan, nan},
			"float32 NaNs":             {float32(nan), float32(nan)},
			"maps":                     {map[string]any{"a": 1}, map[string]any{"a": 1}},
			"named maps":               {namedMap{"a": 1}, namedMap{"a": 1}},
			"nil and empty maps":       {map[string]any(nil), map[string]any{}},
			"nested NaNs":              {map[string]any{"a": []any{nan}}, map[string]any{"a": []any{nan}}},
			"slices":                   {[]any{1, "a"}, []any{1, "a"}},
			"nil and empty slices":     {[]any(nil), []any{}},
			"string slices":            {[]string{"a"}, []string{"a"}},
			"nil interfaces in a list": {[]any{nil}, []any{nil}},
			"structs":                  {point{1, 2}, point{1, 2}},
			"pointers":                 {&point{1, 2}, &point{1, 2}},
			"maps of distinct types":   {map[string]any{"a": 1}, namedMap{"a": 1}},
			"nil and empty named maps": {namedMap(nil), map[string]any{}},
			"slices of distinct types": {[]any{1}, namedSlice{1}},
			"NaN keys":                 {map[float64]int{nan: 1}, map[float64]int{nan: 1}},
		} {
			t.Run(name, func(t *testing.T) {
				if !Equal(p.A, p.B) || !Equal(p.B, p.A) {
					t.Errorf("Equal(%#v, %#v) = false, want true", p.A, p.B)
				}
			})
		}
	})

	t.Run("should report different values", func(t *testing.T) {
		for name, p := range map[string]pair{
			"nil and a value":            {nil, 0},
			"ints":                       {1, 2},
			"types":                      {1, int64(1)},
			"an int and a float":         {1, 1.0},
			"floats":                     {0.5, 1.5},
			"a NaN and a float":          {nan, 0.5},
			"map sizes":                  {map[string]any{"a": 1}, map[string]any{"a": 1, "b": 2}},
			"map keys":                   {map[string]any{"a": 1}, map[string]any{"b": 1}},
			"map values":                 {map[string]any{"a": 1}, map[string]any{"a": 2}},
			"map key types":              {map[string]any{}, map[int]any{}},
			"map element types":          {map[string]any{}, map[string]int{}},
			"slice element types":        {[]any{}, []int{}},
			"NaN keys and values":        {map[float64]int{nan: 1}, map[float64]int{nan: 2}},
			"slice sizes":                {[]any{1}, []any{1, 2}},
			"slice items":                {[]any{1}, []any{2}},
			"a nil and a value in lists": {[]any{nil}, []any{0}},
			"structs":                    {point{1, 2}, point{2, 1}},
		} {
			t.Run(name, func(t *testing.T) {
				if Equal(p.A, p.B) || Equal(p.B, p.A) {
					t.Errorf("Equal(%#v, %#v) = true, want false", p.A, p.B)
				}
			})
		}
	})
}
