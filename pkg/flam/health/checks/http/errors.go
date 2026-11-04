package http

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the check configuration, or its default,
	// does not hold a value of the expected type: a kind read by
	// [github.com/cjdias/flam-in-go/pkg/flam/health.ParseKind], a URL,
	// absolute without client, a client id, statuses from 100 to 599, the
	// lowest first, and a non-negative duration.
	ErrInvalidField = errors.New("invalid http health check field")

	// ErrNoFactory is the error wrapped by the error returned from the
	// creator when a client is set but the container provides no
	// [flam.IClientFactory], as the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/client] is not registered.
	ErrNoFactory = errors.New("no client factory")

	// ErrUnknownClient is the error wrapped by the error returned from the
	// creator when the client factory has no client of the id.
	ErrUnknownClient = errors.New("unknown client")

	// ErrClient is the error wrapped, along with the error of the client
	// factory, by the error of a check when the client cannot be taken.
	ErrClient = errors.New("unable to get client")

	// ErrRequest is the error wrapped, along with the error of the
	// request, by the error of a check when the request fails, such as
	// when the server cannot be reached or does not answer in time.
	ErrRequest = errors.New("unable to request http health check url")

	// ErrStatus is the error wrapped, along with the status, by the error
	// of a check when the status of the response is out of its range.
	ErrStatus = errors.New("unhealthy http status")
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
