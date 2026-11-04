package tracing

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// memoryDriver is the driver of the memory exporters of the tests.
const memoryDriver = "memory"

// memory is a tracing exporter recording the spans it exports, failing
// its shutdown with shutdownErr when not nil.
type memory struct {
	id          string
	shutdownErr error

	mu       sync.Mutex
	spans    []sdktrace.ReadOnlySpan
	shutdown int
}

func (m *memory) Id() string { return m.id }

func (m *memory) ExportSpans(_ context.Context, spans []sdktrace.ReadOnlySpan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.spans = append(m.spans, spans...)
	return nil
}

func (m *memory) Shutdown(context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.shutdown++
	return m.shutdownErr
}

// exported returns the names of the spans exported, and the number of
// shutdowns.
func (m *memory) exported() ([]string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	names := make([]string, len(m.spans))
	for i, s := range m.spans {
		names[i] = s.Name()
	}
	return names, m.shutdown
}

// last returns the last span exported.
func (m *memory) last(t *testing.T) sdktrace.ReadOnlySpan {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.spans) == 0 {
		t.Fatalf("no span exported")
	}
	return m.spans[len(m.spans)-1]
}

// memoryCreator creates the memory exporters of its map, by id, counting
// its creations, and fails for the ids it does not hold.
type memoryCreator struct {
	exporters map[string]*memory

	mu      sync.Mutex
	created int
}

func (c *memoryCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == memoryDriver
}

func (c *memoryCreator) Create(bag flam.IBag) (flam.ITracingExporter, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.created++
	exporter, ok := c.exporters[bag.String(factory.ConfigFieldId)]
	if !ok {
		return nil, errBoom
	}
	return exporter, nil
}

// newApp returns a testutil.App with the config and tracing bundles, the
// memory creator and a configuration holding the values, at paths, along
// with the memory exporter "mem", configured.
func newApp(t *testing.T, values map[string]any) (*testutil.App, *memoryCreator) {
	t.Helper()
	creator := &memoryCreator{exporters: map[string]*memory{"mem": {id: "mem"}}}
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(tracing.ExporterConfigPath, container.Bag{
		"mem":     container.Bag{factory.ConfigFieldDriver: memoryDriver},
		"missing": container.Bag{factory.ConfigFieldDriver: memoryDriver},
	}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, app.RegisterBundle(tracing.NewBundle()))
	testutil.NoError(t, app.Provide(func() flam.ITracingExporterCreator { return creator },
		dig.Group(tracing.ExporterCreatorGroup)))
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app, creator
}

// provider returns the tracer provider of the app, or the root cause of
// the error of its creation.
func provider(app *testutil.App) (trace.TracerProvider, error) {
	var p trace.TracerProvider
	err := app.Invoke(func(provided trace.TracerProvider) { p = provided })
	return p, dig.RootCause(err)
}

// sdkProvider returns the SDK tracer provider of the app, failing the test
// when it is another one.
func sdkProvider(t *testing.T, app *testutil.App) *sdktrace.TracerProvider {
	t.Helper()
	p, err := provider(app)
	testutil.NoError(t, err)
	sdk, ok := p.(*sdktrace.TracerProvider)
	if !ok {
		t.Fatalf("provider = %T, want *sdktrace.TracerProvider", p)
	}
	return sdk
}

// spanOf starts and ends the span of the name with the provider, from the
// context, returning its span context.
func spanOf(p trace.TracerProvider, ctx context.Context, name string) trace.SpanContext {
	_, span := p.Tracer("test").Start(ctx, name)
	defer span.End()
	return span.SpanContext()
}

// remote returns the context of a remote parent span, sampled or not.
func remote(sampled bool) context.Context {
	flags := trace.TraceFlags(0)
	if sampled {
		flags = trace.FlagsSampled
	}
	return trace.ContextWithRemoteSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: flags,
		Remote:     true,
	}))
}

