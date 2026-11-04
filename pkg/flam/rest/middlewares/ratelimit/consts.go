package ratelimit

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.ratelimit"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "ratelimit"

	// Driver is the driver name of the ratelimit middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.ratelimit"
)

// Fields of a ratelimit middleware configuration.
const (
	// ConfigFieldRate is the field holding the number of tokens put back
	// in a bucket per second, a finite float64 or a whole number, over 0.
	ConfigFieldRate = "rate"

	// ConfigFieldBurst is the field holding the int number of tokens of a
	// full bucket, 1 or more.
	ConfigFieldBurst = "burst"

	// ConfigFieldMaxKeys is the field holding the int maximum number of
	// buckets held, 1 or more.
	ConfigFieldMaxKeys = "max_keys"

	// ConfigFieldTrustedProxyHeader is the field holding the name of the
	// header whose last IP is the one of the client, such as
	// "X-Forwarded-For", a string, empty for none.
	ConfigFieldTrustedProxyHeader = "trusted_proxy_header"

	// ConfigFieldIPv6Prefix is the field holding the int length, from 0 to
	// 128, of the prefix of an IPv6 address telling a client.
	ConfigFieldIPv6Prefix = "ipv6_prefix"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultRateConfigPath is the path of the default of
	// [ConfigFieldRate], 10 unless changed.
	DefaultRateConfigPath = "flam.rest.defaults.middlewares.ratelimit.rate"

	// DefaultBurstConfigPath is the path of the default of
	// [ConfigFieldBurst], 20 unless changed.
	DefaultBurstConfigPath = "flam.rest.defaults.middlewares.ratelimit.burst"

	// DefaultMaxKeysConfigPath is the path of the default of
	// [ConfigFieldMaxKeys], 10000 unless changed.
	DefaultMaxKeysConfigPath = "flam.rest.defaults.middlewares.ratelimit.max_keys"

	// DefaultTrustedProxyHeaderConfigPath is the path of the default of
	// [ConfigFieldTrustedProxyHeader], empty unless changed.
	DefaultTrustedProxyHeaderConfigPath = "flam.rest.defaults.middlewares.ratelimit.trusted_proxy_header"

	// DefaultIPv6PrefixConfigPath is the path of the default of
	// [ConfigFieldIPv6Prefix], 64 unless changed.
	DefaultIPv6PrefixConfigPath = "flam.rest.defaults.middlewares.ratelimit.ipv6_prefix"
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
	defaultRate               = 10.0
	defaultBurst              = 20
	defaultMaxKeys            = 10000
	defaultTrustedProxyHeader = ""
	defaultIPv6Prefix         = 64
)
