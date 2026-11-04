package etag

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.etag"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "etag"

	// Driver is the driver name of the etag middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.etag"
)

// Fields of an etag middleware configuration.
const (
	// ConfigFieldMaxSize is the field holding the int maximum number of
	// bytes of body buffered to compute its entity tag, 1 or more, a
	// longer response being sent as it is, without one.
	ConfigFieldMaxSize = "max_size"

	// ConfigFieldStrong is the field holding the bool telling whether the
	// entity tags computed are strong ones, rather than weak ones,
	// prefixed by "W/". A strong tag tells apart the responses whose
	// bodies differ byte for byte, such as a compressed one from the
	// uncompressed one: set it only when the middleware sees the body as
	// sent, listed before the compress middleware.
	ConfigFieldStrong = "strong"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultMaxSizeConfigPath is the path of the default of
	// [ConfigFieldMaxSize], 1048576, a MiB, unless changed.
	DefaultMaxSizeConfigPath = "flam.rest.defaults.middlewares.etag.max_size"

	// DefaultStrongConfigPath is the path of the default of
	// [ConfigFieldStrong], false unless changed.
	DefaultStrongConfigPath = "flam.rest.defaults.middlewares.etag.strong"
)

// TagLength is the number of bytes of the SHA-256 hash of a body kept in
// its entity tag, as hexadecimal digits.
const TagLength = 16

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultMaxSize = 1 << 20
	defaultStrong  = false
)
