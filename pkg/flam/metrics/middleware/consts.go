package middleware

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.metrics.middleware"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "metrics"

	// Driver is the driver name of the metrics middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.metrics"
)

// Fields of a metrics middleware configuration.
const (
	// ConfigFieldNamespace is the field holding the namespace of the
	// metrics, the first part of their names, a string, empty for none.
	ConfigFieldNamespace = "namespace"

	// ConfigFieldBuckets is the field holding the upper bounds of the
	// buckets of [MetricDuration], in seconds, a non-empty list of
	// increasing numbers, a []float64 or a []any of whole or floating
	// numbers.
	ConfigFieldBuckets = "buckets"

	// ConfigFieldCodeClass is the field holding the bool telling whether
	// [LabelCode] holds the class of the status, such as "4xx", rather
	// than the status itself, so as to reduce the number of series.
	ConfigFieldCodeClass = "code_class"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultNamespaceConfigPath is the path of the default of
	// [ConfigFieldNamespace], "flam" unless changed.
	DefaultNamespaceConfigPath = "flam.rest.defaults.middlewares.metrics.namespace"

	// DefaultBucketsConfigPath is the path of the default of
	// [ConfigFieldBuckets], the default buckets of the Prometheus client,
	// from 5ms to 10s, unless changed.
	DefaultBucketsConfigPath = "flam.rest.defaults.middlewares.metrics.buckets"

	// DefaultCodeClassConfigPath is the path of the default of
	// [ConfigFieldCodeClass], false unless changed.
	DefaultCodeClassConfigPath = "flam.rest.defaults.middlewares.metrics.code_class"
)

// Names of the metrics, following the namespace and the "rest" subsystem.
const (
	// MetricRequests is the name of the counter of the requests served.
	MetricRequests = "requests_total"

	// MetricDuration is the name of the histogram of the time taken to
	// serve the requests, in seconds.
	MetricDuration = "request_duration_seconds"

	// MetricInFlight is the name of the gauge of the requests being
	// served.
	MetricInFlight = "requests_in_flight"

	// Subsystem is the subsystem of the metrics, following their
	// namespace in their names.
	Subsystem = "rest"
)

// Labels of the metrics.
const (
	// LabelService holds the id of the service of the endpoint.
	LabelService = "service"

	// LabelEndpoint holds the id of the endpoint, 0 for the requests
	// matching no pattern.
	LabelEndpoint = "endpoint"

	// LabelPattern holds the pattern of the endpoint, empty for the
	// requests matching no pattern.
	LabelPattern = "pattern"

	// LabelMethod holds the method of the request, or [MethodOther].
	LabelMethod = "method"

	// LabelCode holds the status of the response, its class with
	// [ConfigFieldCodeClass], or [CodePanic].
	LabelCode = "code"
)

const (
	// MethodOther is the value of [LabelMethod] for the requests of a
	// method other than the standard ones, as the methods are chosen by
	// the clients.
	MethodOther = "OTHER"

	// CodePanic is the value of [LabelCode] for the requests whose handler
	// panicked before writing a response.
	CodePanic = "panic"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultNamespace = "flam"
	defaultCodeClass = false
)

// defaultBuckets returns the default of [ConfigFieldBuckets], a new list
// each time, so that the configuration never shares it.
func defaultBuckets() []float64 {
	return []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}
}
