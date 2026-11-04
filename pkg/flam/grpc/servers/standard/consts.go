package standard

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc.servers.standard"

// Driver is the driver name of the standard servers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.grpc.servers.driver.standard"

// Fields of a standard server configuration.
const (
	// ConfigFieldAddress is the field holding the TCP address the server
	// listens on, such as ":9090" or "127.0.0.1:0".
	ConfigFieldAddress = "address"

	// ConfigFieldGroup is the field holding the group of the service
	// registers of the server, the server id unless set.
	ConfigFieldGroup = "group"

	// ConfigFieldLogger is the field holding the id of the logger adaptor
	// of the server, in the logger adaptor factory, an empty id leaving
	// the server without logger adaptor.
	ConfigFieldLogger = "logger"

	// ConfigFieldInterceptors is the field holding the list of the ids of
	// the interceptors of the server, in the interceptor factory, the
	// first one being the outermost.
	ConfigFieldInterceptors = "interceptors"

	// ConfigFieldCertFile is the field holding the path of the PEM file of
	// the TLS certificate of the server, serving over TLS when set along
	// with [ConfigFieldKeyFile].
	ConfigFieldCertFile = "cert_file"

	// ConfigFieldKeyFile is the field holding the path of the PEM file of
	// the private key of the TLS certificate of the server.
	ConfigFieldKeyFile = "key_file"

	// ConfigFieldConnectionTimeout is the field holding the time allowed
	// to a new connection to complete its handshakes, a duration such as
	// "2m", 0 for the gRPC default of 120 seconds.
	ConfigFieldConnectionTimeout = "connection_timeout"

	// ConfigFieldMaxRecvMsgSize is the field holding the maximum size of a
	// message received, in bytes, a positive whole number.
	ConfigFieldMaxRecvMsgSize = "max_recv_msg_size"

	// ConfigFieldMaxSendMsgSize is the field holding the maximum size of a
	// message sent, in bytes, a positive whole number.
	ConfigFieldMaxSendMsgSize = "max_send_msg_size"

	// ConfigFieldKeepaliveTime is the field holding the time without
	// activity after which the server pings a client to check its
	// connection, a duration such as "2h", 0 for the gRPC default of 2
	// hours.
	ConfigFieldKeepaliveTime = "keepalive_time"

	// ConfigFieldKeepaliveTimeout is the field holding the time the server
	// waits for the answer to a ping before closing the connection, a
	// duration such as "20s", 0 for the gRPC default of 20 seconds.
	ConfigFieldKeepaliveTimeout = "keepalive_timeout"

	// ConfigFieldKeepaliveMinTime is the field holding the minimum time a
	// client must wait between its pings, a duration such as "5m", the
	// connection of a client pinging more often being closed.
	ConfigFieldKeepaliveMinTime = "keepalive_min_time"

	// ConfigFieldKeepalivePermitWithoutStream is the field holding the
	// bool telling whether the clients may ping while they have no call
	// under way.
	ConfigFieldKeepalivePermitWithoutStream = "keepalive_permit_without_stream"

	// ConfigFieldMaxConnectionIdle is the field holding the time after
	// which a connection without call under way is closed, a duration such
	// as "15m", 0 never closing it.
	ConfigFieldMaxConnectionIdle = "max_connection_idle"

	// ConfigFieldMaxConnectionAge is the field holding the time after
	// which a connection is closed, so that the clients spread over the
	// servers again, a duration such as "30m", 0 never closing it.
	ConfigFieldMaxConnectionAge = "max_connection_age"

	// ConfigFieldMaxConnectionAgeGrace is the field holding the time
	// allowed to the calls under way on a connection closed by its age to
	// end, a duration such as "1m", 0 allowing any time.
	ConfigFieldMaxConnectionAgeGrace = "max_connection_age_grace"

	// ConfigFieldReflection is the field holding the bool telling whether
	// the server registers the gRPC reflection service, describing its
	// services to the clients such as grpcurl.
	ConfigFieldReflection = "reflection"

	// ConfigFieldHealth is the field holding the bool telling whether the
	// server registers the standard gRPC health service, of
	// [google.golang.org/grpc/health/grpc_health_v1].
	ConfigFieldHealth = "health"

	// ConfigFieldHealthInterval is the field holding the time between the
	// runs of the readiness probe of the
	// [github.com/cjdias/flam-in-go/pkg/flam.IHealthRegistry] setting the
	// status of the health service, a positive duration such as "5s".
	ConfigFieldHealthInterval = "health_interval"

	// ConfigFieldShutdownTimeout is the field holding the time allowed to
	// the calls under way to end when the server stops, a duration such as
	// "10s", after which their connections are closed, 0 closing them at
	// once.
	ConfigFieldShutdownTimeout = "shutdown_timeout"
)

