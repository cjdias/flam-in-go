package gcs

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the disk configuration, or its default, does
	// not hold a value of the expected type: a non-empty bucket, prefix,
	// endpoint and credentials strings, at most one of the credentials
	// file and JSON, a known credentials type, and an anonymous bool, true
	// only without credentials file nor JSON.
	ErrInvalidField = errors.New("invalid gcs disk field")

	// ErrClient is the error wrapped, along with the error of the client
	// library, by the error returned from the creator when the client
	// cannot be created, such as for invalid credentials, or missing
	// application default credentials.
	ErrClient = errors.New("unable to create gcs disk client")
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

// newErrClient returns the error wrapping ErrClient, joined with the
// cause, with the message and the ErrorFieldID context field.
func newErrClient(
	message string,
	id string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrClient, cause), message).
		With(ErrorFieldID, id)
}
