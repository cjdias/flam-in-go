package mini

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.redis.connections.mini"

// Driver is the driver name of the mini connections, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.redis.connections.driver.mini"

// Fields of a mini connection configuration, along with the fields shared
// by the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/redis].
const (
	// ConfigFieldAddress is the field holding the TCP address the server
	// of the connection listens on, such as "127.0.0.1:0", which picks a
	// free port.
	ConfigFieldAddress = "address"
)

// Configuration paths of the values taken by the fields missing from a
// connection configuration.
const (
	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], "127.0.0.1:0" unless changed.
	DefaultAddressConfigPath = "flam.redis.defaults.connections.mini.address"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the connection.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldAddress holds the address the server failed to listen on.
	ErrorFieldAddress = "address"
)

// Defaults of the fields of a connection configuration.
const (
	defaultAddress = "127.0.0.1:0"
)
