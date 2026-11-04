package awssm

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the store configuration, or its default,
	// does not hold a valid value: strings for the region, the profile and
	// the credentials, an absolute http or https endpoint, or an empty one,
	// an access key id and a secret access key set together, a non-empty
	// version stage, non-negative durations and a positive int maximum
	// number of attempts.
	ErrInvalidField = errors.New("invalid awssm secret store field")

	// ErrClient is the error wrapped, along with the error of the AWS SDK,
	// by the error returned from the creator when the configuration of the
	// client cannot be loaded, such as for a profile missing from the
	// shared configuration.
	ErrClient = errors.New("unable to create awssm secret store client")

	// ErrRequest is the error wrapped, along with the error of the AWS SDK,
	// by the error returned from the stores when a read fails, but for a
	// secret not found, such as for credentials the service refuses, a
	// service that cannot be reached, or a timeout.
	ErrRequest = errors.New("unable to read awssm secret")

	// ErrResponse is the error wrapped by the error returned from the
	// stores when the version read holds neither a string nor a binary
	// value.
	ErrResponse = errors.New("invalid awssm response")

	// ErrFormat is the error wrapped by the error returned from the stores
	// when the field of a secret, or its fields, are asked for, and its
	// value is not a JSON object.
	ErrFormat = errors.New("awssm secret is not a JSON object")
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
	return newErr(sentinel, message, id, cause).
		With(ErrorFieldKey, key)
}
