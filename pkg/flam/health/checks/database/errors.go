package database

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the check configuration, or its default,
	// does not hold a value of the expected type: a kind read by
	// [github.com/cjdias/flam-in-go/pkg/flam/health.ParseKind] and a
	// non-empty connection id.
	ErrInvalidField = errors.New("invalid database health check field")

	// ErrNoFactory is the error wrapped by the error returned from the
	// creator when the container provides no
	// [flam.IDatabaseConnectionFactory], as the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/database] is not registered.
	ErrNoFactory = errors.New("no database connection factory")

	// ErrUnknownConnection is the error wrapped by the error returned from
	// the creator when the connection factory has no connection of the id.
	ErrUnknownConnection = errors.New("unknown database connection")

	// ErrConnection is the error wrapped, along with the error of the
	// connection factory, or of the connection, by the error of a check
	// when the connection, or its pool, cannot be taken.
	ErrConnection = errors.New("unable to get database connection")

	// ErrPing is the error wrapped, along with the error of the ping, by
	// the error of a check when the database does not answer.
	ErrPing = errors.New("unable to ping database")
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
