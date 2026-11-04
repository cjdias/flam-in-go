package standard

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// clientMetrics are the Prometheus metrics of the clients, shared by the
// clients of the same namespace.
type clientMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// newClientMetrics registers the metrics of the namespace on the
// registerer, or takes those registered already by a client of the same
// namespace, unregistering those it registered when one of them cannot
// be registered.
func newClientMetrics(
	registerer prometheus.Registerer,
	namespace string,
) (*clientMetrics, error) {
	labels := []string{LabelClient, LabelMethod, LabelCode}
	r := &registration{registerer: registerer}
	m := &clientMetrics{
		requests: register(r, prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: Subsystem,
			Name:      MetricRequests,
			Help:      metricRequestsHelp,
		}, labels)),
		duration: register(r, prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: Subsystem,
			Name:      MetricDuration,
			Help:      metricDurationHelp,
			Buckets:   prometheus.DefBuckets,
		}, labels)),
	}
	if r.err != nil {
		r.rollback()
		return nil, r.err
	}
	return m, nil
}

// Help texts of the metrics.
const (
	metricRequestsHelp = "Number of attempts of the requests sent by the HTTP clients, by client, method and status or error."
	metricDurationHelp = "Time taken by the attempts of the requests sent by the HTTP clients to get their response headers, in seconds, by client, method and status or error."
)

// registration registers the metrics of a client, keeping the first
// error, after which it registers nothing, and the metrics it registered,
// to unregister them on failure.
type registration struct {
	registerer prometheus.Registerer
	registered []prometheus.Collector
	err        error
}

// rollback unregisters the metrics registered.
func (r *registration) rollback() {
	for _, collector := range r.registered {
		r.registerer.Unregister(collector)
	}
}

// register registers the metric, returning it, or the metric of the same
// type registered already under the same description, or the zero value
// once the registration failed.
func register[C prometheus.Collector](
	r *registration,
	collector C,
) C {
	var zero C
	if r.err != nil {
		return zero
	}
	err := r.registerer.Register(collector)
	if err == nil {
		r.registered = append(r.registered, collector)
		return collector
	}
	var registered prometheus.AlreadyRegisteredError
	if errors.As(err, &registered) {
		if existing, ok := registered.ExistingCollector.(C); ok {
			return existing
		}
	}
	r.err = err
	return zero
}

// startSpan returns the context of the attempt of the request to the
// target URL, holding its client span, and the span, which is a no-op
// one, leaving the context of the request as is, without tracer.
func (c *httpClient) startSpan(
	req *http.Request,
	target *url.URL,
	attempt int,
) (context.Context, trace.Span) {
	ctx := req.Context()
	if c.tracer == nil {
		return ctx, trace.SpanFromContext(context.Background())
	}
	method, known := spanMethod(req.Method)
	name := spanNamePrefix + method
	attributes := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLFull(redact(target)),
		semconv.ServerAddress(target.Hostname()),
	}
	if !known {
		name = spanNameOther
		attributes = append(attributes, semconv.HTTPRequestMethodOriginal(req.Method))
	}
	if port, err := strconv.Atoi(target.Port()); err == nil {
		attributes = append(attributes, semconv.ServerPort(port))
	}
	if attempt > 0 {
		attributes = append(attributes, semconv.HTTPRequestResendCount(attempt))
	}
	return c.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attributes...),
	)
}

// observe records the outcome of the attempt of the request, its response
// or its error, on its span, then ends it, and on the metrics of the
// client, when it has some, the attempt having started at start.
func (c *httpClient) observe(
	span trace.Span,
	method string,
	start time.Time,
	resp *http.Response,
	err error,
) {
	defer span.End()
	var code string
	switch {
	case err != nil:
		code = errorType(err)
		span.RecordError(err)
		span.SetAttributes(semconv.ErrorTypeKey.String(code))
		span.SetStatus(codes.Error, err.Error())
	default:
		code = strconv.Itoa(resp.StatusCode)
		span.SetAttributes(semconv.HTTPResponseStatusCode(resp.StatusCode))
		if resp.StatusCode >= http.StatusBadRequest {
			span.SetAttributes(semconv.ErrorTypeKey.String(code))
			span.SetStatus(codes.Error, http.StatusText(resp.StatusCode))
		}
	}
	if c.metrics != nil {
		label, known := spanMethod(method)
		if !known {
			label = MethodOther
		}
		c.metrics.requests.WithLabelValues(c.id, label, code).Inc()
		c.metrics.duration.WithLabelValues(c.id, label, code).Observe(c.clock.Since(start).Seconds())
	}
}

// spanMethod returns the method of the request, GET when empty, as the
// semantic conventions name it, "_OTHER" for a method other than the
// standard ones, and whether it is a standard one.
func spanMethod(
	method string,
) (string, bool) {
	switch method {
	case "":
		return http.MethodGet, true
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method, true
	}
	return semconv.HTTPRequestMethodOther.Value.AsString(), false
}

// errorType returns the type of the error of an attempt, as held by the
// "error.type" attribute of its span and by the LabelCode label of its
// metrics: ErrorTypeOpen when the circuit breaker refused it,
// ErrorTypeTimeout when it timed out, ErrorTypeCanceled when its context
// was cancelled, or else "_OTHER".
func errorType(
	err error,
) string {
	var timeout interface{ Timeout() bool }
	switch {
	case errors.Is(err, ErrOpen):
		return ErrorTypeOpen
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &timeout) && timeout.Timeout():
		return ErrorTypeTimeout
	case errors.Is(err, context.Canceled):
		return ErrorTypeCanceled
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

// redact returns the URL, its user and password, if any, replaced by
// "REDACTED", as the semantic conventions require.
func redact(
	target *url.URL,
) string {
	if target.User == nil {
		return target.String()
	}
	u := *target
	u.User = url.UserPassword(redacted, redacted)
	return u.String()
}

// Names of the spans and replacement of the credentials of their URL.
const (
	spanNamePrefix = "HTTP "
	spanNameOther  = "HTTP"
	redacted       = "REDACTED"
)
