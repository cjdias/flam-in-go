package standard

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the client configuration, or its default,
	// does not hold a value of the expected type: an absolute base URL,
	// unless empty, a bag of string headers, a breaker string, a
	// non-negative number of retries, non-negative durations, a maximum
	// retry delay not shorter than the retry delay, tracing and metrics
	// bools and a namespace string.
	ErrInvalidField = errors.New("invalid standard client field")

	// ErrBreaker is the error wrapped, along with the error of the
	// [flam.IClientCircuitBreakerFactory], by the error returned from the
	// creator when the circuit breaker cannot be taken from the factory.
	ErrBreaker = errors.New("unable to get standard client circuit breaker")

	// ErrRegister is the error wrapped, along with the error of the
	// Prometheus registerer, by the error returned from the creator when
	// the metrics of the client cannot be registered, such as when its
	// namespace is not a valid metric name, or when other metrics of the
	// same names are registered.
	ErrRegister = errors.New("unable to register standard client metrics")

	// ErrInvalidRequest is the error wrapped by the error returned from Do
	// when the request, or its URL, is nil, or its URL is not absolute
	// once resolved against the base URL, and from Request when the
	// request cannot be created, along with the error of its creation. The
	// request is not sent.
	ErrInvalidRequest = errors.New("invalid standard client request")

	// ErrOpen is the error wrapped by the error returned from Do and
	// Request when the circuit breaker of the client refuses the request,
	// or one of its retries, its circuit being open, or half open with its
	// trial requests under way. The request is not sent.
	ErrOpen = errors.New("standard client circuit open")

	// ErrRequest is the error wrapped, along with the error of the
	// [net/http] client, or of the context, by the error returned from Do
	// and Request when the request cannot be sent, or its response cannot
	// be read, such as when its context is done, its timeout elapses or
	// the server cannot be reached.
	ErrRequest = errors.New("unable to do standard client request")
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
