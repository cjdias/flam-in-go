package interceptors

import (
	"context"
	"errors"
	"sort"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/tracing"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// traceparent is the W3C trace context of a remote parent span.
const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

// tracingWith returns the tracing interceptor "main", with the provider
// and the propagator when not nil.
func tracingWith(t *testing.T, fields container.Bag, provider trace.TracerProvider, propagator propagation.TextMapPropagator) (flam.IGrpcInterceptor, error) {
	t.Helper()
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = tracing.Driver
	}
	var providers []any
	if provider != nil {
		providers = append(providers, func() trace.TracerProvider { return provider })
	}
	if propagator != nil {
		providers = append(providers, func() propagation.TextMapPropagator { return propagator })
	}
	return create(t, tracing.NewBundle(), fields, nil, providers...)
}

// traced returns a tracing interceptor of the propagator, recording its
// spans in the recorder returned.
func traced(t *testing.T, propagator propagation.TextMapPropagator) (flam.IGrpcInterceptor, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	i, err := tracingWith(t, container.Bag{}, sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)), propagator)
	testutil.NoError(t, err)
	return i, recorder
}

// traceContext is the propagator of the W3C trace context.
var traceContext = propagation.TraceContext{}

// spanAttributes returns the attributes of the span, by key, as text.
func spanAttributes(span sdktrace.ReadOnlySpan) map[attribute.Key]string {
	all := map[attribute.Key]string{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value.String()
	}
	return all
}

// incoming returns the context of the incoming metadata holding the trace
// context of the remote parent span.
func incoming() context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("traceparent", traceparent))
}

// keys is a propagator recording the keys of the carrier it extracts from,
// after setting a key.
type keys struct {
	propagation.TraceContext

	keys []string
}

func (k *keys) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	carrier.Set("Set", "value")
	k.keys = carrier.Keys()
	sort.Strings(k.keys)
	return ctx
}

func Test_Tracing_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":           {tracing.Id, "tracing"},
		"Driver":       {tracing.Driver, "flam.grpc.interceptors.driver.tracing"},
		"ScopeName":    {tracing.ScopeName, "github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/tracing"},
		"ErrorFieldID": {tracing.ErrorFieldID, "id"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	testutil.Equal(t, "ErrMissingTracing", tracing.ErrMissingTracing.Error(), "tracer provider or propagator not provided by the container")
	testutil.Equal(t, "ErrPanicked", tracing.ErrPanicked.Error(), "grpc handler panicked")
}

func Test_Tracing_Bundle(t *testing.T) {
	b := tracing.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.interceptors.tracing")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	testutil.Equal(t, "interceptor", bag.Get(flamgrpc.InterceptorConfigPath+"."+tracing.Id),
		any(container.Bag{factory.ConfigFieldDriver: tracing.Driver}))
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}
}

