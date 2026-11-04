package envelope

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a non-empty
	// channel, a level and a bool exposure of the errors.
	ErrInvalidField = errors.New("invalid envelope rest middleware field")

	// ErrInternal is the error of the 500 envelope answering an
	// unexpected panic, standing for its value, which is not sent to the
	// client: the message of an error value is sent only when the errors
	// are exposed.
	ErrInternal = errors.New("internal server error")
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
