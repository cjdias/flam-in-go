package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// deps are the values provided to the container of an app, when not nil.
type deps struct {
	provider   trace.TracerProvider
	propagator propagation.TextMapPropagator
}

// create returns the tracing middleware "tracing", created by the
// middleware factory of an app with the middleware bundle and the deps.
func create(t *testing.T, d deps) (flam.IRestMiddleware, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), middleware.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	app := testutil.NewApp()
	if d.provider != nil {
		testutil.NoError(t, app.Provide(func() trace.TracerProvider { return d.provider }))
	}
	if d.propagator != nil {
		testutil.NoError(t, app.Provide(func() propagation.TextMapPropagator { return d.propagator }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var m flam.IRestMiddleware
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) { m, err = f.Get(middleware.Id) }))
	return m, err
}

// traced returns a tracing middleware recording its spans in the
// recorder returned.
func traced(t *testing.T) (flam.IRestMiddleware, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	m, err := create(t, deps{
		provider:   provider,
		propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	})
	testutil.NoError(t, err)
	return m, recorder
}

// serve serves the request with a router of the service 3 holding the
// endpoint 7 of the pattern, wrapped in the middleware, returning the
// recorded response and the value its serving panicked with, if any.
func serve(t *testing.T, m flam.IRestMiddleware, pattern string, req *http.Request, handler http.HandlerFunc) (rec *httptest.ResponseRecorder, panicked any) {
	t.Helper()
	router, err := rest.NewRouter(3, m)
	testutil.NoError(t, err)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: pattern}, handler))

	rec = httptest.NewRecorder()
	defer func() { panicked = recover() }()
	router.ServeHTTP(rec, req)
	return rec, nil
}

// only returns the single span ended in the recorder.
func only(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	return spans[0]
}

// attributes returns the attributes of the span, by key.
func attributes(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the creator and configure the middleware", func(t *testing.T) {
		b := middleware.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.tracing")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag.Get(rest.MiddlewareConfigPath+"."+middleware.Id),
			container.Bag{factory.ConfigFieldDriver: middleware.Driver})
		testutil.Equal(t, "Id", middleware.Id, "tracing")
		testutil.Equal(t, "Driver", middleware.Driver, "flam.rest.middlewares.driver.tracing")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.tracing", "provider/1"})
	})

	t.Run("should fail to configure on a bag refusing the values", func(t *testing.T) {
		if err := middleware.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "taken"}); err == nil {
			t.Errorf("Config() = nil, want an error")
		}
	})

	t.Run("should create the middleware with the tracing bundle", func(t *testing.T) {
		bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), tracing.NewBundle(), middleware.NewBundle()}
		bag := container.Bag{}
		testutil.NoError(t, middleware.NewBundle().(flam.IConfigurableBundle).Config(bag))
		app := testutil.NewApp()
		for _, b := range bundles {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) error {
			m, err := f.Get(middleware.Id)
			if err == nil {
				testutil.Equal(t, "Id()", m.Id(), middleware.Id)
			}
			return err
		}))
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should fail without tracer provider or propagator", func(t *testing.T) {
		for _, d := range []deps{
			{},
			{provider: sdktrace.NewTracerProvider()},
			{propagator: propagation.TraceContext{}},
		} {
			_, err := create(t, d)
			err = errors.Unwrap(err) // the error of the creator, wrapped by the factory
			testutil.Error(t, err, middleware.ErrMissingTracing,
				"unable to create tracing rest middleware: tracer provider or propagator not provided by the container",
				map[string]any{middleware.ErrorFieldID: middleware.Id})
		}
	})
}