func Test_Tracing_Interceptor(t *testing.T) {
	t.Run("should serve a unary call within a server span, child of the remote span", func(t *testing.T) {
		i, recorder := traced(t, traceContext)
		testutil.Equal(t, "Id()", i.Id(), "main")
		var inner trace.SpanContext
		resp, err := i.Unary()(incoming(), "req", &grpc.UnaryServerInfo{FullMethod: fullMethod}, func(ctx context.Context, _ any) (any, error) {
			inner = trace.SpanContextFromContext(ctx)
			return "resp", nil
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "resp", resp, "resp")

		spans := recorder.Ended()
		testutil.Equal(t, "spans", len(spans), 1)
		span := spans[0]
		testutil.Equal(t, "name", span.Name(), "pkg.Users/Get")
		testutil.Equal(t, "kind", span.SpanKind(), trace.SpanKindServer)
		testutil.Equal(t, "scope", span.InstrumentationScope().Name, tracing.ScopeName)
		testutil.Equal(t, "trace", span.SpanContext().TraceID().String(), "0af7651916cd43dd8448eb211c80319c")
		testutil.Equal(t, "parent", span.Parent().SpanID().String(), "b7ad6b7169203331")
		testutil.Equal(t, "inner", inner, span.SpanContext())
		testutil.Equal(t, "attributes", spanAttributes(span), map[attribute.Key]string{
			"rpc.system.name":          "grpc",
			"rpc.method":               "pkg.Users/Get",
			"rpc.response.status_code": "OK",
		})
		testutil.Equal(t, "status", span.Status().Code, otelcodes.Unset)
	})

	t.Run("should serve a streaming call within a server span, held by the context of its stream", func(t *testing.T) {
		i, recorder := traced(t, traceContext)
		var inner trace.SpanContext
		err := i.Stream()("srv", stream{ctx: incoming()}, &grpc.StreamServerInfo{FullMethod: fullMethod}, func(_ any, ss grpc.ServerStream) error {
			inner = trace.SpanContextFromContext(ss.Context())
			return status.Error(codes.Unavailable, "down")
		})
		testutil.Equal(t, "code", status.Code(err), codes.Unavailable)
		span := recorder.Ended()[0]
		testutil.Equal(t, "inner", inner, span.SpanContext())
		testutil.Equal(t, "parent", span.Parent().SpanID().String(), "b7ad6b7169203331")
		testutil.Equal(t, "attributes", spanAttributes(span), map[attribute.Key]string{
			"rpc.system.name":          "grpc",
			"rpc.method":               "pkg.Users/Get",
			"rpc.response.status_code": "UNAVAILABLE",
			"error.type":               "UNAVAILABLE",
		})
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: otelcodes.Error, Description: "UNAVAILABLE"})
	})

	t.Run("should start a root span without metadata", func(t *testing.T) {
		i, recorder := traced(t, traceContext)
		_, _ = unary(i, context.Background(), nil, nil, nil, false)
		testutil.Equal(t, "parent", recorder.Ended()[0].Parent().IsValid(), false)
	})

	for name, tc := range map[string]struct {
		err   error
		code  string
		error bool
	}{
		"CANCELLED":           {status.Error(codes.Canceled, ""), "CANCELLED", false},
		"NOT_FOUND":           {status.Error(codes.NotFound, ""), "NOT_FOUND", false},
		"INVALID_ARGUMENT":    {status.Error(codes.InvalidArgument, ""), "INVALID_ARGUMENT", false},
		"UNAUTHENTICATED":     {status.Error(codes.Unauthenticated, ""), "UNAUTHENTICATED", false},
		"DEADLINE_EXCEEDED":   {status.Error(codes.DeadlineExceeded, ""), "DEADLINE_EXCEEDED", true},
		"INTERNAL":            {status.Error(codes.Internal, ""), "INTERNAL", true},
		"UNKNOWN of an error": {errBoom, "UNKNOWN", true},
		"an unknown code":     {status.Error(codes.Code(99), ""), "99", false},
	} {
		t.Run("should record the code "+name, func(t *testing.T) {
			i, recorder := traced(t, traceContext)
			_, err := unary(i, context.Background(), nil, nil, tc.err, false)
			testutil.Equal(t, "error", err, tc.err)
			span := recorder.Ended()[0]
			attributes := spanAttributes(span)
			testutil.Equal(t, "code", attributes["rpc.response.status_code"], tc.code)
			_, typed := attributes["error.type"]
			testutil.Equal(t, "error type", typed, tc.error)
			testutil.Equal(t, "error status", span.Status().Code == otelcodes.Error, tc.error)
		})
	}

	t.Run("should record the panics of the handlers, the panic going on", func(t *testing.T) {
		i, recorder := traced(t, traceContext)
		testutil.Panics(t, "unary", func() { _, _ = unary(i, context.Background(), nil, nil, nil, true) })
		testutil.Panics(t, "stream", func() { _ = streaming(i, context.Background(), nil, nil, true) })
		spans := recorder.Ended()
		testutil.Equal(t, "spans", len(spans), 2)
		for _, span := range spans {
			testutil.Equal(t, "code", spanAttributes(span)["rpc.response.status_code"], "INTERNAL")
			testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: otelcodes.Error, Description: "INTERNAL"})
			events := span.Events()
			testutil.Equal(t, "events", len(events), 1)
			testutil.Equal(t, "event", events[0].Name, "exception")
			var message string
			for _, kv := range events[0].Attributes {
				if kv.Key == "exception.message" {
					message = kv.Value.AsString()
				}
			}
			testutil.Equal(t, "exception", message, "grpc handler panicked")
		}
	})

	t.Run("should give the propagator a carrier of the metadata", func(t *testing.T) {
		k := &keys{}
		i, _ := traced(t, k)
		md := metadata.Pairs("traceparent", traceparent, "b", "1")
		_, _ = unary(i, metadata.NewIncomingContext(context.Background(), md), nil, nil, nil, false)
		testutil.Equal(t, "keys", k.keys, []string{"b", "set", "traceparent"})
	})

	t.Run("should fail without tracer provider or propagator", func(t *testing.T) {
		for name, tc := range map[string]struct {
			provider   trace.TracerProvider
			propagator propagation.TextMapPropagator
		}{
			"neither":       {nil, nil},
			"no propagator": {sdktrace.NewTracerProvider(), nil},
			"no provider":   {nil, traceContext},
		} {
			_, err := tracingWith(t, container.Bag{}, tc.provider, tc.propagator)
			if !errors.Is(err, tracing.ErrMissingTracing) {
				t.Fatalf("%s: Get() = %v, want ErrMissingTracing", name, err)
			}
			testutil.Error(t, errors.Unwrap(err), tracing.ErrMissingTracing,
				"unable to create tracing grpc interceptor: tracer provider or propagator not provided by the container",
				map[string]any{tracing.ErrorFieldID: "main"})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := tracingWith(t, container.Bag{factory.ConfigFieldDriver: "other"}, sdktrace.NewTracerProvider(), traceContext)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
