package base

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Context fields of the errors of the creators, as the drivers document
// them.
const (
	errorFieldID    = "id"
	errorFieldField = "field"
)

// NewErrInvalidField returns the error wrapping the sentinel of the
// invalid fields of a driver with the message and the id and field context
// fields.
func NewErrInvalidField(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(errorFieldID, id).
		With(errorFieldField, field)
}
