package tracing

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrMissingTracing is the error wrapped by the error returned from the
	// creator when the container provides no
	// [go.opentelemetry.io/otel/trace.TracerProvider] or no
	// [go.opentelemetry.io/otel/propagation.TextMapPropagator], as when
	// the bundle of [github.com/cjdias/flam-in-go/pkg/flam/tracing] is not
	// registered.
	ErrMissingTracing = errors.New("tracer provider or propagator not provided by the container")

	// ErrPanicked is the error recorded on the span of a call whose
	// handler panicked, the panic then going on.
	ErrPanicked = errors.New("grpc handler panicked")
)

// newErrCreate returns the error wrapping the sentinel with the message and
// the ErrorFieldID context field.
func newErrCreate(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}
