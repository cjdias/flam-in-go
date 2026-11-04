package base

import (
	"fmt"
	"maps"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// Field returns the value of the field of the dialect configuration, or
// else the one stored in the configuration at the default path, or else
// the built-in default, as the field is optional. It delegates to the
// shared settings.Field.
func Field(
	config flam.IConfig,
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(config, bag, field, defaultPath, builtIn)
}

// NonEmpty returns the string held by the value, or false when it is not
// a non-empty string.
func NonEmpty(
	val any,
) (string, bool) {
	s, ok := val.(string)
	return s, ok && s != ""
}

// Text returns the string held by the value, which may be empty, or false
// when it is not a string.
func Text(
	val any,
) (string, bool) {
	s, ok := val.(string)
	return s, ok
}

// Port returns the port held by the value, a whole number of any integer
// or float kind, or a string holding one, between 1 and 65535, or false
// for any other value.
func Port(
	val any,
) (int, bool) {
	if s, ok := val.(string); ok {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		return n, err == nil && n >= 1 && n <= math.MaxUint16
	}

	var n float64
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n = float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		n = rv.Float()
	default:
		return 0, false
	}
	if n != math.Trunc(n) || n < 1 || n > math.MaxUint16 {
		return 0, false
	}
	return int(n), true
}

// Params returns the parameters held by the value, a bag, a
// map[string]any or a map[string]string, its values formatted with fmt %v,
// or nil, which holds none, or false for any other value or a nested
// value.
func Params(
	val any,
) (map[string]string, bool) {
	var entries map[string]any
	switch v := val.(type) {
	case nil:
		return map[string]string{}, true
	case flam.IBag:
		entries = map[string]any{}
		for key, item := range v.All() {
			entries[key] = item
		}
	case map[string]any:
		entries = v
	case map[string]string:
		return maps.Clone(v), true
	default:
		return nil, false
	}

	params := make(map[string]string, len(entries))
	for key, item := range entries {
		switch item.(type) {
		case nil, flam.IBag, map[string]any, []any, []string:
			return nil, false
		}
		params[key] = fmt.Sprintf("%v", item)
	}
	return params, true
}
