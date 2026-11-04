package secure

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the middleware configuration, or its default,
// does not hold a value of the expected type: header value strings
// holding no carriage return, line feed or NUL, a max-age duration of 0
// or more and bools.
var ErrInvalidField = errors.New("invalid secure rest middleware field")

// newErrCreate returns the error wrapping the sentinel with the message and
// the ErrorFieldID and ErrorFieldField context fields.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}
