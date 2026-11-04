package otlp

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the stream configuration, or its default,
	// does not hold a value of the expected type: a level, a list of
	// non-empty channels, a non-empty endpoint, a bool insecure, headers
	// of string values, a positive timeout and a service name.
	ErrInvalidField = errors.New("invalid otlp log stream field")

	// ErrExporter is the error wrapped, along with the error of the
	// OTLP/HTTP exporter, by the error returned from the creator when the
	// exporter cannot be created, such as for an endpoint that is not a
	// host.
	ErrExporter = errors.New("unable to create otlp log stream exporter")
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

// newErrExporter returns the error wrapping ErrExporter, joined with the
// error of the exporter, with the message and the ErrorFieldID and
// ErrorFieldEndpoint context fields.
func newErrExporter(
	message string,
	id string,
	endpoint string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrExporter, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldEndpoint, endpoint)
}
