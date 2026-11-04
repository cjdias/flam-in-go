package grpc

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc"

// Configuration paths and container groups of the servers, the logger
// adaptors, the interceptors and the services.
const (
	// ServerConfigPath is the configuration path holding one server
	// configuration per server id, as read by the server factory.
	ServerConfigPath = "flam.grpc.servers"

	// ServerCreatorGroup is the container group the server driver bundles
	// provide their [flam.IGrpcServerCreator] in, collected by the server
	// factory.
	ServerCreatorGroup = "flam.grpc.servers.creator"

	// LoggerConfigPath is the configuration path holding one logger
	// adaptor configuration per adaptor id, as read by the logger adaptor
	// factory.
	LoggerConfigPath = "flam.grpc.loggers"

	// LoggerCreatorGroup is the container group the logger adaptor driver
	// bundles provide their [flam.IGrpcLoggerAdaptorCreator] in, collected
	// by the logger adaptor factory.
	LoggerCreatorGroup = "flam.grpc.loggers.creator"

	// InterceptorConfigPath is the configuration path holding one
	// interceptor configuration per interceptor id, as read by the
	// interceptor factory.
	InterceptorConfigPath = "flam.grpc.interceptors"

	// InterceptorCreatorGroup is the container group the interceptor
	// driver bundles provide their [flam.IGrpcInterceptorCreator] in,
	// collected by the interceptor factory.
	InterceptorCreatorGroup = "flam.grpc.interceptors.creator"

	// ServiceGroup is the container group the application provides its
	// [flam.IGrpcServiceRegister] values in, registering their services on
	// the servers of their group when created.
	ServiceGroup = "flam.grpc.services"
)

// Configuration paths of the values taken by the fields missing from a
// server configuration, whatever its driver.
const (
	// DefaultLoggerConfigPath is the path of the default id of the logger
	// adaptor of the servers, set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/grpc/loggers/standard] to its
	// adaptor, and unset otherwise: the servers then have no logger
	// adaptor.
	DefaultLoggerConfigPath = "flam.grpc.defaults.servers.logger"

	// DefaultInterceptorsConfigPath is the path of the default list of the
	// ids of the interceptors of the servers, unset unless set by the
	// application: the servers then have no interceptor.
	DefaultInterceptorsConfigPath = "flam.grpc.defaults.servers.interceptors"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of a server.
	ErrorFieldID = "id"
)
