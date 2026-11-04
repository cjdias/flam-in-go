package compress

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the middleware configuration, or its default,
// does not hold a value of the expected type: a level from -2 to 9, a
// minimum size of 0 or more, a list of non-empty media types and a list
// of at least one of the supported encodings.
var ErrInvalidField = errors.New("invalid compress rest middleware field")

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
