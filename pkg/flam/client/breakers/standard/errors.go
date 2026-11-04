package standard

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the circuit breaker configuration, or its
	// default, does not hold a value of the expected type: a positive
	// failure threshold, open timeout and number of half open requests.
	ErrInvalidField = errors.New("invalid standard client circuit breaker field")
)

// newErrInvalidField returns the error wrapping ErrInvalidField, with the
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
