// Package tracing is the gRPC interceptor driver tracing the calls served
// by a server, unary or streaming, each within a server span of the
// [go.opentelemetry.io/otel/trace.TracerProvider] of the container,
// provided by the bundle of [github.com/cjdias/flam-in-go/pkg/flam/tracing].
//
// The span of a call is the child of the span of the context the
// [go.opentelemetry.io/otel/propagation.TextMapPropagator] of the
// container extracts from its metadata, such as the "traceparent" entry of
// the W3C trace context sent by the service calling it, so that the trace
// goes on across the services, or else the root of a new trace. It is
// named by the method of the call, such as "pkg.Users/Get", and holds the
// attributes of the RPC semantic conventions of OpenTelemetry: the
// system, "grpc", the method and the code of the status of the call, such
// as "NOT_FOUND". A server error, as
// [github.com/cjdias/flam-in-go/pkg/flam/grpc.IsServerError] tells, or a
// handler that panicked, recorded as [ErrPanicked] and taken for an
// Internal status, sets the status of the span to an error.
//
// The context of the call given to the handler, or of its stream, holds
// the span, so that the spans started from it, such as by the database and
// redis instrumentations of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing], are its children.
//
// The interceptor is used by listing its id, [Id], among the interceptors
// of a server, first, so that its span covers the other interceptors, such
// as:
//
//	flam:
//	  grpc:
//	    defaults:
//	      servers:
//	        interceptors: [tracing, metrics, log, recovery]
package tracing
