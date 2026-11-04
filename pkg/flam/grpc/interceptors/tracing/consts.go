package tracing

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc.interceptors.tracing"

const (
	// Id is the id of the interceptor configured by the bundle returned
	// from [NewBundle].
	Id = "tracing"

	// Driver is the driver name of the tracing interceptors, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.grpc.interceptors.driver.tracing"

	// ScopeName is the name of the instrumentation scope of the spans of
	// the interceptors, the name of the tracer they start them with.
	ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/tracing"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the interceptor.
	ErrorFieldID = "id"
)
