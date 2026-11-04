package mysql

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database.dialects.mysql"

// Driver is the driver name of the MySQL dialects, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.database.dialects.driver.mysql"

// Fields of a MySQL dialect configuration.
const (
	// ConfigFieldProtocol is the field holding the network protocol, such
	// as "tcp" or "unix".
	ConfigFieldProtocol = "protocol"

	// ConfigFieldHost is the field holding the host of the server, or the
	// path of its socket for the "unix" protocol.
	ConfigFieldHost = "host"

	// ConfigFieldPort is the field holding the port of the server, a whole
	// number or a string holding one, unused for the "unix" protocol.
	ConfigFieldPort = "port"

	// ConfigFieldSchema is the field holding the name of the schema.
	ConfigFieldSchema = "schema"

	// ConfigFieldUsername is the field holding the username.
	ConfigFieldUsername = "username"

	// ConfigFieldPassword is the field holding the password, which may be
	// empty.
	ConfigFieldPassword = "password"

	// ConfigFieldParams is the field holding the parameters of the
	// connection string, a bag of names and values, such as "charset".
	ConfigFieldParams = "params"
)

// Configuration paths of the values taken by the fields missing from a
// dialect configuration.
const (
	// DefaultProtocolConfigPath is the path of the default of
	// [ConfigFieldProtocol], "tcp" unless changed.
	DefaultProtocolConfigPath = "flam.database.defaults.dialects.mysql.protocol"

	// DefaultHostConfigPath is the path of the default of
	// [ConfigFieldHost], "127.0.0.1" unless changed.
	DefaultHostConfigPath = "flam.database.defaults.dialects.mysql.host"

	// DefaultPortConfigPath is the path of the default of
	// [ConfigFieldPort], 3306 unless changed.
	DefaultPortConfigPath = "flam.database.defaults.dialects.mysql.port"

	// DefaultSchemaConfigPath is the path of the default of
	// [ConfigFieldSchema], "flam" unless changed.
	DefaultSchemaConfigPath = "flam.database.defaults.dialects.mysql.schema"

	// DefaultUsernameConfigPath is the path of the default of
	// [ConfigFieldUsername], "root" unless changed.
	DefaultUsernameConfigPath = "flam.database.defaults.dialects.mysql.username"

	// DefaultPasswordConfigPath is the path of the default of
	// [ConfigFieldPassword], empty unless changed.
	DefaultPasswordConfigPath = "flam.database.defaults.dialects.mysql.password" //nolint:gosec // a configuration path, not a credential

	// DefaultParamsConfigPath is the path of the default of
	// [ConfigFieldParams], none unless changed.
	DefaultParamsConfigPath = "flam.database.defaults.dialects.mysql.params"
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
	defaultProtocol = "tcp"
	defaultHost     = "127.0.0.1"
	defaultPort     = 3306
	defaultSchema   = "flam"
	defaultUsername = "root"
	defaultPassword = ""
)
