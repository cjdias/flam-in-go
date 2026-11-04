// Package value holds the helpers on dynamically typed values shared by the
// flam packages.
package value

import (
	"math"
	"reflect"
)

// IsNil reports whether v is nil, including the case of a nil pointer,
// unsafe pointer, map, slice, function, channel or interface held by a
// non-nil interface, such as a nil *T given as an error or an any.
func IsNil(
	v any,
) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return rv.IsNil()
	}
	return false
}

// Equal reports whether a and b are equal as [reflect.DeepEqual] tells, but
// for the NaN floats, equal to each other, so that a value holding one
// equals itself, the nil and empty maps and slices, equal to each other,
// and the maps, or the slices, of distinct types sharing their key and
// element types, such as a named map type and map[string]any, compared by
// content. The maps and slices are compared element by element, at any
// depth, through the interfaces they hold, a map key being looked up by
// equality when the lookup misses it, as for a NaN key; the other values,
// structs included, are compared by reflect.DeepEqual. The values must not
// contain themselves.
func Equal(
	a any,
	b any,
) bool {
	return valueEqual(reflect.ValueOf(a), reflect.ValueOf(b))
}

// valueEqual reports whether the values are equal, as described by Equal.
func valueEqual(
	a reflect.Value,
	b reflect.Value,
) bool {
	if !a.IsValid() || !b.IsValid() {
		return a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() && !valueSameShape(a.Type(), b.Type()) {
		return false
	}

	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		x, y := a.Float(), b.Float()
		return x == y || math.IsNaN(x) && math.IsNaN(y)
	case reflect.Interface:
		return valueEqual(a.Elem(), b.Elem())
	case reflect.Map:
		if a.Len() != b.Len() {
			return false
		}
		for entry := a.MapRange(); entry.Next(); {
			other := valueMapIndex(b, entry.Key())
			if !other.IsValid() || !valueEqual(entry.Value(), other) {
				return false
			}
		}
		return true
	case reflect.Slice:
		if a.Len() != b.Len() {
			return false
		}
		for i := range a.Len() {
			if !valueEqual(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(a.Interface(), b.Interface())
}

// valueSameShape reports whether the types are maps, or slices, of the same
// key and element types, compared by content.
func valueSameShape(
	a reflect.Type,
	b reflect.Type,
) bool {
	switch {
	case a.Kind() == reflect.Map && b.Kind() == reflect.Map:
		return a.Key() == b.Key() && a.Elem() == b.Elem()
	case a.Kind() == reflect.Slice && b.Kind() == reflect.Slice:
		return a.Elem() == b.Elem()
	}
	return false
}

// valueMapIndex returns the value of the map m at the key, looked up by
// equality when the map lookup misses it, as for a NaN key, which no
// lookup finds, or the invalid value when there is none.
func valueMapIndex(
	m reflect.Value,
	key reflect.Value,
) reflect.Value {
	if v := m.MapIndex(key); v.IsValid() {
		return v
	}
	for entry := m.MapRange(); entry.Next(); {
		if valueEqual(entry.Key(), key) {
			return entry.Value()
		}
	}
	return reflect.Value{}
}