func Test_Middleware(t *testing.T) {
	t.Run("should serve the request within a server span", func(t *testing.T) {
		m, recorder := traced(t)
		var inner trace.SpanContext
		rec, _ := serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/items/1", nil),
			func(w http.ResponseWriter, req *http.Request) {
				inner = trace.SpanContextFromContext(req.Context())
				w.WriteHeader(http.StatusCreated)
			})
		testutil.Equal(t, "status", rec.Code, http.StatusCreated)

		span := only(t, recorder)
		testutil.Equal(t, "name", span.Name(), "GET /items/{id}")
		testutil.Equal(t, "kind", span.SpanKind(), trace.SpanKindServer)
		testutil.Equal(t, "span in context", inner, span.SpanContext())
		testutil.Equal(t, "parent", span.Parent().IsValid(), false)
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
		testutil.Equal(t, "attributes", attributes(span), map[attribute.Key]attribute.Value{
			"http.request.method":          attribute.StringValue("GET"),
			"url.path":                     attribute.StringValue("/items/1"),
			"http.route":                   attribute.StringValue("/items/{id}"),
			"http.response.status_code":    attribute.IntValue(http.StatusCreated),
			middleware.AttributeServiceId:  attribute.IntValue(3),
			middleware.AttributeEndpointId: attribute.IntValue(7),
		})
		testutil.Equal(t, "ScopeName", span.InstrumentationScope().Name, middleware.ScopeName)
	})

	t.Run("should continue the trace of the request headers", func(t *testing.T) {
		m, recorder := traced(t)
		req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
		req.Header.Set("traceparent", "00-0102030405060708090a0b0c0d0e0f10-1112131415161718-01")
		serve(t, m, "GET /items/{id}", req, func(http.ResponseWriter, *http.Request) {})

		span := only(t, recorder)
		testutil.Equal(t, "trace id", span.SpanContext().TraceID().String(), "0102030405060708090a0b0c0d0e0f10")
		testutil.Equal(t, "parent id", span.Parent().SpanID().String(), "1112131415161718")
		testutil.Equal(t, "remote parent", span.Parent().IsRemote(), true)
	})

	t.Run("should set the error status for a 5xx response", func(t *testing.T) {
		m, recorder := traced(t)
		serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/items/1", nil),
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })

		span := only(t, recorder)
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: "Service Unavailable"})
		attrs := attributes(span)
		testutil.Equal(t, "status code", attrs["http.response.status_code"], attribute.IntValue(503))
		testutil.Equal(t, "error type", attrs["error.type"], attribute.StringValue("503"))
		testutil.Equal(t, "events", len(span.Events()), 0)
	})

	t.Run("should leave the status unset for a 4xx response", func(t *testing.T) {
		m, recorder := traced(t)
		serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/items/1", nil),
			func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

		span := only(t, recorder)
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
		_, ok := attributes(span)["error.type"]
		testutil.Equal(t, "has error type", ok, false)
	})

	t.Run("should record a panic before the response, the panic going on", func(t *testing.T) {
		m, recorder := traced(t)
		_, panicked := serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/items/1", nil),
			func(http.ResponseWriter, *http.Request) { panic("boom") })
		testutil.Equal(t, "panicked", panicked, "boom")

		span := only(t, recorder)
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: "Internal Server Error"})
		testutil.Equal(t, "status code", attributes(span)["http.response.status_code"], attribute.IntValue(500))
		events := span.Events()
		testutil.Equal(t, "events", len(events), 1)
		testutil.Equal(t, "event", events[0].Name, "exception")
		for _, kv := range events[0].Attributes {
			if kv.Key == "exception.message" {
				testutil.Equal(t, "message", kv.Value.AsString(), middleware.ErrPanicked.Error())
			}
		}
	})

	t.Run("should record a panic after the response", func(t *testing.T) {
		m, recorder := traced(t)
		_, panicked := serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/items/1", nil),
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				panic("boom")
			})
		testutil.Equal(t, "panicked", panicked, "boom")

		span := only(t, recorder)
		testutil.Equal(t, "status", span.Status().Code, codes.Error)
		attrs := attributes(span)
		testutil.Equal(t, "status code", attrs["http.response.status_code"], attribute.IntValue(202))
		testutil.Equal(t, "error type", attrs["error.type"], attribute.StringValue("202"))
	})

	t.Run("should trace the requests matching no endpoint", func(t *testing.T) {
		m, recorder := traced(t)
		rec, _ := serve(t, m, "GET /items/{id}", httptest.NewRequest(http.MethodGet, "/other", nil),
			func(http.ResponseWriter, *http.Request) {})
		testutil.Equal(t, "status", rec.Code, http.StatusNotFound)

		span := only(t, recorder)
		testutil.Equal(t, "name", span.Name(), middleware.UnmatchedSpanName)
		testutil.Equal(t, "attributes", attributes(span), map[attribute.Key]attribute.Value{
			"http.request.method":         attribute.StringValue("GET"),
			"url.path":                    attribute.StringValue("/other"),
			"http.response.status_code":   attribute.IntValue(http.StatusNotFound),
			middleware.AttributeServiceId: attribute.IntValue(3),
		})
	})

	t.Run("should route by the path of the pattern", func(t *testing.T) {
		for pattern, want := range map[string]string{
			"/items/{id}":                "/items/{id}",
			"POST  /items":               "/items",
			"GET example.com/items/{id}": "/items/{id}",
			"example.com/items/{id...}":  "/items/{id...}",
			"api.example.com":            "api.example.com",
		} {
			m, recorder := traced(t)
			m.Wrap(flam.RestEndpoint{Id: 7, Pattern: pattern}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).
				ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			span := only(t, recorder)
			testutil.Equal(t, pattern+" name", span.Name(), pattern)
			testutil.Equal(t, pattern+" route", attributes(span)["http.route"], attribute.StringValue(want))
		}
	})
}
