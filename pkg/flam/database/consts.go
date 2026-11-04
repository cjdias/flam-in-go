package database

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database"

// Configuration paths and container groups of the configurations, dialects
// and connections.
const (
	// ConfigConfigPath is the configuration path holding one gorm
	// configuration per configuration id, as read by the configuration
	// factory.
	ConfigConfigPath = "flam.database.configs"

	// ConfigCreatorGroup is the container group the configuration driver
	// bundles provide their [flam.IDatabaseConfigCreator] in, collected by
	// the configuration factory.
	ConfigCreatorGroup = "flam.database.configs.creator"

	// DialectConfigPath is the configuration path holding one dialect
	// configuration per dialect id, as read by the dialect factory.
	DialectConfigPath = "flam.database.dialects"

	// DialectCreatorGroup is the container group the dialect driver bundles
	// provide their [flam.IDatabaseDialectCreator] in, collected by the
	// dialect factory.
	DialectCreatorGroup = "flam.database.dialects.creator"

	// ConnectionConfigPath is the configuration path holding one connection
	// configuration per connection id, as read by the connection factory.
	ConnectionConfigPath = "flam.database.connections"

	// ConnectionCreatorGroup is the container group holding the
	// [flam.IDatabaseConnectionCreator] values collected by the connection
	// factory, among which the one of the bundle returned by [NewBundle].
	ConnectionCreatorGroup = "flam.database.connections.creator"
)

// Driver is the driver name of the connections created by the creator of
// the bundle returned by [NewBundle], held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration, which may also be left out.
const Driver = "flam.database.connections.driver.gorm"

// Fields of a connection configuration.
const (
	// ConfigFieldDialect is the field holding the id of the dialect of the
	// connection, in the dialect factory.
	ConfigFieldDialect = "dialect"

	// ConfigFieldConfig is the field holding the id of the gorm
	// configuration of the connection, in the configuration factory.
	ConfigFieldConfig = "config"
)

// Configuration paths of the values taken by the fields missing from a
// connection configuration.
const (
	// DefaultDialectConfigPath is the path of the default of
	// [ConfigFieldDialect], unset unless configured.
	DefaultDialectConfigPath = "flam.database.defaults.connections.dialect"

	// DefaultConfigConfigPath is the path of the default of
	// [ConfigFieldConfig], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard] to
	// its configuration.
	DefaultConfigConfigPath = "flam.database.defaults.connections.config"
)

// Context fields of the [*flam.Error] values returned by the database
// package.
const (
	// ErrorFieldID holds the id of a connection or a dialect.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldDialect holds the id of the dialect of a connection.
	ErrorFieldDialect = "dialect"

	// ErrorFieldConfig holds the id of the gorm configuration of a
	// connection.
	ErrorFieldConfig = "config"
)
