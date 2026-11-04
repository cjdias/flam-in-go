package standard

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.client.clients.standard"

const (
	// Id is the id of the default client configured by the bundle
	// returned from [NewBundle], without base URL.
	Id = "standard"

	// Driver is the driver name of the standard clients, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.client.clients.driver.standard"
)

// Fields of a standard client configuration.
const (
	// ConfigFieldBaseURL is the field holding the absolute URL the
	// relative URLs of the requests are resolved against, as
	// [net/url.URL.ResolveReference] does, such as
	// "https://users.internal/v1/", for which "users/7" is
	// "https://users.internal/v1/users/7" while "/users/7" is
	// "https://users.internal/users/7". Unless set, the requests must have
	// an absolute URL.
	ConfigFieldBaseURL = "base_url"

	// ConfigFieldTimeout is the field holding the time allowed to each
	// attempt of a request, its response body included, a duration such
	// as "30s", 0 allowing any time.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldHeaders is the field holding the default headers of the
	// requests, a bag of header names and string values, added to the
	// requests not setting them.
	ConfigFieldHeaders = "headers"

	// ConfigFieldBreaker is the field holding the id of the circuit
	// breaker of the client, in the circuit breaker factory, an empty id
	// leaving the client without circuit breaker.
	ConfigFieldBreaker = "breaker"

	// ConfigFieldRetries is the field holding the number of times a failed
	// request is sent again, a non-negative whole number, 0 sending each
	// request once.
	ConfigFieldRetries = "retries"

	// ConfigFieldRetryDelay is the field holding the delay before the
	// first retry of a request, a duration such as "100ms", doubled at each
	// following retry.
	ConfigFieldRetryDelay = "retry_delay"

	// ConfigFieldRetryMaxDelay is the field holding the maximum delay
	// before a retry, a duration not shorter than the retry delay.
	ConfigFieldRetryMaxDelay = "retry_max_delay"

	// ConfigFieldTracing is the field holding the bool telling whether the
	// client traces its requests, which it does only when the container
	// provides a tracer provider and a propagator.
	ConfigFieldTracing = "tracing"

	// ConfigFieldMetrics is the field holding the bool telling whether the
	// client measures its requests, which it does only when the container
	// provides a Prometheus registerer.
	ConfigFieldMetrics = "metrics"

	// ConfigFieldNamespace is the field holding the namespace of the
	// metrics of the client, the first part of their names, a string,
	// empty for none.
	ConfigFieldNamespace = "namespace"
)

// Configuration paths of the values taken by the fields missing from a
// client configuration.
const (
	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], "30s" unless set.
	DefaultTimeoutConfigPath = "flam.client.defaults.clients.standard.timeout"

	// DefaultHeadersConfigPath is the path of the default of
	// [ConfigFieldHeaders], unset unless set by the application: the
	// requests then have no default header.
	DefaultHeadersConfigPath = "flam.client.defaults.clients.standard.headers"

	// DefaultRetriesConfigPath is the path of the default of
	// [ConfigFieldRetries], 0 unless set.
	DefaultRetriesConfigPath = "flam.client.defaults.clients.standard.retries"

	// DefaultRetryDelayConfigPath is the path of the default of
	// [ConfigFieldRetryDelay], "100ms" unless set.
	DefaultRetryDelayConfigPath = "flam.client.defaults.clients.standard.retry_delay"

	// DefaultRetryMaxDelayConfigPath is the path of the default of
	// [ConfigFieldRetryMaxDelay], "2s" unless set.
	DefaultRetryMaxDelayConfigPath = "flam.client.defaults.clients.standard.retry_max_delay"

	// DefaultTracingConfigPath is the path of the default of
	// [ConfigFieldTracing], true unless set.
	DefaultTracingConfigPath = "flam.client.defaults.clients.standard.tracing"

	// DefaultMetricsConfigPath is the path of the default of
	// [ConfigFieldMetrics], true unless set.
	DefaultMetricsConfigPath = "flam.client.defaults.clients.standard.metrics"

	// DefaultNamespaceConfigPath is the path of the default of
	// [ConfigFieldNamespace], "flam" unless set.
	DefaultNamespaceConfigPath = "flam.client.defaults.clients.standard.namespace"
)

// ScopeName is the name of the instrumentation scope of the spans of the
// clients, the name of the tracer they start them with.
const ScopeName = "github.com/cjdias/flam-in-go/pkg/flam/client/clients/standard"

// Names of the metrics, following the namespace and the [Subsystem].
const (
	// MetricRequests is the name of the counter of the attempts of the
	// requests, retries included.
	MetricRequests = "requests_total"

	// MetricDuration is the name of the histogram of the time taken by the
	// attempts of the requests to get their response headers, in seconds.
	MetricDuration = "request_duration_seconds"

	// Subsystem is the subsystem of the metrics, following their
	// namespace in their names, such as "flam_client_requests_total".
	Subsystem = "client"
)

// Labels of the metrics.
const (
	// LabelClient holds the id of the client.
	LabelClient = "client"

	// LabelMethod holds the method of the request, or [MethodOther].
	LabelMethod = "method"

	// LabelCode holds the status of the response, such as "200", or the
	// type of the error of an attempt without response: [ErrorTypeOpen],
	// [ErrorTypeTimeout], [ErrorTypeCanceled] or "_OTHER".
	LabelCode = "code"
)

const (
	// MethodOther is the value of [LabelMethod] for the requests of a
	// method other than the standard ones.
	MethodOther = "OTHER"

	// ErrorTypeOpen is the type of the error of an attempt refused by the
	// circuit breaker, held by the "error.type" attribute of its span and
	// by [LabelCode].
	ErrorTypeOpen = "circuit_open"

	// ErrorTypeTimeout is the type of the error of an attempt whose
	// timeout, or the deadline of its context, elapsed.
	ErrorTypeTimeout = "timeout"

	// ErrorTypeCanceled is the type of the error of an attempt whose
	// context was cancelled.
	ErrorTypeCanceled = "canceled"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the clients.
const (
	// ErrorFieldID holds the id of the client.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldBreaker holds the id of the circuit breaker of the client.
	ErrorFieldBreaker = "breaker"

	// ErrorFieldMethod holds the method of a request.
	ErrorFieldMethod = "method"

	// ErrorFieldURL holds the URL of a request, resolved against the base
	// URL of the client when it could be.
	ErrorFieldURL = "url"
)

// Defaults of the fields of a client configuration.
const (
	defaultTimeout       = 30 * time.Second
	defaultRetries       = 0
	defaultRetryDelay    = 100 * time.Millisecond
	defaultRetryMaxDelay = 2 * time.Second
	defaultTracing       = true
	defaultMetrics       = true
	defaultNamespace     = "flam"
)

// Settings of the transport of the clients, those of
// [net/http.DefaultTransport].
const (
	transportDialTimeout           = 30 * time.Second
	transportKeepAlive             = 30 * time.Second
	transportMaxIdleConns          = 100
	transportIdleConnTimeout       = 90 * time.Second
	transportTLSHandshakeTimeout   = 10 * time.Second
	transportExpectContinueTimeout = time.Second
)

// discardLimit is the number of bytes read from the body of a response
// discarded for a retry, so that its connection may be reused.
const discardLimit = 4096
