package gcpsm

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the store configuration, or its default,
	// does not hold a valid value: a non-empty project and version holding
	// no slash, strings for the endpoint and the credentials, a single one
	// of the credentials file and JSON, a known credentials type, an
	// endpoint when insecure, a bool insecure and non-negative durations.
	ErrInvalidField = errors.New("invalid gcpsm secret store field")

	// ErrClient is the error wrapped, along with the error of the Google
	// Cloud client, by the error returned from the creator when the client
	// cannot be created, such as for credentials that cannot be loaded.
	ErrClient = errors.New("unable to create gcpsm secret store client")

	// ErrRequest is the error wrapped, along with the error of the Google
	// Cloud client, by the error returned from the stores when a read
	// fails, but for a secret not found, such as for credentials the
	// service refuses, a service that cannot be reached, or a timeout.
	ErrRequest = errors.New("unable to read gcpsm secret")

	// ErrResponse is the error wrapped by the error returned from the
	// stores when the payload read does not match its CRC32C checksum.
	ErrResponse = errors.New("invalid gcpsm response")

	// ErrFormat is the error wrapped by the error returned from the stores
	// when the field of a secret, or its fields, are asked for, and its
	// value is not a JSON object.
	ErrFormat = errors.New("gcpsm secret is not a JSON object")
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
