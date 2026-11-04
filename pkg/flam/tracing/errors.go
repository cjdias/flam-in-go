package tracing

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned when the
	// tracer provider is created, and so by the container, when a value of
	// its configuration is not of the expected type: bools at
	// [EnabledConfigPath], [SamplerParentBasedConfigPath] and
	// [BatchConfigPath], strings at [ServiceNameConfigPath] and
	// [ServiceVersionConfigPath], a bag of strings, bools, ints, int64s
	// and float64s at [ResourceConfigPath], a known sampler at
	// [SamplerConfigPath], a ratio from 0 to 1 at [SamplerRatioConfigPath],
	// and a list of ids at [ExportConfigPath], the values of the disabled
	// tracing not being read. It is also wrapped by the error returned
	// from the boot of the bundle when [GlobalConfigPath] does not hold a
	// bool, and from its closing when [ShutdownTimeoutConfigPath] does not
	// hold a non-negative duration.
	ErrInvalidField = errors.New("invalid tracing field")

	// ErrExporter is the error wrapped, along with the error of the
	// [flam.ITracingExporterFactory], by the error returned when the
	// tracer provider is created, and so by the container, when an
	// exporter of [ExportConfigPath] cannot be taken from the factory.
	ErrExporter = errors.New("unable to get tracing exporter")

	// ErrShutdown is the error wrapped, along with the error of the tracer
	// provider, by the error returned from the closing of the bundle when
	// the provider fails to shut down, such as when an exporter fails to
	// export the spans left in time.
	ErrShutdown = errors.New("unable to shut down tracer provider")
)

// newErrField returns the error wrapping ErrInvalidField with the message
// and the ErrorFieldField context field.
func newErrField(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldField, path)
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// cause, with the message.
func newErrJoined(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, cause), message)
}