// attributes returns the attributes of the resource of the span, by key.
func attributes(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Resource().Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

// closeApp closes the tracing bundle of the app.
func closeApp(app *testutil.App) error {
	return tracing.NewBundle().(flam.IClosableBundle).Close(app)
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the providers", func(t *testing.T) {
		b := tracing.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.tracing", "provider/0", "provider/0", "provider/0",
		})
	})

	t.Run("should fail when a provider cannot be registered", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(kind string) error {
			if kind == "provider" {
				return errBoom
			}
			return nil
		}
		if err := app.RegisterBundle(tracing.NewBundle()); !errors.Is(err, errBoom) {
			t.Errorf("RegisterBundle() = %v, want it to wrap errBoom", err)
		}
	})

	t.Run("should provide the W3C trace context and baggage propagator", func(t *testing.T) {
		app, _ := newApp(t, nil)
		var p propagation.TextMapPropagator
		testutil.NoError(t, app.Invoke(func(provided propagation.TextMapPropagator) { p = provided }))
		testutil.Equal(t, "Fields()", slices.Sorted(slices.Values(p.Fields())), []string{"baggage", "traceparent", "tracestate"})
	})

	t.Run("should provide the exporter factory of the configuration", func(t *testing.T) {
		app, _ := newApp(t, nil)
		testutil.NoError(t, app.Invoke(func(f flam.ITracingExporterFactory) {
			testutil.Equal(t, "Available()", f.Available(), []string{"mem", "missing"})
		}))
	})
}

