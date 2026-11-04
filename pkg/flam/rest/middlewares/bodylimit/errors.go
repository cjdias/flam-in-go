package bodylimit

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: an int
	// max_bytes of more than 0.
	ErrInvalidField = errors.New("invalid body limit rest middleware field")

	// ErrTooLarge is the error of the 413 envelope answering a request
	// announcing a body longer than the maximum of the middleware.
	ErrTooLarge = errors.New("request body too large")
)

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
