package base

import (
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
)

// errorFieldField is the context field of the errors of the invalid
// fields, as the drivers document it.
const errorFieldField = "field"

// NewErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the id context field of the locker.
func NewErr(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(lock.ErrorFieldID, id)
}

// NewErrInvalidField returns the error wrapping the sentinel of the
// invalid fields of a driver with the message and the id and field
// context fields.
func NewErrInvalidField(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	return NewErr(sentinel, message, id, nil).
		With(errorFieldField, field)
}

// newErrKey returns the error of NewErr with the key context field.
func newErrKey(
	sentinel error,
	message string,
	id string,
	key string,
	cause error,
) *flam.Error {
	return NewErr(sentinel, message, id, cause).
		With(lock.ErrorFieldKey, key)
}
