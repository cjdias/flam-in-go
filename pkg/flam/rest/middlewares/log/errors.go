package log

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a non-empty
	// channel, levels, a bool parse_json, an int max_body of 0 or more,
	// and lists of header and query parameter names.
	ErrInvalidField = errors.New("invalid log rest middleware field")

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
