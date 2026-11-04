package standard

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.redis.connections.standard"

// Driver is the driver name of the standard connections, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.redis.connections.driver.standard"

// Fields of a standard connection configuration, along with the fields
// shared by the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/redis].
const (
	// ConfigFieldNetwork is the field holding the network, "tcp" or
	// "unix".
	ConfigFieldNetwork = "network"

	// ConfigFieldHost is the field holding the host of the server, or the
	// path of its socket for the "unix" network.
	ConfigFieldHost = "host"

	// ConfigFieldPort is the field holding the port of the server, a whole
	// number or a string holding one, unused for the "unix" network.
	ConfigFieldPort = "port"
)

// Configuration paths of the values taken by the fields missing from a
// connection configuration.
const (
	// DefaultNetworkConfigPath is the path of the default of
	// [ConfigFieldNetwork], "tcp" unless changed.
	DefaultNetworkConfigPath = "flam.redis.defaults.connections.standard.network"

	// DefaultHostConfigPath is the path of the default of
	// [ConfigFieldHost], "127.0.0.1" unless changed.
	DefaultHostConfigPath = "flam.redis.defaults.connections.standard.host"

	// DefaultPortConfigPath is the path of the default of
	// [ConfigFieldPort], 6379 unless changed.
	DefaultPortConfigPath = "flam.redis.defaults.connections.standard.port"

	// DefaultUsernameConfigPath is the path of the default of the
	// [github.com/cjdias/flam-in-go/pkg/flam/redis.ConfigFieldUsername]
	// field, empty unless changed.
	DefaultUsernameConfigPath = "flam.redis.defaults.connections.standard.username"

	// DefaultPasswordConfigPath is the path of the default of the
	// [github.com/cjdias/flam-in-go/pkg/flam/redis.ConfigFieldPassword]
	// field, empty unless changed.
	DefaultPasswordConfigPath = "flam.redis.defaults.connections.standard.password" //nolint:gosec // a configuration path, not a credential

	// DefaultDatabaseConfigPath is the path of the default of the
	// [github.com/cjdias/flam-in-go/pkg/flam/redis.ConfigFieldDatabase]
	// field, 0 unless changed.
	DefaultDatabaseConfigPath = "flam.redis.defaults.connections.standard.database"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the connection.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a connection configuration.
const (
	defaultNetwork  = "tcp"
	defaultHost     = "127.0.0.1"
	defaultPort     = 6379
	defaultUsername = ""
	defaultPassword = ""
	defaultDatabase = 0
)
