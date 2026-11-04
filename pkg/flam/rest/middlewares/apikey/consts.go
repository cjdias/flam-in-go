package apikey

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.apikey"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "apikey"

	// Driver is the driver name of the apikey middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.apikey"
)

// Fields of an apikey middleware configuration.
const (
	// ConfigFieldHeader is the field holding the name of the header of the
	// keys, a string, empty for no header.
	ConfigFieldHeader = "header"

	// ConfigFieldQuery is the field holding the name of the query
	// parameter of the keys, a string, empty for no query parameter. The
	// header and the query parameter cannot be both empty.
	ConfigFieldQuery = "query"

	// ConfigFieldKeys is the field holding the keys accepted, at least
	// one, all non-empty and distinct: a list of keys, a []string or a
	// []any of strings, setting no principal, or a map of the name of each
	// principal to its key, or to the list of its keys.
	ConfigFieldKeys = "keys"

	// ConfigFieldOptional is the field holding the bool telling whether a
	// request without a key is served, without principal.
	ConfigFieldOptional = "optional"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration. The keys have none, as they belong to each
// middleware.
const (
	// DefaultHeaderConfigPath is the path of the default of
	// [ConfigFieldHeader], "X-Api-Key" unless changed.
	DefaultHeaderConfigPath = "flam.rest.defaults.middlewares.apikey.header"

	// DefaultQueryConfigPath is the path of the default of
	// [ConfigFieldQuery], empty unless changed.
	DefaultQueryConfigPath = "flam.rest.defaults.middlewares.apikey.query"

	// DefaultOptionalConfigPath is the path of the default of
	// [ConfigFieldOptional], false unless changed.
	DefaultOptionalConfigPath = "flam.rest.defaults.middlewares.apikey.optional"
)

// Challenge is the value of the WWW-Authenticate header of the 401
// answers.
const Challenge = "ApiKey"

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultHeader   = "X-Api-Key"
	defaultQuery    = ""
	defaultOptional = false
)
