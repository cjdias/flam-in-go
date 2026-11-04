package env

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.stores.env"

const (
	// Id is the id of the default store configured by the bundle returned
	// from [NewBundle].
	Id = "env"

	// Driver is the driver name of the env stores, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.secrets.stores.driver.env"
)

// Fields of an env store configuration.
const (
	// ConfigFieldPrefix is the field holding the string prefixed to the
	// names of the variables, such as "APP_", empty for none.
	ConfigFieldPrefix = "prefix"
)

// Configuration paths of the values taken by the fields missing from a
// store configuration.
const (
	// DefaultPrefixConfigPath is the path of the default of
	// [ConfigFieldPrefix], empty unless changed.
	DefaultPrefixConfigPath = "flam.secrets.defaults.stores.env.prefix"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the stores, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets].
const (
	// ErrorFieldID holds the id of the store.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds the key of a secret, or the path given to
	// GetMap.
	ErrorFieldKey = "key"

	// ErrorFieldVariable holds the name of the variable of a key, or the
	// prefix of the variables of a path.
	ErrorFieldVariable = "variable"
)

// Defaults of the fields of a store configuration.
const (
	defaultPrefix = ""
)
