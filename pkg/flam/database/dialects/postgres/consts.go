package postgres

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database.dialects.postgres"

// Driver is the driver name of the PostgreSQL dialects, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.database.dialects.driver.postgres"

// Fields of a PostgreSQL dialect configuration.
const (
	// ConfigFieldHost is the field holding the host of the server.
	ConfigFieldHost = "host"

	// ConfigFieldPort is the field holding the port of the server, a whole
	// number or a string holding one.
	ConfigFieldPort = "port"

	// ConfigFieldDatabase is the field holding the name of the database.
	ConfigFieldDatabase = "database"

	// ConfigFieldUsername is the field holding the username.
	ConfigFieldUsername = "username"

	// ConfigFieldPassword is the field holding the password, which may be
	// empty.
	ConfigFieldPassword = "password"

	// ConfigFieldParams is the field holding the parameters of the
	// connection string, a bag of keywords and values, such as "sslmode".
	ConfigFieldParams = "params"
)

// Configuration paths of the values taken by the fields missing from a
// dialect configuration.
const (
	// DefaultHostConfigPath is the path of the default of
	// [ConfigFieldHost], "127.0.0.1" unless changed.
	DefaultHostConfigPath = "flam.database.defaults.dialects.postgres.host"

	// DefaultPortConfigPath is the path of the default of
	// [ConfigFieldPort], 5432 unless changed.
	DefaultPortConfigPath = "flam.database.defaults.dialects.postgres.port"

	// DefaultDatabaseConfigPath is the path of the default of
	// [ConfigFieldDatabase], "flam" unless changed.
	DefaultDatabaseConfigPath = "flam.database.defaults.dialects.postgres.database"

	// DefaultUsernameConfigPath is the path of the default of
	// [ConfigFieldUsername], "postgres" unless changed.
	DefaultUsernameConfigPath = "flam.database.defaults.dialects.postgres.username"

	// DefaultPasswordConfigPath is the path of the default of
	// [ConfigFieldPassword], empty unless changed.
	DefaultPasswordConfigPath = "flam.database.defaults.dialects.postgres.password" //nolint:gosec // a configuration path, not a credential

	// DefaultParamsConfigPath is the path of the default of
	// [ConfigFieldParams], none unless changed.
	DefaultParamsConfigPath = "flam.database.defaults.dialects.postgres.params"
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
	defaultHost     = "127.0.0.1"
	defaultPort     = 5432
	defaultDatabase = "flam"
	defaultUsername = "postgres"
	defaultPassword = ""
)
