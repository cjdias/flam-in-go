package env

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the store configuration, or its default,
	// does not hold a value of the expected type: a string prefix.
	ErrInvalidField = errors.New("invalid env secret store field")
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

// newErrKey returns the error wrapping the sentinel, such as
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.ErrNotFound], with the
// message and the ErrorFieldID and ErrorFieldKey context fields, and the
// ErrorFieldVariable one when the variable is not empty.
func newErrKey(
	sentinel error,
	message string,
	id string,
	key string,
	variable string,
) *flam.Error {
	e := flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldKey, key)
	if variable != "" {
		e = e.With(ErrorFieldVariable, variable)
	}
	return e
}
