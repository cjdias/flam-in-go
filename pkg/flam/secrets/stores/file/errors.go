package file

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the store configuration, or its default,
	// does not hold a value of the expected type: a non-empty directory
	// and a trim bool.
	ErrInvalidField = errors.New("invalid file secret store field")

	// ErrRead is the error wrapped, along with the error of the file
	// system, by the error returned from the stores when a secret file, or
	// a directory, exists but cannot be read, such as a directory read as
	// a file, or a symbolic link leading out of the directory of the store.
	ErrRead = errors.New("unable to read secret file")
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

// newErrKey returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the ErrorFieldID and ErrorFieldKey
// context fields.
func newErrKey(
	sentinel error,
	message string,
	id string,
	key string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldKey, key)
}
