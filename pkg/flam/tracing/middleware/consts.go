package middleware

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.tracing"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "tracing"

	// Driver is the driver name of the tracing middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.tracing"

	// ScopeName is the name of the instrumentation scope of the spans of
	// the middlewares, the name of the tracer they start them with.
	ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware"

	// UnmatchedSpanName is the name of the spans of the requests matching
	// the pattern of no endpoint.
	UnmatchedSpanName = "unmatched"
)

// Attributes of the spans of the middlewares, along with the ones of the
// OpenTelemetry semantic conventions.
const (
	// AttributeServiceId is the attribute holding the id of the service of
	// the endpoint, an int.
	AttributeServiceId = "flam.rest.service_id"

	// AttributeEndpointId is the attribute holding the id of the endpoint,
	// an int, left out for the requests matching no endpoint.
	AttributeEndpointId = "flam.rest.endpoint_id"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"
)
