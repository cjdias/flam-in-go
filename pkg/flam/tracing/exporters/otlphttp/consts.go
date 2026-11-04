package otlphttp

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.tracing.exporters.otlphttp"

const (
	// Id is the id of the exporter configured by the bundle returned from
	// [NewBundle].
	Id = "otlphttp"

	// Driver is the driver name of the otlphttp exporters, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.tracing.exporters.driver.otlphttp"
)

// Fields of an otlphttp exporter configuration.
const (
	// ConfigFieldEndpoint is the field holding the host and the port of
	// the collector the spans are sent to, such as "collector:4318", or
	// empty to keep the endpoint OpenTelemetry reads from the
	// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT
	// environment variables, or else "localhost:4318".
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldPath is the field holding the path of the URL the spans
	// are sent to, or empty to keep "/v1/traces".
	ConfigFieldPath = "path"

	// ConfigFieldInsecure is the field holding a bool telling whether the
	// spans are sent over plain HTTP rather than HTTPS.
	ConfigFieldInsecure = "insecure"

	// ConfigFieldHeaders is the field holding the bag of the headers sent
	// with the spans, such as an authorization header, each name holding
	// its value, a string.
	ConfigFieldHeaders = "headers"

	// ConfigFieldTimeout is the field holding the longest time an export
	// of spans takes, retries included, a positive duration, such as
	// "10s".
	ConfigFieldTimeout = "timeout"

	// ConfigFieldCompression is the field holding the compression of the
	// spans sent: [CompressionNone] or [CompressionGzip].
	ConfigFieldCompression = "compression"
)

// Compressions held by [ConfigFieldCompression].
const (
	// CompressionNone sends the spans uncompressed.
	CompressionNone = "none"

	// CompressionGzip sends the spans compressed with gzip.
	CompressionGzip = "gzip"
)

// Configuration paths of the values taken by the fields missing from an
// exporter configuration.
const (
	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], empty unless changed.
	DefaultEndpointConfigPath = "flam.tracing.defaults.exporters.otlphttp.endpoint"

	// DefaultPathConfigPath is the path of the default of
	// [ConfigFieldPath], empty unless changed.
	DefaultPathConfigPath = "flam.tracing.defaults.exporters.otlphttp.path"

	// DefaultInsecureConfigPath is the path of the default of
	// [ConfigFieldInsecure], false unless changed.
	DefaultInsecureConfigPath = "flam.tracing.defaults.exporters.otlphttp.insecure"

	// DefaultHeadersConfigPath is the path of the default of
	// [ConfigFieldHeaders], unset unless set by the application: the
	// spans are then sent with no header of their own.
	DefaultHeadersConfigPath = "flam.tracing.defaults.exporters.otlphttp.headers"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], "10s" unless changed.
	DefaultTimeoutConfigPath = "flam.tracing.defaults.exporters.otlphttp.timeout"

	// DefaultCompressionConfigPath is the path of the default of
	// [ConfigFieldCompression], [CompressionNone] unless changed.
	DefaultCompressionConfigPath = "flam.tracing.defaults.exporters.otlphttp.compression"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the exporter.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of an exporter configuration.
const (
	defaultEndpoint    = ""
	defaultPath        = ""
	defaultInsecure    = false
	defaultTimeout     = 10 * time.Second
	defaultCompression = CompressionNone
)
