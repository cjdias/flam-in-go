package cors

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.cors"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "cors"

	// Driver is the driver name of the CORS middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.cors"
)

// Fields of a CORS middleware configuration.
const (
	// ConfigFieldAllowedOrigins is the field holding the list of the
	// origins allowed to call the endpoints, such as
	// "https://example.com", matched whatever their case, "*" allowing any
	// origin, a []string or a []any of non-empty strings.
	ConfigFieldAllowedOrigins = "allowed_origins"

	// ConfigFieldAllowedMethods is the field holding the list of the
	// methods the preflight requests may ask for, such as "PUT", matched
	// exactly, a []string or a []any of non-empty strings.
	ConfigFieldAllowedMethods = "allowed_methods"

	// ConfigFieldAllowedHeaders is the field holding the list of the
	// request headers the preflight requests may ask for, matched whatever
	// their case, "*" allowing any header, a []string or a []any of
	// non-empty strings.
	ConfigFieldAllowedHeaders = "allowed_headers"

	// ConfigFieldExposedHeaders is the field holding the list of the
	// response headers the browsers let the calling pages read, besides
	// the safelisted ones, "*" exposing every header to the requests
	// without credentials, a []string or a []any of non-empty strings.
	ConfigFieldExposedHeaders = "exposed_headers"

	// ConfigFieldAllowCredentials is the field holding the bool telling
	// whether the browsers may send the credentials, cookies and
	// authorization headers, along with the requests, and let the calling
	// pages read the responses to them.
	ConfigFieldAllowCredentials = "allow_credentials"

	// ConfigFieldMaxAge is the field holding how long the browsers may
	// cache the answer to a preflight request, a [time.Duration] or a
	// string such as "10m", of 0 or more, sent in whole seconds, 0 leaving
	// the Access-Control-Max-Age header out.
	ConfigFieldMaxAge = "max_age"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultAllowedOriginsConfigPath is the path of the default of
	// [ConfigFieldAllowedOrigins], "*" unless changed.
	DefaultAllowedOriginsConfigPath = "flam.rest.defaults.middlewares.cors.allowed_origins"

	// DefaultAllowedMethodsConfigPath is the path of the default of
	// [ConfigFieldAllowedMethods], "GET", "HEAD", "POST", "PUT", "PATCH"
	// and "DELETE" unless changed.
	DefaultAllowedMethodsConfigPath = "flam.rest.defaults.middlewares.cors.allowed_methods"

	// DefaultAllowedHeadersConfigPath is the path of the default of
	// [ConfigFieldAllowedHeaders], "Accept", "Authorization",
	// "Content-Type" and "X-Request-Id" unless changed.
	DefaultAllowedHeadersConfigPath = "flam.rest.defaults.middlewares.cors.allowed_headers"

	// DefaultExposedHeadersConfigPath is the path of the default of
	// [ConfigFieldExposedHeaders], "X-Request-Id" unless changed.
	DefaultExposedHeadersConfigPath = "flam.rest.defaults.middlewares.cors.exposed_headers"

	// DefaultAllowCredentialsConfigPath is the path of the default of
	// [ConfigFieldAllowCredentials], false unless changed.
	DefaultAllowCredentialsConfigPath = "flam.rest.defaults.middlewares.cors.allow_credentials"

	// DefaultMaxAgeConfigPath is the path of the default of
	// [ConfigFieldMaxAge], "10m" unless changed.
	DefaultMaxAgeConfigPath = "flam.rest.defaults.middlewares.cors.max_age"
)

// Wildcard is the value of [ConfigFieldAllowedOrigins],
// [ConfigFieldAllowedHeaders] and [ConfigFieldExposedHeaders] standing for
// any origin or header.
const Wildcard = "*"

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultAllowCredentials = false
	defaultMaxAge           = 10 * time.Minute
)

// defaultAllowedOrigins returns the default of
// [ConfigFieldAllowedOrigins], a new list each time, so that the
// configuration never shares it.
func defaultAllowedOrigins() []string {
	return []string{Wildcard}
}

// defaultAllowedMethods returns the default of
// [ConfigFieldAllowedMethods], a new list each time, so that the
// configuration never shares it.
func defaultAllowedMethods() []string {
	return []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"}
}

// defaultAllowedHeaders returns the default of
// [ConfigFieldAllowedHeaders], a new list each time, so that the
// configuration never shares it.
func defaultAllowedHeaders() []string {
	return []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"}
}

// defaultExposedHeaders returns the default of
// [ConfigFieldExposedHeaders], a new list each time, so that the
// configuration never shares it.
func defaultExposedHeaders() []string {
	return []string{"X-Request-Id"}
}
