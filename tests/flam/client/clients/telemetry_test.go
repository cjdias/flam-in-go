package clients

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/client/clients/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// telemetry is what an app provides to trace and measure its clients,
// each part being provided when not nil.
type telemetry struct {
	recorder   *tracetest.SpanRecorder
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
	registry   *prometheus.Registry
}

// newTelemetry returns a telemetry recording the spans of its provider,
// propagating the W3C trace context, and registering on a registry of its
// own.
func newTelemetry() *telemetry {
	recorder := tracetest.NewSpanRecorder()
	return &telemetry{
		recorder:   recorder,
		provider:   sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)),
		propagator: propagation.TraceContext{},
		registry:   prometheus.NewRegistry(),
	}
}

// setup provides the parts of the telemetry that are not nil to the app.
func (tel *telemetry) setup(app *testutil.App) {
	if tel.provider != nil {
		_ = app.Provide(func() trace.TracerProvider { return tel.provider })
	}
	if tel.propagator != nil {
		_ = app.Provide(func() propagation.TextMapPropagator { return tel.propagator })
	}
	if tel.registry != nil {
		_ = app.Provide(func() prometheus.Registerer { return tel.registry })
	}
}

// spans returns the spans ended in the recorder.
func (tel *telemetry) spans() []sdktrace.ReadOnlySpan {
	return tel.recorder.Ended()
}

// metrics asserts that the metrics of the names gathered from the registry
// are the ones of the text exposition format.
func (tel *telemetry) metrics(t *testing.T, want string, names ...string) {
	t.Helper()
	if err := promtestutil.GatherAndCompare(tel.registry, strings.NewReader(want), names...); err != nil {
		t.Error(err)
	}
}

// telemetryEnv returns the env of the client of the fields, traced and
// measured by the telemetry.
func telemetryEnv(t *testing.T, tel *telemetry, fields container.Bag) *env {
	t.Helper()
	e, err := create(t, fields, nil, tel.setup)
	testutil.NoError(t, err)
	return e
}

