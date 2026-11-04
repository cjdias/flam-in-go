package etag

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the middleware configuration, or its default,
// does not hold a value of the expected type: a maximum size of 1 or
// more.
var ErrInvalidField = errors.New("invalid etag rest middleware field")

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
