package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the tracing [flam.IRestMiddleware], serving each request
// within a server span.
type middleware struct {
	id         string
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next within a
// server span, a child of the span of the context the propagator extracts
// from the headers of the request, if any, named by the pattern of the
// endpoint, or [UnmatchedSpanName] for the requests matching no pattern.
// The context of the request given to next holds the span, so that the
// spans the handler starts are its children.
//
// The span holds the method, the route, the path of the URL, and the
// status of the response, along with the ids of the service and of the
// endpoint. A response of a 5xx status, or a handler that panicked, sets
// the status of the span to an error, a panic being recorded as
// [ErrPanicked], then going on.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	name := endpoint.Pattern
	attributes := []attribute.KeyValue{
		attribute.Int(AttributeServiceId, endpoint.ServiceId),
	}
	if name == "" {
		name = UnmatchedSpanName
	} else {
		attributes = append(attributes,
			semconv.HTTPRoute(middlewareRoute(endpoint.Pattern)),
			attribute.Int(AttributeEndpointId, endpoint.Id),
		)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := m.propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		ctx, span := m.tracer.Start(ctx, name,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attributes...),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(req.Method),
				semconv.URLPath(req.URL.Path),
			),
		)
		rw := rest.NewResponseWriter(w)
		panicked := true
		defer func() {
			m.end(span, rw, panicked)
		}()
		next.ServeHTTP(rw, req.WithContext(ctx))
		panicked = false
	})
}

// end records the status of the response on the span, and its error, if
// any, then ends it.
func (m *middleware) end(
	span trace.Span,
	w flam.IRestResponseWriter,
	panicked bool,
) {
	defer span.End()
	status := w.Status()
	if panicked && !w.Written() {
		status = http.StatusInternalServerError
	}
	span.SetAttributes(semconv.HTTPResponseStatusCode(status))
	if panicked {
		span.RecordError(ErrPanicked)
	}
	if panicked || status >= http.StatusInternalServerError {
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(status)))
		span.SetStatus(codes.Error, http.StatusText(status))
	}
}

// middlewareRoute returns the path of the pattern, without its method and
// host, such as "/items/{id}" for "GET /items/{id}", as the route of the
// semantic conventions is.
func middlewareRoute(
	pattern string,
) string {
	if _, path, found := strings.Cut(pattern, " "); found {
		pattern = strings.TrimLeft(path, " \t")
	}
	if i := strings.Index(pattern, "/"); i > 0 {
		pattern = pattern[i:]
	}
	return pattern
}
