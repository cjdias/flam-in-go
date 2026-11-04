// Package standard is the client driver sending the requests with a
// [net/http] client of its own: each client resolves the relative URLs of
// its requests against its base URL, adds its default headers to them,
// and sends them within its timeout, under the protection of its circuit
// breaker, taken from the
// [github.com/cjdias/flam-in-go/pkg/flam.IClientCircuitBreakerFactory],
// which fails them fast with [ErrOpen] while its circuit is open.
//
// The requests the server fails, answering with a 5xx status, and those
// failing to reach it count as failures for the circuit breaker, while the
// requests cancelled by their caller do not count. Those of an idempotent
// method, GET, HEAD, OPTIONS, TRACE, PUT or DELETE, whose body can be sent
// again, are sent again on such a failure, up to the number of retries of
// the client, after a delay doubled at each retry, up to its maximum,
// waited with the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
// container.
//
// When the container provides the
// [go.opentelemetry.io/otel/trace.TracerProvider] and the
// [go.opentelemetry.io/otel/propagation.TextMapPropagator] of the bundle
// of [github.com/cjdias/flam-in-go/pkg/flam/tracing], each attempt of a
// request, retries included, runs within a client span of its own, a
// child of the span of the context of the request, named by its method,
// such as "HTTP GET", and holding the attributes of the HTTP semantic
// conventions of OpenTelemetry: the method, the full URL, its user and
// password redacted, the address and port of the server, the number of
// the retry, and the status of the response, or the type of the error, a
// 4xx or 5xx status, or an error, setting the status of the span to an
// error. The context of the span is injected into the headers of the copy
// of the request sent, such as the "traceparent" header of the W3C trace
// context, so that the server goes on with the trace, the request of the
// caller being left as is.
//
// When the container provides the Prometheus registerer of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics], each attempt is also
// counted on [MetricRequests] and its time, until its response headers
// are received, observed on [MetricDuration], named after the namespace of
// the client, "flam" unless set, such as "flam_client_requests_total".
// They are labelled by the id of the client at [LabelClient], the method
// at [LabelMethod] and the status of the response, or the type of the
// error, at [LabelCode]. The clients of the same namespace share their
// metrics. The tracing and the metrics of a client are disabled by its
// [ConfigFieldTracing] and [ConfigFieldMetrics] fields:
//
//	flam:
//	  client:
//	    clients:
//	      users: {driver: flam.client.clients.driver.standard, tracing: true, metrics: false}
package standard
