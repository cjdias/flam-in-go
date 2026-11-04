package sqlite

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database.dialects.sqlite"

// Driver is the driver name of the SQLite dialects, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.database.dialects.driver.sqlite"

// Fields of a SQLite dialect configuration.
const (
	// ConfigFieldPath is the field holding the path of the database file,
	// or ":memory:".
	ConfigFieldPath = "path"

	// ConfigFieldParams is the field holding the parameters of the
	// connection string, a bag of names and values.
	ConfigFieldParams = "params"
)

// Configuration paths of the values taken by the fields missing from a
// dialect configuration.
const (
	// DefaultPathConfigPath is the path of the default of
	// [ConfigFieldPath], ":memory:" unless changed.
	DefaultPathConfigPath = "flam.database.defaults.dialects.sqlite.path"

	// DefaultParamsConfigPath is the path of the default of
	// [ConfigFieldParams], none unless changed.
	DefaultParamsConfigPath = "flam.database.defaults.dialects.sqlite.params"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the dialect.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a dialect configuration.
const (
	defaultPath = ":memory:"
)
