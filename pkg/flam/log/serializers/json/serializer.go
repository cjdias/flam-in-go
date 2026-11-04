package json

import (
	"bytes"
	"encoding"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// newSerializer returns a JSON serializerImpl of the given id, formatting
// the timestamps with the layout.
func newSerializer(
	id string,
	format string,
) flam.ILogSerializer {
	return &serializerImpl{id: id, format: format}
}

// serializerImpl is the JSON [flam.ILogSerializer]. It holds no state but
// its id and layout, so it is safe for concurrent use and has nothing to
// close.
type serializerImpl struct {
	id     string
	format string
}

// compile time check that *serializerImpl implements ILogSerializer
var _ flam.ILogSerializer = (*serializerImpl)(nil)

func (s *serializerImpl) Id() string {
	return s.id
}

func (s *serializerImpl) Close() error {
	return nil
}

func (s *serializerImpl) Serialize(
	timestamp time.Time,
	level flam.LogLevel,
	message string,
	ctx flam.IBag,
) string {
	entry := map[string]any{}
	if !value.IsNil(ctx) {
		entry = serializerObject(ctx)
	}
	fields := map[string]any{
		FieldTime:    timestamp.Format(s.format),
		FieldLevel:   strings.ToUpper(level.String()),
		FieldMessage: message,
	}
	for key, val := range fields {
		entry[key] = val
	}

	line, err := serializerEncode(entry)
	if err != nil {
		fields[FieldContextError] = err.Error()
		line, _ = serializerEncode(fields) // strings only
	}
	return line
}

// serializerEncode returns the JSON encoding of the value, without
// escaping the HTML characters, nor a trailing newline.
func serializerEncode(
	val any,
) (string, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(val); err != nil {
		return "", err
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

// serializerObject returns a map holding the entries of the bag, converted
// by serializerValue.
func serializerObject(
	bag flam.IBag,
) map[string]any {
	object := map[string]any{}
	for key, val := range bag.All() {
		object[key] = serializerValue(val)
	}
	return object
}

// serializerValue converts a context value for encoding/json, at any
// depth: a bag, or a map of string keys, becomes a map[string]any, an
// error its message, and a slice or an array, but of bytes, a []any, their
// items converted. A value encoding itself, as a json.Marshaler or an
// encoding.TextMarshaler, such as a time.Time, is kept.
func serializerValue(
	val any,
) any {
	switch v := val.(type) {
	case nil, json.Marshaler, encoding.TextMarshaler:
		return val
	case flam.IBag:
		if value.IsNil(v) {
			return nil
		}
		return serializerObject(v)
	case error:
		if value.IsNil(v) {
			return nil
		}
		return v.Error()
	}

	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Map:
		if rv.Type().Key().Kind() != reflect.String {
			return val
		}
		if rv.IsNil() {
			return nil
		}
		object := make(map[string]any, rv.Len())
		for entry := rv.MapRange(); entry.Next(); {
			object[entry.Key().String()] = serializerValue(entry.Value().Interface())
		}
		return object
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return val // as encoding/json encodes the bytes
		}
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return nil
		}
		items := make([]any, rv.Len())
		for i := range rv.Len() {
			items[i] = serializerValue(rv.Index(i).Interface())
		}
		return items
	}
	return val
}