// attributesOf returns the attributes of the span, by key.
func attributesOf(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

// requestsHeader is the header of the exposition of the requests counter.
const requestsHeader = `# HELP flam_client_requests_total Number of attempts of the requests sent by the HTTP clients, by client, method and status or error.
# TYPE flam_client_requests_total counter
`

func Test_Tracing(t *testing.T) {
	t.Run("should trace the request and propagate its context to the server", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		base := strings.Replace(s.URL, "http://", "http://user:secret@", 1)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldBaseURL: base + "/v1/"})

		ctx, parent := tel.provider.Tracer("test").Start(context.Background(), "parent")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "items?q=1", nil)
		testutil.NoError(t, err)
		resp, err := e.client.Do(req)
		testutil.NoError(t, err)
		_ = read(t, resp)
		parent.End()

		testutil.Equal(t, "caller headers", req.Header, http.Header{})
		spans := tel.spans()
		testutil.Equal(t, "spans", len(spans), 2)
		span := spans[0]
		testutil.Equal(t, "Name()", span.Name(), "HTTP GET")
		testutil.Equal(t, "SpanKind()", span.SpanKind(), trace.SpanKindClient)
		testutil.Equal(t, "Parent()", span.Parent().SpanID(), parent.SpanContext().SpanID())
		testutil.Equal(t, "Status()", span.Status().Code, codes.Unset)
		testutil.Equal(t, "InstrumentationScope()", span.InstrumentationScope().Name, standard.ScopeName)
		host, port, _ := strings.Cut(strings.TrimPrefix(s.URL, "http://"), ":")
		attrs := attributesOf(span)
		testutil.Equal(t, "method", attrs["http.request.method"].AsString(), "GET")
		testutil.Equal(t, "url", attrs["url.full"].AsString(),
			strings.Replace(s.URL, "http://", "http://REDACTED:REDACTED@", 1)+"/v1/items?q=1")
		testutil.Equal(t, "address", attrs["server.address"].AsString(), host)
		wantPort, err := strconv.Atoi(port)
		testutil.NoError(t, err)
		testutil.Equal(t, "port", attrs["server.port"].AsInt64(), int64(wantPort))
		testutil.Equal(t, "status", attrs["http.response.status_code"].AsInt64(), int64(200))
		testutil.Equal(t, "resend", attrs["http.request.resend_count"].Type(), attribute.EMPTY)
		testutil.Equal(t, "error", attrs["error.type"].Type(), attribute.EMPTY)

		header := s.recorded()[0].header.Get("Traceparent")
		testutil.Equal(t, "traceparent", header,
			"00-"+span.SpanContext().TraceID().String()+"-"+span.SpanContext().SpanID().String()+"-01")
	})

	t.Run("should trace each attempt of a retried request", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil, http.StatusServiceUnavailable, http.StatusOK)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "0s"})
		resp, err := e.client.Request(context.Background(), http.MethodPut, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)

		spans := tel.spans()
		testutil.Equal(t, "spans", len(spans), 2)
		first, second := attributesOf(spans[0]), attributesOf(spans[1])
		testutil.Equal(t, "first name", spans[0].Name(), "HTTP PUT")
		testutil.Equal(t, "first status", spans[0].Status(), sdktrace.Status{Code: codes.Error, Description: "Service Unavailable"})
		testutil.Equal(t, "first error", first["error.type"].AsString(), "503")
		testutil.Equal(t, "first resend", first["http.request.resend_count"].Type(), attribute.EMPTY)
		testutil.Equal(t, "second status", spans[1].Status().Code, codes.Unset)
		testutil.Equal(t, "second resend", second["http.request.resend_count"].AsInt64(), int64(1))
		if spans[0].SpanContext().TraceID() == spans[1].SpanContext().TraceID() {
			t.Error("the attempts of a request without span share a trace")
		}
		hits := s.recorded()
		testutil.Equal(t, "first traceparent", hits[0].header.Get("Traceparent"),
			"00-"+spans[0].SpanContext().TraceID().String()+"-"+spans[0].SpanContext().SpanID().String()+"-01")
		testutil.Equal(t, "second traceparent", hits[1].header.Get("Traceparent"),
			"00-"+spans[1].SpanContext().TraceID().String()+"-"+spans[1].SpanContext().SpanID().String()+"-01")
	})

	t.Run("should name the span of a method other than the standard ones", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil, http.StatusNotFound)
		e := telemetryEnv(t, tel, container.Bag{})
		resp, err := e.client.Request(context.Background(), "PURGE", s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)

		span := tel.spans()[0]
		attrs := attributesOf(span)
		testutil.Equal(t, "Name()", span.Name(), "HTTP")
		testutil.Equal(t, "method", attrs["http.request.method"].AsString(), "_OTHER")
		testutil.Equal(t, "original", attrs["http.request.method_original"].AsString(), "PURGE")
		testutil.Equal(t, "error", attrs["error.type"].AsString(), "404")
		testutil.Equal(t, "Status()", span.Status(), sdktrace.Status{Code: codes.Error, Description: "Not Found"})
		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="404",method="OTHER"} 1
`, "flam_client_requests_total")
	})

	t.Run("should take an empty method as GET", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldBaseURL: s.URL})
		resp, err := e.client.Do(&http.Request{URL: &url.URL{Path: "/x"}})
		testutil.NoError(t, err)
		_ = read(t, resp)
		span := tel.spans()[0]
		testutil.Equal(t, "Name()", span.Name(), "HTTP GET")
		testutil.Equal(t, "method", attributesOf(span)["http.request.method"].AsString(), "GET")
		testutil.Equal(t, "hit", s.recorded()[0].method, http.MethodGet)
		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="200",method="GET"} 1
`, "flam_client_requests_total")
	})

	t.Run("should record a transport failure as of another type", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		s.Close()
		e := telemetryEnv(t, tel, container.Bag{})
		_, err := e.client.Request(context.Background(), http.MethodGet, s.URL+"/down", nil)
		if !errors.Is(err, standard.ErrRequest) {
			t.Fatalf("Request() = %v, want ErrRequest", err)
		}
		span := tel.spans()[0]
		testutil.Equal(t, "error", attributesOf(span)["error.type"].AsString(), "_OTHER")
		testutil.Equal(t, "Status()", span.Status(), sdktrace.Status{Code: codes.Error, Description: err.Error()})
		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="_OTHER",method="GET"} 1
