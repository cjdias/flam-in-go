package metrics

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc.interceptors.metrics"

const (
	// Id is the id of the interceptor configured by the bundle returned
	// from [NewBundle].
	Id = "metrics"

	// Driver is the driver name of the metrics interceptors, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.grpc.interceptors.driver.metrics"
)

// Fields of a metrics interceptor configuration.
const (
	// ConfigFieldNamespace is the field holding the namespace of the
	// metrics, the first part of their names, a string, empty for none.
	ConfigFieldNamespace = "namespace"

	// ConfigFieldBuckets is the field holding the upper bounds of the
	// buckets of [MetricDuration], in seconds, a non-empty list of
	// increasing numbers, a []float64 or a []any of whole or floating
	// numbers.
	ConfigFieldBuckets = "buckets"
)

// Configuration paths of the values taken by the fields missing from an
// interceptor configuration.
const (
	// DefaultNamespaceConfigPath is the path of the default of
	// [ConfigFieldNamespace], "flam" unless changed.
	DefaultNamespaceConfigPath = "flam.grpc.defaults.interceptors.metrics.namespace"

	// DefaultBucketsConfigPath is the path of the default of
	// [ConfigFieldBuckets], the default buckets of the Prometheus client,
	// from 5ms to 10s, unless changed.
	DefaultBucketsConfigPath = "flam.grpc.defaults.interceptors.metrics.buckets"
)

// Names of the metrics, following the namespace and the "grpc" subsystem.
const (
	// MetricHandled is the name of the counter of the calls served.
	MetricHandled = "server_handled_total"

	// MetricDuration is the name of the histogram of the time taken to
	// serve the calls, in seconds.
	MetricDuration = "server_handling_seconds"

	// Subsystem is the subsystem of the metrics, following their
	// namespace in their names.
	Subsystem = "grpc"
)

// Labels of the metrics.
const (
	// LabelService holds the service of the call, such as "pkg.Users".
	LabelService = "service"

	// LabelMethod holds the method of the call, such as "Get".
	LabelMethod = "method"

	// LabelCode holds the name of the code of the status of the call, such
	// as "OK" or "NotFound", or [CodePanic].
	LabelCode = "code"
)

// CodePanic is the value of [LabelCode] for the calls whose handler
// panicked.
const CodePanic = "panic"

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the interceptor.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of an interceptor configuration.
const (
	defaultNamespace = "flam"
)

// defaultBuckets returns the default of [ConfigFieldBuckets], a new list
// each time, so that the configuration never shares it.
func defaultBuckets() []float64 {
	return []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
}
