package otlp

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.streams.otlp"

// Driver is the driver name of the otlp streams, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.log.streams.driver.otlp"

// ScopeName is the name of the instrumentation scope of the records.
const ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/log"

// Fields of an otlp stream configuration.
const (
	// ConfigFieldLevel is the field holding the least severe level the
	// stream keeps, a whole number, a [flam.LogLevel] included, or a level
	// name such as "info", as [flam.LogLevel] From reads it.
	ConfigFieldLevel = "level"

	// ConfigFieldChannels is the field holding the list of the channels
	// the stream listens to, "*" listening to all of them. It defaults to
	// none: the stream then keeps only the broadcast entries.
	ConfigFieldChannels = "channels"

	// ConfigFieldEndpoint is the field holding the host and port of the
	// collector, such as "localhost:4318", the records being posted to its
	// "/v1/logs" path.
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldInsecure is the field holding whether the collector is
	// reached with HTTP, instead of HTTPS.
	ConfigFieldInsecure = "insecure"

	// ConfigFieldHeaders is the field holding the headers of the requests
	// to the collector, a bag or a map of string values, such as an
	// authorization header.
	ConfigFieldHeaders = "headers"

	// ConfigFieldTimeout is the field holding the timeout of each request
	// to the collector, and of the export of the records left when the
	// stream is closed, a positive time.Duration or a string such as
	// "10s".
	ConfigFieldTimeout = "timeout"

	// ConfigFieldServiceName is the field holding the service name of the
	// resource of the records, the empty string leaving the default of
	// the OpenTelemetry SDK, such as "unknown_service:app".
	ConfigFieldServiceName = "service_name"
)

// Configuration paths of the values taken by the fields missing from a
// stream configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.log.defaults.streams.otlp.level"

	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], "localhost:4318" unless changed.
	DefaultEndpointConfigPath = "flam.log.defaults.streams.otlp.endpoint"

	// DefaultInsecureConfigPath is the path of the default of
	// [ConfigFieldInsecure], unset unless configured, then false.
	DefaultInsecureConfigPath = "flam.log.defaults.streams.otlp.insecure"

	// DefaultHeadersConfigPath is the path of the default of
	// [ConfigFieldHeaders], unset unless configured, then none.
	DefaultHeadersConfigPath = "flam.log.defaults.streams.otlp.headers"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], "10s" unless changed.
	DefaultTimeoutConfigPath = "flam.log.defaults.streams.otlp.timeout"

	// DefaultServiceNameConfigPath is the path of the default of
	// [ConfigFieldServiceName], unset unless configured, then empty.
	DefaultServiceNameConfigPath = "flam.log.defaults.streams.otlp.service_name"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the stream.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldEndpoint holds the endpoint of the collector.
	ErrorFieldEndpoint = "endpoint"
)

// Defaults of the fields of a stream configuration.
const (
	defaultLevel    = flam.LogInfo
	defaultEndpoint = "localhost:4318"
	defaultTimeout  = 10 * time.Second
)
