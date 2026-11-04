package bodylimit

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.bodylimit"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "bodylimit"

	// Driver is the driver name of the body limit middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.bodylimit"
)

// Fields of a body limit middleware configuration.
const (
	// ConfigFieldMaxBytes is the field holding the int maximum number of
	// bytes of the body of a request, more than 0.
	ConfigFieldMaxBytes = "max_bytes"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultMaxBytesConfigPath is the path of the default of
	// [ConfigFieldMaxBytes], 1048576, 1 MiB, unless changed.
	DefaultMaxBytesConfigPath = "flam.rest.defaults.middlewares.bodylimit.max_bytes"
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
	defaultMaxBytes = 1 << 20
)
