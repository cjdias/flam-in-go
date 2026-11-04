package jwt

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a non-empty
	// list of known signing algorithms, other than "none", exactly one of
	// a secret, a public key or a public key file, the key fitting every
	// algorithm, an issuer and an audience string, a leeway of 0 or more,
	// and bool optional and require_expiration. The cause of the failure,
	// such as the error reading the public key file, is joined to it.
	ErrInvalidField = errors.New("invalid jwt rest middleware field")

	// ErrMissingToken is the error of the 401 envelope answering a request
	// without a bearer token, when the middleware is not optional.
	ErrMissingToken = errors.New("missing bearer token")

	// ErrInvalidToken is the error of the 401 envelope answering a request
	// whose bearer token fails verification, standing for the cause of
	// the failure, which is not sent to the client.
	ErrInvalidToken = errors.New("invalid bearer token")
)

// newErrCreate returns the error wrapping the sentinel, joined with the
// cause when not nil, with the message and the ErrorFieldID and
// ErrorFieldField context fields.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}
