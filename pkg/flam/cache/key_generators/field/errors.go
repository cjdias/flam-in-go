package field

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the key generator configuration does not hold a
// value of the expected type: a non-empty field name, a prefix string and
// a type bool.
var ErrInvalidField = errors.New("invalid field cache key generator field")

// Causes of the errors of the objects Generate cannot key.
var (
	errNil       = errors.New("nil object")
	errNotStruct = errors.New("not a struct")
	errUnnamed   = errors.New("unnamed type")
	errNoField   = errors.New("no such field")
	errField     = errors.New("field value is not a string, an integer, a fmt.Stringer nor an encoding.TextMarshaler")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrKey returns the error wrapping cache.ErrKey, joined with the
// cause, with the message and the cache.ErrorFieldID and
// cache.ErrorFieldType context fields.
func newErrKey(
	message string,
	id string,
	obj any,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", cache.ErrKey, cause), message).
		With(cache.ErrorFieldID, id).
		With(cache.ErrorFieldType, fmt.Sprintf("%T", obj))
}
