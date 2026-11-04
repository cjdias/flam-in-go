package json

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
)

// serializer is the [flam.ICacheSerializer] of the JSON encoding.
type serializer struct {
	id string
}

// compile time check that *serializer implements ICacheSerializer
var _ flam.ICacheSerializer = (*serializer)(nil)

func (s *serializer) Id() string {
	return s.id
}

// Serialize returns the JSON encoding of the object, failing with
// cache.ErrSerialize when it cannot be encoded.
func (s *serializer) Serialize(
	obj any,
) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", s.newErr(cache.ErrSerialize, serializerSerializeMessage, obj, err)
	}
	return string(data), nil
}

// Deserialize decodes the JSON encoding into the object, failing with
// cache.ErrDeserialize when it is not a non-nil pointer or the data does
// not decode into it.
func (s *serializer) Deserialize(
	data string,
	obj any,
) error {
	if rv := reflect.ValueOf(obj); rv.Kind() != reflect.Pointer || rv.IsNil() {
		return s.newErr(cache.ErrDeserialize, serializerDeserializeMessage, obj, errNotPointer)
	}
	if err := json.Unmarshal([]byte(data), obj); err != nil {
		return s.newErr(cache.ErrDeserialize, serializerDeserializeMessage, obj, err)
	}
	return nil
}

// Messages of the errors returned by the serializer.
const (
	serializerSerializeMessage   = "unable to serialize json cache value"
	serializerDeserializeMessage = "unable to deserialize json cache value"
)

// errNotPointer is the cause of the errors of the objects Deserialize
// cannot decode into.
var errNotPointer = errors.New("not a non-nil pointer")

// newErr returns the error wrapping the sentinel, joined with the cause,
// with the message and the cache.ErrorFieldID and cache.ErrorFieldType
// context fields.
func (s *serializer) newErr(
	sentinel error,
	message string,
	obj any,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, cause), message).
		With(cache.ErrorFieldID, s.id).
		With(cache.ErrorFieldType, fmt.Sprintf("%T", obj))
}
