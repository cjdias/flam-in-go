package requestid

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.requestid"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "requestid"

	// Driver is the driver name of the request id middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.requestid"
)

// Fields of a request id middleware configuration.
const (
	// ConfigFieldHeader is the field holding the name of the header the id
	// is read from, in the request, and set on, in the response, a
	// non-empty string.
	ConfigFieldHeader = "header"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultHeaderConfigPath is the path of the default of
	// [ConfigFieldHeader], "X-Request-Id" unless changed.
	DefaultHeaderConfigPath = "flam.rest.defaults.middlewares.requestid.header"
)

// MaxLength is the maximum number of bytes of a valid request id read from
// a request: a longer one is replaced by a generated one.
const MaxLength = 128

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultHeader = "X-Request-Id"
)
