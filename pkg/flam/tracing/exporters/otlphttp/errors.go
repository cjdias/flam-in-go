package otlphttp

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the exporter configuration, or its default,
	// does not hold a value of the expected type: an endpoint and a path
	// strings, an insecure bool, a bag of string headers, a positive
	// timeout duration and a known compression.
	ErrInvalidField = errors.New("invalid otlphttp tracing exporter field")

	// ErrExporter is the error wrapped, along with the error of
	// OpenTelemetry, by the error returned from the creator when the
	// exporter cannot be started, such as when it is insecure while the
	// environment of OpenTelemetry configures a TLS certificate.
	ErrExporter = errors.New("unable to start otlphttp tracing exporter")
)

// newErrCreate returns the error wrapping the sentinel with the message and
// the ErrorFieldID context field, and the ErrorFieldField one when field is
// not empty.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	e := flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
	if field != "" {
		e = e.With(ErrorFieldField, field)
	}
	return e
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// cause, with the message and the ErrorFieldID context field.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	return newErrCreate(fmt.Errorf("%w: %w", sentinel, cause), message, id, "")
}
