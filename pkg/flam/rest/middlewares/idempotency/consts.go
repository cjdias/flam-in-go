package idempotency

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.idempotency"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "idempotency"

	// Driver is the driver name of the idempotency middlewares, held by
	// the [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.idempotency"
)

// Fields of an idempotency middleware configuration.
const (
	// ConfigFieldCache is the field holding the id of the cache adaptor
	// storing the responses, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.ICacheAdaptorFactory], a
	// non-empty string.
	ConfigFieldCache = "cache"

	// ConfigFieldHeader is the field holding the name of the request
	// header holding the idempotency key, a non-empty string.
	ConfigFieldHeader = "header"

	// ConfigFieldMethods is the field holding the list of the methods of
	// the requests made idempotent, matched exactly, a []string or a []any
	// of at least one non-empty string.
	ConfigFieldMethods = "methods"

	// ConfigFieldRequired is the field holding the bool telling whether
	// the requests of the methods must hold an idempotency key, the ones
	// without being served as they are otherwise.
	ConfigFieldRequired = "required"

	// ConfigFieldTTL is the field holding how long a response is stored,
	// a [time.Duration] or a string such as "24h", of more than 0.
	ConfigFieldTTL = "ttl"

	// ConfigFieldLockTTL is the field holding how long the marker of a
	// request in flight is stored, at most, a [time.Duration] or a string
	// such as "1m", of more than 0, which should be longer than the
	// handlers take.
	ConfigFieldLockTTL = "lock_ttl"

	// ConfigFieldMaxBodySize is the field holding the int maximum number
	// of bytes of the body of a response stored, 0 or more, a longer
	// response being sent without being stored.
	ConfigFieldMaxBodySize = "max_body_size"

	// ConfigFieldHeaders is the field holding the list of the names of
	// the response headers stored along with a response, and replayed, a
	// []string or a []any of non-empty strings.
	ConfigFieldHeaders = "headers"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultCacheConfigPath is the path of the default of
	// [ConfigFieldCache], unset unless set by the application.
	DefaultCacheConfigPath = "flam.rest.defaults.middlewares.idempotency.cache"

	// DefaultHeaderConfigPath is the path of the default of
	// [ConfigFieldHeader], "Idempotency-Key" unless changed.
	DefaultHeaderConfigPath = "flam.rest.defaults.middlewares.idempotency.header"

	// DefaultMethodsConfigPath is the path of the default of
	// [ConfigFieldMethods], "POST" and "PATCH" unless changed.
	DefaultMethodsConfigPath = "flam.rest.defaults.middlewares.idempotency.methods"

	// DefaultRequiredConfigPath is the path of the default of
	// [ConfigFieldRequired], false unless changed.
	DefaultRequiredConfigPath = "flam.rest.defaults.middlewares.idempotency.required"

	// DefaultTTLConfigPath is the path of the default of [ConfigFieldTTL],
	// "24h0m0s" unless changed.
	DefaultTTLConfigPath = "flam.rest.defaults.middlewares.idempotency.ttl"

	// DefaultLockTTLConfigPath is the path of the default of
	// [ConfigFieldLockTTL], "1m0s" unless changed.
	DefaultLockTTLConfigPath = "flam.rest.defaults.middlewares.idempotency.lock_ttl"

	// DefaultMaxBodySizeConfigPath is the path of the default of
	// [ConfigFieldMaxBodySize], 1048576, a MiB, unless changed.
	DefaultMaxBodySizeConfigPath = "flam.rest.defaults.middlewares.idempotency.max_body_size"

	// DefaultHeadersConfigPath is the path of the default of
	// [ConfigFieldHeaders], "Content-Type" and "Location" unless changed.
	DefaultHeadersConfigPath = "flam.rest.defaults.middlewares.idempotency.headers"
)

// MaxKeyLength is the maximum number of bytes of an idempotency key: a
// request holding a longer one is answered with a 400 envelope of
// [ErrInvalidKey].
const MaxKeyLength = 255

// ReplayedHeader is the header set to "true" on the responses replayed
// from the cache.
const ReplayedHeader = "Idempotent-Replayed"

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldCache holds the id of the cache adaptor of the middleware.
	ErrorFieldCache = "cache"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultCache       = ""
	defaultHeader      = "Idempotency-Key"
	defaultRequired    = false
	defaultTTL         = 24 * time.Hour
	defaultLockTTL     = time.Minute
	defaultMaxBodySize = 1 << 20
)

// defaultMethods returns the default of [ConfigFieldMethods], a new list
// each time, so that the configuration never shares it.
func defaultMethods() []string {
	return []string{"POST", "PATCH"}
}

// defaultHeaders returns the default of [ConfigFieldHeaders], a new list
// each time, so that the configuration never shares it.
func defaultHeaders() []string {
	return []string{"Content-Type", "Location"}
}
