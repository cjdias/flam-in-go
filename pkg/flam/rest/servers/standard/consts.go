package standard

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.servers.standard"

// Driver is the driver name of the standard servers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.rest.servers.driver.standard"

// Fields of a standard server configuration.
const (
	// ConfigFieldAddress is the field holding the TCP address the server
	// listens on, such as ":8080" or "127.0.0.1:0".
	ConfigFieldAddress = "address"

	// ConfigFieldGroup is the field holding the group of the endpoint
	// registers of the server, the server id unless set.
	ConfigFieldGroup = "group"

	// ConfigFieldLogger is the field holding the id of the logger adaptor
	// of the server, in the logger adaptor factory, an empty id leaving
	// the server without logger adaptor.
	ConfigFieldLogger = "logger"

	// ConfigFieldMiddlewares is the field holding the list of the ids of
	// the middlewares of the server, in the middleware factory, the first
	// one being the outermost.
	ConfigFieldMiddlewares = "middlewares"

	// ConfigFieldCertFile is the field holding the path of the PEM file of
	// the TLS certificate of the server, serving HTTPS when set along with
	// [ConfigFieldKeyFile].
	ConfigFieldCertFile = "cert_file"

	// ConfigFieldKeyFile is the field holding the path of the PEM file of
	// the private key of the TLS certificate of the server.
	ConfigFieldKeyFile = "key_file"

	// ConfigFieldReadHeaderTimeout is the field holding the time allowed
	// to read the headers of a request, a duration such as "10s", 0
	// allowing any time.
	ConfigFieldReadHeaderTimeout = "read_header_timeout"

	// ConfigFieldReadTimeout is the field holding the time allowed to read
	// a whole request, its body included, a duration such as "30s", 0
	// allowing any time.
	ConfigFieldReadTimeout = "read_timeout"

	// ConfigFieldWriteTimeout is the field holding the time allowed to
	// write a response, from the end of the reading of its request, a
	// duration such as "30s", 0 allowing any time, as for a streamed
	// response.
	ConfigFieldWriteTimeout = "write_timeout"

	// ConfigFieldIdleTimeout is the field holding the time a kept alive
	// connection waits for its next request, a duration such as "2m", 0
	// waiting as long as the read timeout, or else the read header
	// timeout.
	ConfigFieldIdleTimeout = "idle_timeout"

	// ConfigFieldMaxHeaderBytes is the field holding the maximum size of
	// the headers of a request, in bytes, a positive whole number.
	ConfigFieldMaxHeaderBytes = "max_header_bytes"

	// ConfigFieldShutdownTimeout is the field holding the time allowed to
	// the requests under way to end when the server shuts down, a duration
	// such as "10s", after which their connections are closed, 0 closing
	// them at once.
	ConfigFieldShutdownTimeout = "shutdown_timeout"
)

// Configuration paths of the values taken by the fields missing from a
// server configuration.
const (
	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], ":8080" unless set.
	DefaultAddressConfigPath = "flam.rest.defaults.servers.standard.address"

	// DefaultReadHeaderTimeoutConfigPath is the path of the default of
	// [ConfigFieldReadHeaderTimeout], "10s" unless set.
	DefaultReadHeaderTimeoutConfigPath = "flam.rest.defaults.servers.standard.read_header_timeout"

	// DefaultReadTimeoutConfigPath is the path of the default of
	// [ConfigFieldReadTimeout], "30s" unless set.
	DefaultReadTimeoutConfigPath = "flam.rest.defaults.servers.standard.read_timeout"

	// DefaultWriteTimeoutConfigPath is the path of the default of
	// [ConfigFieldWriteTimeout], "0s" unless set.
	DefaultWriteTimeoutConfigPath = "flam.rest.defaults.servers.standard.write_timeout"

	// DefaultIdleTimeoutConfigPath is the path of the default of
	// [ConfigFieldIdleTimeout], "2m0s" unless set.
	DefaultIdleTimeoutConfigPath = "flam.rest.defaults.servers.standard.idle_timeout"

	// DefaultMaxHeaderBytesConfigPath is the path of the default of
	// [ConfigFieldMaxHeaderBytes], 1048576 unless set.
	DefaultMaxHeaderBytesConfigPath = "flam.rest.defaults.servers.standard.max_header_bytes"

	// DefaultShutdownTimeoutConfigPath is the path of the default of
	// [ConfigFieldShutdownTimeout], "10s" unless set.
	DefaultShutdownTimeoutConfigPath = "flam.rest.defaults.servers.standard.shutdown_timeout"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the servers.
const (
	// ErrorFieldID holds the id of the server.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldLogger holds the id of the logger adaptor of the server.
	ErrorFieldLogger = "logger"

	// ErrorFieldMiddleware holds the id of a middleware of the server.
	ErrorFieldMiddleware = "middleware"

	// ErrorFieldAddress holds the address of the server.
	ErrorFieldAddress = "address"
)

// Defaults of the fields of a server configuration.
const (
	defaultAddress           = ":8080"
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultWriteTimeout      = time.Duration(0)
	defaultIdleTimeout       = 2 * time.Minute
	defaultMaxHeaderBytes    = 1 << 20
	defaultShutdownTimeout   = 10 * time.Second
)
