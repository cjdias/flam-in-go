package source

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.source"

const (
	// Id is the id of the source configured by the bundle returned from
	// [NewBundle], at
	// [github.com/cjdias/flam-in-go/pkg/flam/config.SourceConfigPath].
	Id = "secrets"

	// Driver is the driver name of the secrets sources, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.config.sources.driver.secrets"
)

// Fields of a secrets source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldStore is the field holding the id of the store of the
	// secrets, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.ISecretStoreFactory].
	ConfigFieldStore = "store"

	// ConfigFieldValues is the field holding the bag mapping configuration
	// paths to the keys of their secrets.
	ConfigFieldValues = "values"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration, which describe the source of id [Id].
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.secrets.source.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], false unless changed.
	DefaultReloadConfigPath = "flam.secrets.source.reload"

	// DefaultStoreConfigPath is the path of the default of
	// [ConfigFieldStore], empty unless changed.
	DefaultStoreConfigPath = "flam.secrets.source.store"

	// DefaultValuesConfigPath is the path of the default of
	// [ConfigFieldValues], no value unless changed.
	DefaultValuesConfigPath = "flam.secrets.source.values"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldPath holds the configuration path of a value.
	ErrorFieldPath = "path"

	// ErrorFieldKey holds the key of the secret of a value, never the
	// secret itself.
	ErrorFieldKey = "key"

	// ErrorFieldStore holds the id of the store of the source.
	ErrorFieldStore = "store"
)

// Defaults of the fields of a source configuration.
const (
	defaultPriority = 0
	defaultReload   = false
	defaultStore    = ""
)
