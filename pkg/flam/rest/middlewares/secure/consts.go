package secure

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.secure"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "secure"

	// Driver is the driver name of the secure middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.secure"
)

// Fields of a secure middleware configuration. The header fields hold
// strings, an empty one leaving its header out, holding no carriage
// return, line feed or NUL.
const (
	// ConfigFieldContentTypeOptions is the field holding the value of the
	// X-Content-Type-Options header.
	ConfigFieldContentTypeOptions = "content_type_options"

	// ConfigFieldFrameOptions is the field holding the value of the
	// X-Frame-Options header.
	ConfigFieldFrameOptions = "frame_options"

	// ConfigFieldReferrerPolicy is the field holding the value of the
	// Referrer-Policy header.
	ConfigFieldReferrerPolicy = "referrer_policy"

	// ConfigFieldHSTSMaxAge is the field holding the max-age of the
	// Strict-Transport-Security header, a [time.Duration] or a string such
	// as "8760h", of 0 or more, sent in whole seconds, 0 leaving the
	// header out.
	ConfigFieldHSTSMaxAge = "hsts_max_age"

	// ConfigFieldHSTSIncludeSubdomains is the field holding the bool
	// telling whether the Strict-Transport-Security header holds the
	// includeSubDomains directive.
	ConfigFieldHSTSIncludeSubdomains = "hsts_include_subdomains"

	// ConfigFieldHSTSPreload is the field holding the bool telling whether
	// the Strict-Transport-Security header holds the preload directive.
	ConfigFieldHSTSPreload = "hsts_preload"

	// ConfigFieldForceHSTS is the field holding the bool telling whether
	// the Strict-Transport-Security header is set on the responses to the
	// requests received without TLS too, such as behind a proxy ending
	// the TLS connections.
	ConfigFieldForceHSTS = "force_hsts"

	// ConfigFieldContentSecurityPolicy is the field holding the value of
	// the Content-Security-Policy header.
	ConfigFieldContentSecurityPolicy = "content_security_policy"

	// ConfigFieldPermissionsPolicy is the field holding the value of the
	// Permissions-Policy header.
	ConfigFieldPermissionsPolicy = "permissions_policy"

	// ConfigFieldCrossOriginOpenerPolicy is the field holding the value of
	// the Cross-Origin-Opener-Policy header.
	ConfigFieldCrossOriginOpenerPolicy = "cross_origin_opener_policy"

	// ConfigFieldCrossOriginEmbedderPolicy is the field holding the value
	// of the Cross-Origin-Embedder-Policy header.
	ConfigFieldCrossOriginEmbedderPolicy = "cross_origin_embedder_policy"

	// ConfigFieldCrossOriginResourcePolicy is the field holding the value
	// of the Cross-Origin-Resource-Policy header.
	ConfigFieldCrossOriginResourcePolicy = "cross_origin_resource_policy"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultContentTypeOptionsConfigPath is the path of the default of
	// [ConfigFieldContentTypeOptions], "nosniff" unless changed.
	DefaultContentTypeOptionsConfigPath = "flam.rest.defaults.middlewares.secure.content_type_options"

	// DefaultFrameOptionsConfigPath is the path of the default of
	// [ConfigFieldFrameOptions], "DENY" unless changed.
	DefaultFrameOptionsConfigPath = "flam.rest.defaults.middlewares.secure.frame_options"

	// DefaultReferrerPolicyConfigPath is the path of the default of
	// [ConfigFieldReferrerPolicy], "no-referrer" unless changed.
	DefaultReferrerPolicyConfigPath = "flam.rest.defaults.middlewares.secure.referrer_policy"

	// DefaultHSTSMaxAgeConfigPath is the path of the default of
	// [ConfigFieldHSTSMaxAge], "8760h0m0s", a year, unless changed.
	DefaultHSTSMaxAgeConfigPath = "flam.rest.defaults.middlewares.secure.hsts_max_age"

	// DefaultHSTSIncludeSubdomainsConfigPath is the path of the default of
	// [ConfigFieldHSTSIncludeSubdomains], true unless changed.
	DefaultHSTSIncludeSubdomainsConfigPath = "flam.rest.defaults.middlewares.secure.hsts_include_subdomains"

	// DefaultHSTSPreloadConfigPath is the path of the default of
	// [ConfigFieldHSTSPreload], false unless changed.
	DefaultHSTSPreloadConfigPath = "flam.rest.defaults.middlewares.secure.hsts_preload"

	// DefaultForceHSTSConfigPath is the path of the default of
	// [ConfigFieldForceHSTS], false unless changed.
	DefaultForceHSTSConfigPath = "flam.rest.defaults.middlewares.secure.force_hsts"

	// DefaultContentSecurityPolicyConfigPath is the path of the default of
	// [ConfigFieldContentSecurityPolicy], empty unless changed.
	DefaultContentSecurityPolicyConfigPath = "flam.rest.defaults.middlewares.secure.content_security_policy"

	// DefaultPermissionsPolicyConfigPath is the path of the default of
	// [ConfigFieldPermissionsPolicy], empty unless changed.
	DefaultPermissionsPolicyConfigPath = "flam.rest.defaults.middlewares.secure.permissions_policy"

	// DefaultCrossOriginOpenerPolicyConfigPath is the path of the default
	// of [ConfigFieldCrossOriginOpenerPolicy], empty unless changed.
	DefaultCrossOriginOpenerPolicyConfigPath = "flam.rest.defaults.middlewares.secure.cross_origin_opener_policy"

	// DefaultCrossOriginEmbedderPolicyConfigPath is the path of the
	// default of [ConfigFieldCrossOriginEmbedderPolicy], empty unless
	// changed.
	DefaultCrossOriginEmbedderPolicyConfigPath = "flam.rest.defaults.middlewares.secure.cross_origin_embedder_policy"

	// DefaultCrossOriginResourcePolicyConfigPath is the path of the
	// default of [ConfigFieldCrossOriginResourcePolicy], empty unless
	// changed.
	DefaultCrossOriginResourcePolicyConfigPath = "flam.rest.defaults.middlewares.secure.cross_origin_resource_policy"
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
	defaultContentTypeOptions        = "nosniff"
	defaultFrameOptions              = "DENY"
	defaultReferrerPolicy            = "no-referrer"
	defaultHSTSMaxAge                = 365 * 24 * time.Hour
	defaultHSTSIncludeSubdomains     = true
	defaultHSTSPreload               = false
	defaultForceHSTS                 = false
	defaultContentSecurityPolicy     = ""
	defaultPermissionsPolicy         = ""
	defaultCrossOriginOpenerPolicy   = ""
	defaultCrossOriginEmbedderPolicy = ""
	defaultCrossOriginResourcePolicy = ""
)