func Test_Provider(t *testing.T) {
	t.Run("should sample every trace, following the parent, by default", func(t *testing.T) {
		app, creator := newApp(t, nil)
		p := sdkProvider(t, app)
		testutil.Equal(t, "root sampled", spanOf(p, context.Background(), "root").IsSampled(), true)
		testutil.Equal(t, "child of sampled", spanOf(p, remote(true), "child").IsSampled(), true)
		testutil.Equal(t, "child of unsampled", spanOf(p, remote(false), "child").IsSampled(), false)
		testutil.Equal(t, "created", creator.created, 0)
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should export to the exporters, in batches, with the resource", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{
			tracing.ExportConfigPath:         []any{"mem"},
			tracing.ServiceNameConfigPath:    "api",
			tracing.ServiceVersionConfigPath: "1.2.0",
			tracing.ResourceConfigPath: container.Bag{
				"service.name": "hidden",
				"team":         "core",
				"replicas":     3,
				"shard":        int64(4),
				"weight":       0.5,
				"canary":       true,
				"deployment":   map[string]any{"environment": container.Bag{"name": "production"}},
			},
		})
		p := sdkProvider(t, app)
		spanOf(p, context.Background(), "work")
		mem := creator.exporters["mem"]
		names, _ := mem.exported()
		testutil.Equal(t, "exported before flush", len(names), 0)

		testutil.NoError(t, closeApp(app))
		names, shutdowns := mem.exported()
		testutil.Equal(t, "exported", names, []string{"work"})
		testutil.Equal(t, "shutdowns", shutdowns, 1)
		attrs := attributes(mem.last(t))
		for key, want := range map[attribute.Key]attribute.Value{
			"service.name":                attribute.StringValue("api"),
			"service.version":             attribute.StringValue("1.2.0"),
			"team":                        attribute.StringValue("core"),
			"replicas":                    attribute.IntValue(3),
			"shard":                       attribute.Int64Value(4),
			"weight":                      attribute.Float64Value(0.5),
			"canary":                      attribute.BoolValue(true),
			"deployment.environment.name": attribute.StringValue("production"),
			"telemetry.sdk.name":          attribute.StringValue("opentelemetry"),
		} {
			testutil.Equal(t, string(key), attrs[key], want)
		}
	})

	t.Run("should export each span as it ends without batches", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{
			tracing.ExportConfigPath: []string{"mem"},
			tracing.BatchConfigPath:  false,
		})
		spanOf(sdkProvider(t, app), context.Background(), "work")
		names, _ := creator.exporters["mem"].exported()
		testutil.Equal(t, "exported", names, []string{"work"})
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should keep the default service name unless set", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{
			tracing.ExportConfigPath:      []string{"mem"},
			tracing.BatchConfigPath:       false,
			tracing.ServiceNameConfigPath: "",
		})
		spanOf(sdkProvider(t, app), context.Background(), "work")
		name := attributes(creator.exporters["mem"].last(t))["service.name"].AsString()
		if name == "" || name == "api" {
			t.Errorf("service.name = %q, want the default", name)
		}
		_, ok := attributes(creator.exporters["mem"].last(t))["service.version"]
		testutil.Equal(t, "has service.version", ok, false)
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should sample as configured", func(t *testing.T) {
		for name, tc := range map[string]struct {
			values map[string]any
			root   bool
			child  bool
		}{
			"never":                  {map[string]any{tracing.SamplerConfigPath: tracing.SamplerNever}, false, true},
			"never, not parent":      {map[string]any{tracing.SamplerConfigPath: tracing.SamplerNever, tracing.SamplerParentBasedConfigPath: false}, false, false},
			"always, not parent":     {map[string]any{tracing.SamplerParentBasedConfigPath: false}, true, true},
			"ratio 0":                {map[string]any{tracing.SamplerConfigPath: tracing.SamplerRatio, tracing.SamplerRatioConfigPath: 0}, false, true},
			"ratio 1":                {map[string]any{tracing.SamplerConfigPath: tracing.SamplerRatio, tracing.SamplerRatioConfigPath: 1.0}, true, true},
			"ratio default":          {map[string]any{tracing.SamplerConfigPath: tracing.SamplerRatio}, true, true},
			"ratio 0, not parent":    {map[string]any{tracing.SamplerConfigPath: tracing.SamplerRatio, tracing.SamplerRatioConfigPath: 0, tracing.SamplerParentBasedConfigPath: false}, false, false},
			"always explicitly set":  {map[string]any{tracing.SamplerConfigPath: tracing.SamplerAlways}, true, true},
			"ratio 1, parent based":  {map[string]any{tracing.SamplerConfigPath: tracing.SamplerRatio, tracing.SamplerRatioConfigPath: 1}, true, true},
			"always, parent default": {map[string]any{}, true, true},
		} {
			app, _ := newApp(t, tc.values)
			p := sdkProvider(t, app)
			testutil.Equal(t, name+" root", spanOf(p, context.Background(), "root").IsSampled(), tc.root)
			testutil.Equal(t, name+" child", spanOf(p, remote(true), "child").IsSampled(), tc.child)
			testutil.NoError(t, closeApp(app))
		}
	})

	t.Run("should provide the no-op provider when disabled", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{
			tracing.EnabledConfigPath: false,
			tracing.ExportConfigPath:  []string{"mem"},
			tracing.SamplerConfigPath: "unknown",
		})
		p, err := provider(app)
		testutil.NoError(t, err)
		testutil.Equal(t, "provider", p, trace.TracerProvider(noop.NewTracerProvider()))
		testutil.Equal(t, "created", creator.created, 0)
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		for path, value := range map[string]any{
			tracing.EnabledConfigPath:            "yes",
			tracing.SamplerConfigPath:            "sometimes",
			tracing.SamplerRatioConfigPath:       1.5,
			tracing.SamplerParentBasedConfigPath: "yes",
			tracing.ResourceConfigPath:           "team",
			tracing.ServiceNameConfigPath:        1,
			tracing.ServiceVersionConfigPath:     true,
			tracing.BatchConfigPath:              "yes",
			tracing.ExportConfigPath:             "mem",
		} {
			values := map[string]any{path: value}
			if path == tracing.SamplerRatioConfigPath {
				values[tracing.SamplerConfigPath] = tracing.SamplerRatio
			}
			app, _ := newApp(t, values)
			_, err := provider(app)
			testutil.Error(t, err, tracing.ErrInvalidField, "unable to create tracer provider: invalid tracing field",
				map[string]any{tracing.ErrorFieldField: path})
		}
	})

	t.Run("should fail on an invalid ratio", func(t *testing.T) {
		for _, ratio := range []any{-0.1, 2, "half"} {
			app, _ := newApp(t, map[string]any{
				tracing.SamplerConfigPath:      tracing.SamplerRatio,
				tracing.SamplerRatioConfigPath: ratio,
			})
			_, err := provider(app)
			testutil.Error(t, err, tracing.ErrInvalidField, "unable to create tracer provider: invalid tracing field",
				map[string]any{tracing.ErrorFieldField: tracing.SamplerRatioConfigPath})
		}
	})

	t.Run("should fail on an invalid resource attribute", func(t *testing.T) {
		for name, attrs := range map[string]container.Bag{
			"list":        {"tags": []any{"a"}},
			"nil":         {"tag": nil},
			"nested list": {"deployment": container.Bag{"tags": []string{"a"}}},
		} {
			app, _ := newApp(t, map[string]any{tracing.ResourceConfigPath: attrs})
			_, err := provider(app)
			if !errors.Is(err, tracing.ErrInvalidField) {
				t.Fatalf("%s: error = %v, want ErrInvalidField", name, err)
			}
			testutil.Context(t, err.(*flam.Error), map[string]any{tracing.ErrorFieldField: tracing.ResourceConfigPath})
		}
	})

	t.Run("should fail when an exporter cannot be created", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{tracing.ExportConfigPath: []string{"mem", "missing"}})
		_, err := provider(app)
		testutil.Error(t, err, tracing.ErrExporter, err.Error(), map[string]any{tracing.ErrorFieldID: "missing"})
		if !errors.Is(err, errBoom) {
			t.Errorf("error = %v, want it to wrap errBoom", err)
		}
		testutil.Equal(t, "created", creator.created, 2)
	})

	t.Run("should fail when the exporter factory cannot be created", func(t *testing.T) {
		app, _ := newApp(t, nil)
		testutil.NoError(t, app.Provide(func() flam.ITracingExporterCreator { return nil },
			dig.Group(tracing.ExporterCreatorGroup)))
		if _, err := provider(app); !errors.Is(err, factory.ErrNilCreator) {
			t.Errorf("error = %v, want it to wrap factory.ErrNilCreator", err)
		}
	})
}

