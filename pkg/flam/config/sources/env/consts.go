package env

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.sources.env"

// Driver is the driver name of the env sources, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.config.sources.driver.env"

// Fields of an env source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldFiles is the field holding the list of the dotenv files
	// read by the source.
	ConfigFieldFiles = "files"

	// ConfigFieldMappings is the field holding the bag mapping environment
	// variable names to the configuration paths of their values.
	ConfigFieldMappings = "mappings"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration.
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.config.defaults.sources.env.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], true unless changed.
	DefaultReloadConfigPath = "flam.config.defaults.sources.env.reload"

	// DefaultFilesConfigPath is the path of the default of
	// [ConfigFieldFiles], no file unless changed.
	DefaultFilesConfigPath = "flam.config.defaults.sources.env.files"

	// DefaultMappingsConfigPath is the path of the default of
	// [ConfigFieldMappings], no mapping unless changed.
	DefaultMappingsConfigPath = "flam.config.defaults.sources.env.mappings"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds the environment variable name of a mapping.
	ErrorFieldKey = "key"

	// ErrorFieldPath holds the configuration path of a mapping.
	ErrorFieldPath = "path"

	// ErrorFieldOtherKey holds the environment variable name of the
	// mapping conflicting with the one of ErrorFieldKey.
	ErrorFieldOtherKey = "other_key"

	// ErrorFieldOtherPath holds the configuration path of the mapping
	// conflicting with the one of ErrorFieldPath.
	ErrorFieldOtherPath = "other_path"

	// ErrorFieldFiles holds the dotenv files of the source.
	ErrorFieldFiles = "files"
)

// Defaults of the fields of a source configuration.
const (
	defaultPriority = 0
	defaultReload   = true
)
