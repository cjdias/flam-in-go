package cors

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: lists of
	// non-empty origins, methods and header names, a bool
	// allow_credentials and a max_age duration of 0 or more.
	ErrInvalidField = errors.New("invalid cors rest middleware field")

	// ErrWildcardCredentials is the error wrapped by the error returned
	// from the creator when the credentials are allowed along with the "*"
	// origin, or the "*" exposed header, which the browsers then ignore,
	// the field listing it being held by [ErrorFieldField].
	ErrWildcardCredentials = errors.New("cors credentials allowed along with a wildcard")
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
