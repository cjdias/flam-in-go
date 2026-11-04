package authz

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a known policy,
	// a list of rules, each matching endpoints or patterns, all
	// well-formed, and requiring non-empty scopes and roles or being
	// public, a roles claim path without empty segment, a map of the
	// principals to their non-empty roles, and a challenge string.
	ErrInvalidField = errors.New("invalid authz rest middleware field")

	// ErrUnauthenticated is the error of the 401 envelope answering a
	// request of a non-public endpoint without claims or principal.
	ErrUnauthenticated = errors.New("authentication required")

	// ErrForbidden is the error of the 403 envelope answering a request
	// of an endpoint denied by the policy, or without the scopes or roles
	// its rule requires.
	ErrForbidden = errors.New("insufficient permissions")
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