`, "flam_client_requests_total")
	})

	t.Run("should record the errors of the attempts", func(t *testing.T) {
		tel := newTelemetry()
		release := make(chan struct{})
		s := newServer(t, nil, http.StatusInternalServerError)
		s.handle = func(_ http.ResponseWriter, req *http.Request, n int) bool {
			if n <= 2 {
				return false
			}
			select {
			case <-release:
			case <-req.Context().Done():
			}
			return false
		}
		defer close(release)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldBreaker: "cb"})
		for range 2 {
			resp, err := e.client.Request(context.Background(), http.MethodGet, "/x", nil)
			testutil.NoError(t, err)
			_ = read(t, resp)
		}
		_, err := e.client.Request(context.Background(), http.MethodGet, "/x", nil)
		if !errors.Is(err, standard.ErrOpen) {
			t.Fatalf("Request() = %v, want ErrOpen", err)
		}
		e.clock.Advance(10 * time.Second) // half open

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for len(s.recorded()) < 3 {
				time.Sleep(50 * time.Microsecond)
			}
			cancel()
		}()
		_, err = e.client.Request(ctx, http.MethodGet, "/x", nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Request() = %v, want context.Canceled", err)
		}

		ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err = e.client.Request(ctx, http.MethodGet, "/x", nil)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Request() = %v, want context.DeadlineExceeded", err)
		}

		spans := tel.spans()
		testutil.Equal(t, "spans", len(spans), 5)
		for i, want := range []string{"500", "500", standard.ErrorTypeOpen, standard.ErrorTypeCanceled, standard.ErrorTypeTimeout} {
			testutil.Equal(t, "error", attributesOf(spans[i])["error.type"].AsString(), want)
			testutil.Equal(t, "status", spans[i].Status().Code, codes.Error)
		}
		testutil.Equal(t, "events", len(spans[2].Events()), 1)
		testutil.Equal(t, "event", spans[2].Events()[0].Name, "exception")
		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="500",method="GET"} 2
flam_client_requests_total{client="main",code="canceled",method="GET"} 1
flam_client_requests_total{client="main",code="circuit_open",method="GET"} 1
flam_client_requests_total{client="main",code="timeout",method="GET"} 1
`, "flam_client_requests_total")
	})

	t.Run("should take a timeout of the client as a timeout", func(t *testing.T) {
		tel := newTelemetry()
		release := make(chan struct{})
		s := newServer(t, nil)
		s.handle = func(http.ResponseWriter, *http.Request, int) bool {
			<-release
			return false
		}
		defer close(release)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldTimeout: "20ms"})
		_, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		if !errors.Is(err, standard.ErrRequest) {
			t.Fatalf("Request() = %v, want ErrRequest", err)
		}
		testutil.Equal(t, "error", attributesOf(tel.spans()[0])["error.type"].AsString(), standard.ErrorTypeTimeout)
	})

	t.Run("should not trace when disabled", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldTracing: false})
		ctx, parent := tel.provider.Tracer("test").Start(context.Background(), "parent")
		resp, err := e.client.Request(ctx, http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		parent.End()
		testutil.Equal(t, "spans", len(tel.spans()), 1)
		testutil.Equal(t, "traceparent", s.recorded()[0].header.Get("Traceparent"), "")
		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="200",method="GET"} 1
`, "flam_client_requests_total")
	})

	t.Run("should not trace without a propagator", func(t *testing.T) {
		tel := newTelemetry()
		tel.propagator = nil
		s := newServer(t, nil)
		e := telemetryEnv(t, tel, container.Bag{})
		resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		testutil.Equal(t, "spans", len(tel.spans()), 0)
		testutil.Equal(t, "traceparent", s.recorded()[0].header.Get("Traceparent"), "")
	})

	t.Run("should not trace without a tracer provider", func(t *testing.T) {
		tel := newTelemetry()
		tel.provider = nil
		s := newServer(t, nil)
		e := telemetryEnv(t, tel, container.Bag{})
		resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		testutil.Equal(t, "traceparent", s.recorded()[0].header.Get("Traceparent"), "")
	})
}

func Test_Metrics(t *testing.T) {
	t.Run("should measure the attempts", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil, http.StatusBadGateway, http.StatusCreated)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "0s"})
		resp, err := e.client.Request(context.Background(), http.MethodDelete, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)

		tel.metrics(t, requestsHeader+`flam_client_requests_total{client="main",code="201",method="DELETE"} 1
