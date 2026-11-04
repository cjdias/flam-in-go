package standard

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the connection configuration, or its
	// default, does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid standard mongo connection field")

	// ErrTLS is the error wrapped, along with the error of reading them,
	// by the error returned from the creator when the certificate files of
	// the TLS fields cannot be read.
	ErrTLS = errors.New("unable to read standard mongo connection certificates")

	// ErrConnect is the error wrapped, along with the error of the driver,
	// by the error returned from the creator when the client options, such
	// as the URI, are invalid.
	ErrConnect = errors.New("unable to connect standard mongo connection")

	// errNoCertificate is the error of a CA file holding no certificate.
	errNoCertificate = errors.New("no certificate found")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, creatorCreateMessage).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrTLS returns the error wrapping ErrTLS, joined with the error of
// reading the file of the field, with the ErrorFieldID and ErrorFieldField
// context fields.
func newErrTLS(
	id string,
	field string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrTLS, err), creatorCreateMessage).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrConnect returns the error wrapping ErrConnect, joined with the
// error of the driver, with the ErrorFieldID context field.
func newErrConnect(
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrConnect, err), creatorCreateMessage).
		With(ErrorFieldID, id)
}
