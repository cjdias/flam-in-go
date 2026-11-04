package database

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidConnections is the error wrapped by the error of the
	// container when the value at [ConnectionsConfigPath] is not a list of
	// connection ids.
	ErrInvalidConnections = errors.New("invalid metrics database connections")

	// ErrMissingFactory is the error wrapped by the error of the
	// container when connections are listed at [ConnectionsConfigPath]
	// while the container provides no [flam.IDatabaseConnectionFactory].
	ErrMissingFactory = errors.New("database connection factory not provided by the container")

	// ErrConnection is the error wrapped, along with the error of the
	// factory or of the connection, by the error of the container when a
	// connection, or its pool, cannot be taken.
	ErrConnection = errors.New("unable to get metrics database connection")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message.
func newErr(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message)
}