flam_client_requests_total{client="main",code="502",method="DELETE"} 1
`, "flam_client_requests_total")
		count, err := promtestutil.GatherAndCount(tel.registry, "flam_client_request_duration_seconds")
		testutil.NoError(t, err)
		testutil.Equal(t, "duration series", count, 2)
	})

	t.Run("should share the metrics of the clients of a namespace", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		var app *testutil.App
		e, err := create(t, container.Bag{standard.ConfigFieldNamespace: "app"}, map[string]any{
			client.ClientConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: standard.Driver, standard.ConfigFieldNamespace: "app"},
		}, tel.setup, func(a *testutil.App) { app = a })
		testutil.NoError(t, err)
		var other flam.IClient
		testutil.NoError(t, app.Invoke(func(f flam.IClientFactory) (err error) {
			other, err = f.Get("other")
			return err
		}))
		for _, c := range []flam.IClient{e.client, other} {
			resp, err := c.Request(context.Background(), http.MethodPost, s.URL, nil)
			testutil.NoError(t, err)
			_ = read(t, resp)
		}
		tel.metrics(t, `# HELP app_client_requests_total Number of attempts of the requests sent by the HTTP clients, by client, method and status or error.
# TYPE app_client_requests_total counter
app_client_requests_total{client="main",code="200",method="POST"} 1
app_client_requests_total{client="other",code="200",method="POST"} 1
`, "app_client_requests_total")
	})

	t.Run("should not measure when disabled or without registerer", func(t *testing.T) {
		tel := newTelemetry()
		s := newServer(t, nil)
		e := telemetryEnv(t, tel, container.Bag{standard.ConfigFieldMetrics: false})
		resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		families, err := tel.registry.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 0)

		tel = newTelemetry()
		tel.registry = nil
		e = telemetryEnv(t, tel, container.Bag{})
		resp, err = e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		testutil.Equal(t, "spans", len(tel.spans()), 1)
	})

	t.Run("should fail to register the metrics of an invalid namespace", func(t *testing.T) {
		tel := newTelemetry()
		_, err := create(t, container.Bag{standard.ConfigFieldNamespace: "\xff"}, nil, tel.setup)
		e := innermost(t, err)
		testutil.Error(t, e, standard.ErrRegister, e.Error(), map[string]any{standard.ErrorFieldID: "main"})
		if !strings.HasPrefix(e.Error(), "unable to create standard client: unable to register standard client metrics: ") {
			t.Errorf("Error() = %q", e.Error())
		}
	})

	t.Run("should unregister the metrics registered when one cannot be", func(t *testing.T) {
		tel := newTelemetry()
		testutil.NoError(t, tel.registry.Register(prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "flam_client_request_duration_seconds",
			Help: "taken",
		})))
		_, err := create(t, container.Bag{}, nil, tel.setup)
		if !errors.Is(err, standard.ErrRegister) {
			t.Fatalf("Get() = %v, want ErrRegister", err)
		}
		testutil.Equal(t, "requests unregistered", tel.registry.Unregister(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flam_client_requests_total",
			Help: "Number of attempts of the requests sent by the HTTP clients, by client, method and status or error.",
		}, []string{standard.LabelClient, standard.LabelMethod, standard.LabelCode})), false)
	})

	t.Run("should fail when a metric of another type holds the name", func(t *testing.T) {
		tel := newTelemetry()
		testutil.NoError(t, tel.registry.Register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flam_client_requests_total",
			Help: "Number of attempts of the requests sent by the HTTP clients, by client, method and status or error.",
		}, []string{standard.LabelClient, standard.LabelMethod, standard.LabelCode})))
		_, err := create(t, container.Bag{}, nil, tel.setup)
		var already prometheus.AlreadyRegisteredError
		if !errors.Is(err, standard.ErrRegister) || !errors.As(err, &already) {
			t.Fatalf("Get() = %v, want ErrRegister and AlreadyRegisteredError", err)
		}
	})
}