// Configuration paths of the values taken by the fields missing from a
// server configuration.
const (
	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], ":9090" unless set.
	DefaultAddressConfigPath = "flam.grpc.defaults.servers.standard.address"

	// DefaultConnectionTimeoutConfigPath is the path of the default of
	// [ConfigFieldConnectionTimeout], "2m0s" unless set.
	DefaultConnectionTimeoutConfigPath = "flam.grpc.defaults.servers.standard.connection_timeout"

	// DefaultMaxRecvMsgSizeConfigPath is the path of the default of
	// [ConfigFieldMaxRecvMsgSize], 4194304 unless set.
	DefaultMaxRecvMsgSizeConfigPath = "flam.grpc.defaults.servers.standard.max_recv_msg_size"

	// DefaultMaxSendMsgSizeConfigPath is the path of the default of
	// [ConfigFieldMaxSendMsgSize], 4194304 unless set.
	DefaultMaxSendMsgSizeConfigPath = "flam.grpc.defaults.servers.standard.max_send_msg_size"

	// DefaultKeepaliveTimeConfigPath is the path of the default of
	// [ConfigFieldKeepaliveTime], "2h0m0s" unless set.
	DefaultKeepaliveTimeConfigPath = "flam.grpc.defaults.servers.standard.keepalive_time"

	// DefaultKeepaliveTimeoutConfigPath is the path of the default of
	// [ConfigFieldKeepaliveTimeout], "20s" unless set.
	DefaultKeepaliveTimeoutConfigPath = "flam.grpc.defaults.servers.standard.keepalive_timeout"

	// DefaultKeepaliveMinTimeConfigPath is the path of the default of
	// [ConfigFieldKeepaliveMinTime], "5m0s" unless set.
	DefaultKeepaliveMinTimeConfigPath = "flam.grpc.defaults.servers.standard.keepalive_min_time"

	// DefaultKeepalivePermitWithoutStreamConfigPath is the path of the
	// default of [ConfigFieldKeepalivePermitWithoutStream], false unless
	// set.
	DefaultKeepalivePermitWithoutStreamConfigPath = "flam.grpc.defaults.servers.standard.keepalive_permit_without_stream"

	// DefaultMaxConnectionIdleConfigPath is the path of the default of
	// [ConfigFieldMaxConnectionIdle], "0s" unless set.
	DefaultMaxConnectionIdleConfigPath = "flam.grpc.defaults.servers.standard.max_connection_idle"

	// DefaultMaxConnectionAgeConfigPath is the path of the default of
	// [ConfigFieldMaxConnectionAge], "0s" unless set.
	DefaultMaxConnectionAgeConfigPath = "flam.grpc.defaults.servers.standard.max_connection_age"

	// DefaultMaxConnectionAgeGraceConfigPath is the path of the default of
	// [ConfigFieldMaxConnectionAgeGrace], "0s" unless set.
	DefaultMaxConnectionAgeGraceConfigPath = "flam.grpc.defaults.servers.standard.max_connection_age_grace"

	// DefaultReflectionConfigPath is the path of the default of
	// [ConfigFieldReflection], false unless set.
	DefaultReflectionConfigPath = "flam.grpc.defaults.servers.standard.reflection"

	// DefaultHealthConfigPath is the path of the default of
	// [ConfigFieldHealth], true unless set.
	DefaultHealthConfigPath = "flam.grpc.defaults.servers.standard.health"

	// DefaultHealthIntervalConfigPath is the path of the default of
	// [ConfigFieldHealthInterval], "5s" unless set.
	DefaultHealthIntervalConfigPath = "flam.grpc.defaults.servers.standard.health_interval"

	// DefaultShutdownTimeoutConfigPath is the path of the default of
	// [ConfigFieldShutdownTimeout], "10s" unless set.
	DefaultShutdownTimeoutConfigPath = "flam.grpc.defaults.servers.standard.shutdown_timeout"
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

	// ErrorFieldInterceptor holds the id of an interceptor of the server.
	ErrorFieldInterceptor = "interceptor"

	// ErrorFieldAddress holds the address of the server.
	ErrorFieldAddress = "address"

	// ErrorFieldService holds the name of a service registered on the
	// server.
	ErrorFieldService = "service"
)

// Defaults of the fields of a server configuration.
const (
	defaultAddress                      = ":9090"
	defaultConnectionTimeout            = 2 * time.Minute
	defaultMaxRecvMsgSize               = 4 << 20
	defaultMaxSendMsgSize               = 4 << 20
	defaultKeepaliveTime                = 2 * time.Hour
	defaultKeepaliveTimeout             = 20 * time.Second
	defaultKeepaliveMinTime             = 5 * time.Minute
	defaultKeepalivePermitWithoutStream = false
	defaultMaxConnectionIdle            = time.Duration(0)
	defaultMaxConnectionAge             = time.Duration(0)
	defaultMaxConnectionAgeGrace        = time.Duration(0)
	defaultReflection                   = false
	defaultHealth                       = true
	defaultHealthInterval               = 5 * time.Second
	defaultShutdownTimeout              = 10 * time.Second
)
