package source

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the source configuration, or its default,
	// does not hold a value of the expected type: an int priority, a
	// reload bool, a bag of values and a store string, not empty when
	// there are values.
	ErrInvalidField = errors.New("invalid secrets config source field")

	// ErrInvalidValue is the error wrapped by the error returned from the
	// creator when a value of [ConfigFieldValues] is not a non-empty
	// string, has an invalid path, or has a path inside the path of
	// another value.
	ErrInvalidValue = errors.New("invalid secrets config source value")

	// ErrMissingFactory is the error wrapped, along with the error of the
	// container, by the error returned from the creator when the container
	// provides no [flam.ISecretStoreFactory]: register the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/secrets].
	ErrMissingFactory = errors.New("secret store factory not provided by the container")

	// ErrStore is the error wrapped, along with the error of the
	// [flam.ISecretStoreFactory], by the error returned from the creator
	// when the store cannot be taken from the factory.
	ErrStore = errors.New("unable to get secrets config source store")

	// ErrSecret is the error wrapped, along with the error of the store,
	// by the error returned from the creator or Reload when a secret
	// cannot be read.
	ErrSecret = errors.New("unable to read secrets config source secret")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message and the ErrorFieldID context field.
func newErr(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return newErr(ErrInvalidField, message, id, nil).
		With(ErrorFieldField, field)
}

// newErrInvalidValue returns the error wrapping ErrInvalidValue with the
// message and the ErrorFieldID and ErrorFieldPath context fields.
func newErrInvalidValue(
	message string,
	id string,
	path string,
) *flam.Error {
	return newErr(ErrInvalidValue, message, id, nil).
		With(ErrorFieldPath, path)
}

// newErrSecret returns the error wrapping ErrSecret, joined with the error
// of the store, with the message and the ErrorFieldID, ErrorFieldPath and
// ErrorFieldKey context fields.
func newErrSecret(
	message string,
	id string,
	path string,
	key string,
	cause error,
) *flam.Error {
	return newErr(ErrSecret, message, id, cause).
		With(ErrorFieldPath, path).
		With(ErrorFieldKey, key)
}
