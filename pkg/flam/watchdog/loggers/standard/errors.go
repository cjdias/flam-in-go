package standard

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the adaptor configuration, or its default,
	// does not hold a value of the expected type: a non-empty channel and
	// levels.
	ErrInvalidField = errors.New("invalid standard watchdog logger field")

	// ErrMissingLogger is the error wrapped by the error returned from the
	// creator when the container provides no [flam.ILogger].
	ErrMissingLogger = errors.New("logger not provided by the container")
)

// newErrCreate returns the error wrapping the sentinel with the message and
// the ErrorFieldID context field, and the ErrorFieldField one when field is
// not empty.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	e := flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
	if field != "" {
		e = e.With(ErrorFieldField, field)
	}
	return e
}
