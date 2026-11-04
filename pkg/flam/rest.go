package flam

import (
	"net/http"
)

// RestEndpoint describes an endpoint of a REST server: the ids identifying
// it in the errors it answers with, the pattern of the requests it serves
// and the status of its successful responses.
type RestEndpoint struct {
	// ServiceId is the id of the service serving the endpoint, set by the
	// [IRestRouter] from the configuration.
	ServiceId int

	// Id is the id of the endpoint in the service.
	Id int

	// Pattern is the pattern of the requests the endpoint serves, as
	// [http.ServeMux] reads it, such as "GET /users/{id}".
	Pattern string

	// Status is the status of the successful responses of the endpoint,
	// such as 201, or 0 for 200.
	Status int

	// Streaming reports whether the endpoint streams its response, such as
	// Server-Sent Events, or takes its connection over, such as a
	// WebSocket, so that the middlewares buffering the responses, such as
	// the timeout middleware, pass its requests through.
	Streaming bool

	// The fields below document the endpoint, such as in an OpenAPI
	// document, their zero values leaving it undocumented.

	// Summary is the short summary of what the endpoint does.
	Summary string

	// Description is the longer description of the endpoint.
	Description string

	// Tags group the endpoint with the others of the same tags.
	Tags []string

	// Params is a value, such as the zero value, of the struct the path
	// values and the query of the requests are decoded into, its fields
	// named by their schema tags, as with the decoder of the rest package.
	Params any

	// Request is a value, such as the zero value, of the type the JSON body
	// of the requests is decoded into, or nil for no body.
	Request any

	// Response is a value, such as the zero value, of the type of the data
	// of the envelope of the successful responses, or nil for no data.
	Response any

	// Deprecated reports whether the endpoint is deprecated.
	Deprecated bool
}

// IRestRouter routes the requests of a REST server to the handlers of its
// endpoints, each wrapped in the middlewares of the server.
type IRestRouter interface {
	// Handle registers the handler of the endpoint, failing when the
	// endpoint is invalid, or its pattern conflicts with the pattern of
	// another endpoint.
	Handle(endpoint RestEndpoint, handler http.Handler) error
}

// IRestEndpointLister lists the endpoints registered on a router, such as
// to document them, as the [IRestRouter] given to the registers may do.
type IRestEndpointLister interface {
	// Endpoints returns the endpoints registered, in their order of
	// registration.
	Endpoints() []RestEndpoint
}

// IRestEndpointRegister registers endpoints on the routers of the REST
// servers of its group.
type IRestEndpointRegister interface {
	// Group returns the group of the servers the register registers its
	// endpoints on.
	Group() string

	// Register registers the endpoints on the router of a server.
	Register(router IRestRouter) error
}

// IRestResponseWriter is an [http.ResponseWriter] recording the status and
// the size of the response written through it, for the middlewares.
type IRestResponseWriter interface {
	http.ResponseWriter

	// Status returns the status of the response, 200 when the handler
	// wrote none, as the server then answers with it.
	Status() int

	// Size returns the number of bytes of the body written.
	Size() int

	// Written reports whether the status or a part of the body was
	// written, after which no other response can be written in its place.
	Written() bool
}

// IRestMiddleware wraps the handlers of the endpoints of a REST server,
// such as to log their requests. It is held by an
// [IRestMiddlewareFactory].
type IRestMiddleware interface {
	IResource

	// Wrap returns the handler serving the requests of the endpoint
	// through next.
	Wrap(endpoint RestEndpoint, next http.Handler) http.Handler
}

// IRestMiddlewareCreator creates the REST middlewares of a driver, for the
// [IRestMiddlewareFactory].
type IRestMiddlewareCreator = IResourceCreator[IRestMiddleware]

// IRestMiddlewareFactory creates and holds the REST middlewares, one per
// id.
type IRestMiddlewareFactory = IFactory[IRestMiddleware]

// IRestLoggerAdaptor reports the lifecycle of the REST servers, each by
// its id: its start, with the address it listens on, its failures, its
// shutdown and its end. It is held by an [IRestLoggerAdaptorFactory].
type IRestLoggerAdaptor interface {
	IResource

	Start(id string, address string)
	Error(id string, e error)
	Shutdown(id string)
	Done(id string)
}

// IRestLoggerAdaptorCreator creates the REST logger adaptors of a driver,
// for the [IRestLoggerAdaptorFactory].
type IRestLoggerAdaptorCreator = IResourceCreator[IRestLoggerAdaptor]

// IRestLoggerAdaptorFactory creates and holds the REST logger adaptors, one
// per id.
type IRestLoggerAdaptorFactory = IFactory[IRestLoggerAdaptor]

// IRestServer is a REST server, run as a process of the [IKennel]: Run
// serves its endpoints until its context is done or it is closed, then
// shuts it down. It is held by an [IRestServerFactory].
type IRestServer interface {
	IWatchdogProcess

	// Address returns the address the server listens on while it runs,
	// such as "127.0.0.1:8080", or the address it is configured with
	// otherwise.
	Address() string
}

// IRestServerCreator creates the REST servers of a driver, for the
// [IRestServerFactory].
type IRestServerCreator = IResourceCreator[IRestServer]

// IRestServerFactory creates and holds the REST servers, one per id.
type IRestServerFactory = IFactory[IRestServer]
