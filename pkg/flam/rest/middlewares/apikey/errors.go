package apikey

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a header and a
	// query string, not both empty, at least one key, all non-empty and
	// distinct, and a bool optional.
	ErrInvalidField = errors.New("invalid apikey rest middleware field")

	// ErrMissingKey is the error of the 401 envelope answering a request
	// without a key, when the middleware is not optional.
	ErrMissingKey = errors.New("missing api key")

	// ErrInvalidKey is the error of the 401 envelope answering a request
	// whose key matches none of the middleware.
	ErrInvalidKey = errors.New("invalid api key")
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
