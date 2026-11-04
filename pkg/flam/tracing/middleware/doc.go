// Package middleware is the REST middleware driver tracing the requests
// served by the endpoints, each within a server span of the
// [go.opentelemetry.io/otel/trace.TracerProvider] of the container,
// provided by the bundle of [github.com/cjdias/flam-in-go/pkg/flam/tracing].
//
// The span of a request is the child of the span of the context the
// [go.opentelemetry.io/otel/propagation.TextMapPropagator] of the
// container extracts from its headers, such as the "traceparent" header
// of the W3C trace context sent by the service calling it, so that the
// trace goes on across the services, or else the root of a new trace. It
// is named by the pattern of the endpoint, such as "GET /items/{id}", or
// [UnmatchedSpanName] for the requests matching no pattern, and holds the
// attributes of the HTTP semantic conventions of OpenTelemetry: the
// method, the route, the path of the URL and the status of the response,
// along with the ids of the service and of the endpoint, at
// [AttributeServiceId] and [AttributeEndpointId]. A response of a 5xx
// status, or a handler that panicked, sets the status of the span to an
// error.
//
// The context of the request given to the handler holds the span, so that
// the spans started from it, such as by the database and redis
// instrumentations of [github.com/cjdias/flam-in-go/pkg/flam/tracing],
// are its children:
//
//	func (r *users) get(w http.ResponseWriter, req *http.Request) {
//		var user User
//		err := r.db.WithContext(req.Context()).First(&user, req.PathValue("id")).Error
//		...
//	}
//
// The middleware is used by listing its id, [Id], among the middlewares of
// a server, first, so that its span covers the other middlewares, such
// as:
//
//	flam:
//	  rest:
//	    defaults:
//	      servers:
//	        middlewares: [tracing, envelope, log]
package middleware
