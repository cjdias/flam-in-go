package field

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// generator is the [flam.ICacheKeyGenerator] keying the objects by the
// value of a struct field.
type generator struct {
	id     string
	field  string
	prefix string
	typed  bool
}

// compile time check that *generator implements ICacheKeyGenerator
var _ flam.ICacheKeyGenerator = (*generator)(nil)

func (g *generator) Id() string {
	return g.id
}

// Generate returns the prefix, then the type of the object, named by its
// package and name, such as "models.User", and a colon, when typed, then
// the value of the field of the object, a struct or a pointer to one,
// through any number of pointers. The value is a string or an integer,
// through any number of pointers and interfaces, or formatted by its
// MarshalText or String method, pointer receivers included when the
// field is addressable, as in an object given by pointer. It fails with
// cache.ErrKey for any other object, and for an object of an unnamed
// type when typed.
func (g *generator) Generate(
	obj any,
) (string, error) {
	rv := reflect.ValueOf(obj)
	for rv.Kind() == reflect.Pointer && !rv.IsNil() {
		rv = rv.Elem()
	}
	switch {
	case !rv.IsValid() || rv.Kind() == reflect.Pointer:
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errNil)
	case rv.Kind() != reflect.Struct:
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errNotStruct)
	case g.typed && rv.Type().Name() == "":
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errUnnamed)
	}

	sf, ok := rv.Type().FieldByName(g.field)
	if !ok || !sf.IsExported() {
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errNoField)
	}
	fv, err := rv.FieldByIndexErr(sf.Index)
	if err != nil { // through a nil embedded pointer
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errNil)
	}
	val, ok := generatorFormat(fv)
	if !ok {
		return "", newErrKey(generatorGenerateMessage, g.id, obj, errField)
	}

	key := g.prefix
	if g.typed {
		key += rv.Type().String() + ":"
	}
	return key + val, nil
}

// generatorGenerateMessage is the message of the errors returned by
// Generate.
const generatorGenerateMessage = "unable to generate field cache key"

// generatorFormat returns the string of the value of a field, through any
// number of pointers and interfaces, or false when it is not a string, an
// integer, an encoding.TextMarshaler or a fmt.Stringer, or is nil.
func generatorFormat(
	fv reflect.Value,
) (string, bool) {
	for {
		if text, ok, found := generatorMethod(fv); found {
			return text, ok
		}
		switch fv.Kind() {
		case reflect.Pointer, reflect.Interface:
			if fv.IsNil() {
				return "", false
			}
			fv = fv.Elem()
		case reflect.String:
			return fv.String(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(fv.Int(), 10), true
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return strconv.FormatUint(fv.Uint(), 10), true
		default:
			return "", false
		}
	}
}

// generatorMethod returns the text of the MarshalText or String method of
// the value, or of its address when addressable, and whether the value has
// one. A nil value has none, as its methods may not take nil.
func generatorMethod(
	fv reflect.Value,
) (string, bool, bool) {
	candidates := []reflect.Value{fv}
	if fv.CanAddr() {
		candidates = append(candidates, fv.Addr())
	}
	for _, c := range candidates {
		if (c.Kind() == reflect.Pointer || c.Kind() == reflect.Interface) && c.IsNil() {
			continue
		}
		switch v := c.Interface().(type) {
		case encoding.TextMarshaler:
			text, err := v.MarshalText()
			return string(text), err == nil, true
		case fmt.Stringer:
			return v.String(), true, true
		}
	}
	return "", false, false
}
