package standard

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the configuration does not hold a value of the
// expected type: a bool, a whole number, a non-negative duration, or a
// known logger type or level.
var ErrInvalidField = errors.New("invalid standard database config field")

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
