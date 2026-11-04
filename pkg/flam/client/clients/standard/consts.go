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