func Test_Bundle_Boot(t *testing.T) {
	t.Run("should set the global provider and propagator", func(t *testing.T) {
		app, _ := newApp(t, nil)
		testutil.NoError(t, tracing.NewBundle().(flam.IBootableBundle).Boot(app))
		p, _ := provider(app)
		testutil.Equal(t, "global provider", otel.GetTracerProvider(), p)
		testutil.Equal(t, "global propagator fields", slices.Sorted(slices.Values(otel.GetTextMapPropagator().Fields())),
			[]string{"baggage", "traceparent", "tracestate"})
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator())
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should leave the globals when not global", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{tracing.GlobalConfigPath: false})
		before := otel.GetTracerProvider()
		testutil.NoError(t, tracing.NewBundle().(flam.IBootableBundle).Boot(app))
		testutil.Equal(t, "global provider", otel.GetTracerProvider(), before)
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should fail on an invalid global flag", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{tracing.GlobalConfigPath: "yes"})
		err := dig.RootCause(tracing.NewBundle().(flam.IBootableBundle).Boot(app))
		testutil.Error(t, err, tracing.ErrInvalidField, "unable to boot tracing: invalid tracing field",
			map[string]any{tracing.ErrorFieldField: tracing.GlobalConfigPath})
	})

	t.Run("should fail when the provider cannot be created", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{tracing.SamplerConfigPath: "sometimes"})
		err := tracing.NewBundle().(flam.IBootableBundle).Boot(app)
		if !errors.Is(err, tracing.ErrInvalidField) {
			t.Errorf("Boot() = %v, want it to wrap ErrInvalidField", err)
		}
	})
}

func Test_Bundle_Close(t *testing.T) {
	t.Run("should shut the provider down, then close the exporter factory", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{tracing.ExportConfigPath: []string{"mem"}})
		p := sdkProvider(t, app)
		testutil.NoError(t, closeApp(app))
		_, shutdowns := creator.exporters["mem"].exported()
		testutil.Equal(t, "shutdowns", shutdowns, 1)
		_, span := p.Tracer("test").Start(context.Background(), "late")
		testutil.Equal(t, "recording after shutdown", span.IsRecording(), false)
		testutil.NoError(t, app.Invoke(func(f flam.ITracingExporterFactory) {
			if _, err := f.Get("mem"); !errors.Is(err, factory.ErrClosed) {
				t.Errorf("Get() = %v, want ErrClosed", err)
			}
		}))
	})

	t.Run("should shut down without timeout when 0", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{
			tracing.ExportConfigPath:          []string{"mem"},
			tracing.ShutdownTimeoutConfigPath: "0s",
		})
		sdkProvider(t, app)
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should close the no-op provider", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{tracing.EnabledConfigPath: false})
		testutil.NoError(t, closeApp(app))
	})

	t.Run("should fail when the shutdown fails", func(t *testing.T) {
		app, creator := newApp(t, map[string]any{
			tracing.ExportConfigPath:          []string{"mem"},
			tracing.ShutdownTimeoutConfigPath: time.Second,
		})
		creator.exporters["mem"].shutdownErr = errBoom
		err := closeApp(app)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want it to wrap errBoom", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) || !errors.Is(e, tracing.ErrShutdown) {
			t.Fatalf("Close() = %v, want a *flam.Error wrapping ErrShutdown", err)
		}
		testutil.Equal(t, "message", e.Error(), "unable to close tracing: unable to shut down tracer provider: boom")
	})

	t.Run("should fail on an invalid timeout, closing the exporter factory", func(t *testing.T) {
		app, _ := newApp(t, map[string]any{tracing.ShutdownTimeoutConfigPath: "soon"})
		err := closeApp(app)
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Close() = %v, want a *flam.Error", err)
		}
		testutil.Error(t, e, tracing.ErrInvalidField, "unable to close tracing: invalid tracing field",
			map[string]any{tracing.ErrorFieldField: tracing.ShutdownTimeoutConfigPath})
		testutil.NoError(t, app.Invoke(func(f flam.ITracingExporterFactory) {
			testutil.Equal(t, "Available()", f.Available(), []string(nil))
		}))
	})
}
