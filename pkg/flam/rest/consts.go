package rest

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest"

// Configuration paths and container groups of the servers, the logger
// adaptors, the middlewares and the endpoints.
const (
	// ServerConfigPath is the configuration path holding one server
	// configuration per server id, as read by the server factory.
	ServerConfigPath = "flam.rest.servers"

	// ServerCreatorGroup is the container group the server driver bundles
	// provide their [flam.IRestServerCreator] in, collected by the server
	// factory.
	ServerCreatorGroup = "flam.rest.servers.creator"

	// LoggerConfigPath is the configuration path holding one logger
	// adaptor configuration per adaptor id, as read by the logger adaptor
	// factory.
	LoggerConfigPath = "flam.rest.loggers"

	// LoggerCreatorGroup is the container group the logger adaptor driver
	// bundles provide their [flam.IRestLoggerAdaptorCreator] in, collected
	// by the logger adaptor factory.
	LoggerCreatorGroup = "flam.rest.loggers.creator"

	// MiddlewareConfigPath is the configuration path holding one
	// middleware configuration per middleware id, as read by the
	// middleware factory.
	MiddlewareConfigPath = "flam.rest.middlewares"

	// MiddlewareCreatorGroup is the container group the middleware driver
	// bundles provide their [flam.IRestMiddlewareCreator] in, collected by
	// the middleware factory.
	MiddlewareCreatorGroup = "flam.rest.middlewares.creator"

	// EndpointGroup is the container group the application provides its
	// [flam.IRestEndpointRegister] values in, registering their endpoints
	// on the servers of their group when created.
	EndpointGroup = "flam.rest.endpoints"

	// ServiceIdConfigPath is the configuration path of the id of the
	// service, a whole number set as the ServiceId of the endpoints, 0
	// unless set.
	ServiceIdConfigPath = "flam.rest.service_id"
)

// Configuration paths of the values taken by the fields missing from a
// server configuration, whatever its driver.
const (
	// DefaultLoggerConfigPath is the path of the default id of the logger
	// adaptor of the servers, set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/rest/loggers/standard] to its
	// adaptor, and unset otherwise: the servers then have no logger
	// adaptor.
	DefaultLoggerConfigPath = "flam.rest.defaults.servers.logger"

	// DefaultMiddlewaresConfigPath is the path of the default list of the
	// ids of the middlewares of the servers, unset unless set by the
	// application: the servers then have no middleware.
	DefaultMiddlewaresConfigPath = "flam.rest.defaults.servers.middlewares"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of a server or of a middleware.
	ErrorFieldID = "id"

	// ErrorFieldEndpoint holds the id of an endpoint.
	ErrorFieldEndpoint = "endpoint"

	// ErrorFieldPattern holds the pattern of an endpoint.
	ErrorFieldPattern = "pattern"

	// ErrorFieldField holds the name of an invalid field, of an endpoint
	// or of the configuration.
	ErrorFieldField = "field"

	// ErrorFieldSource holds the part of a request failing to be decoded:
	// [SourcePath], [SourceQuery] or [SourceBody].
	ErrorFieldSource = "source"
)

// Parts of a request, held by [ErrorFieldSource].
const (
	// SourcePath is the path of a request, holding its path values.
	SourcePath = "path"

	// SourceQuery is the query of the URL of a request.
	SourceQuery = "query"

	// SourceBody is the body of a request.
	SourceBody = "body"
)

// contentTypeJSON is the content type of the responses written by
// [WriteEnvelope].
const contentTypeJSON = "application/json"
