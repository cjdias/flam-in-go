package envelope

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the converter configuration, or its default,
	// does not hold a value of the expected type: an int status.
	ErrInvalidField = errors.New("invalid envelope validation converter field")

	// ErrInvalidStatus is the error wrapped by the error returned from
	// [NewConverter] when the status is out of the 400 to 599 range of the
	// error statuses.
	ErrInvalidStatus = errors.New("invalid envelope validation converter status")
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

// newErrInvalidStatus returns the error wrapping ErrInvalidStatus with the
// message and the ErrorFieldID and ErrorFieldStatus context fields.
func newErrInvalidStatus(
	message string,
	id string,
	status int,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidStatus, message).
		With(ErrorFieldID, id).
		With(ErrorFieldStatus, status)
}
