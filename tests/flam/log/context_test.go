package log

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ctxKey is the key of the value fakeExtractor extracts from a context.
type ctxKey struct{}

// fakeExtractor is a flam.ILogContextExtractor extracting, at its field,
// the value of a context at ctxKey, when it holds one.
type fakeExtractor struct {
	field string
}

func (e *fakeExtractor) Extract(ctx context.Context) flam.IBag {
	if v := ctx.Value(ctxKey{}); v != nil {
		return container.Bag{e.field: v}
	}
	return nil
}

// withExtractors returns the option providing the extractors in the
// extractor group.
func withExtractors(t *testing.T, extractors ...flam.ILogContextExtractor) func(*testutil.App) {
	t.Helper()
	return func(app *testutil.App) {
		for _, e := range extractors {
			testutil.NoError(t, app.Provide(func() flam.ILogContextExtractor { return e }, dig.Group(log.ContextExtractorGroup)))
		}
	}
}

// contextLogger returns the context logger of the container.
func (a *logApp) contextLogger(t *testing.T) flam.IContextLogger {
	t.Helper()
	var l flam.IContextLogger
	testutil.NoError(t, a.Invoke(func(logger flam.IContextLogger) { l = logger }))
	return l
}

// extractors returns the extractors of the container.
func (a *logApp) extractors(t *testing.T) []flam.ILogContextExtractor {
	t.Helper()
	var list []flam.ILogContextExtractor
	testutil.NoError(t, a.Invoke(func(in struct {
		dig.In
		Extractors []flam.ILogContextExtractor `group:"flam.log.context.extractors"`
	}) {
		list = in.Extractors
	}))
	return list
}

// spanContext returns a context holding a span context of the trace and
// span ids.
func spanContext(t *testing.T, traceId, spanId string) context.Context {
	t.Helper()
	cfg := trace.SpanContextConfig{}
	if traceId != "" {
		id, err := trace.TraceIDFromHex(traceId)
		testutil.NoError(t, err)
		cfg.TraceID = id
	}
	if spanId != "" {
		id, err := trace.SpanIDFromHex(spanId)
		testutil.NoError(t, err)
		cfg.SpanID = id
	}
	return trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(cfg))
}

const (
	traceHex = "0102030405060708090a0b0c0d0e0f10"
	spanHex  = "0102030405060708"
)

func Test_ContextLogger(t *testing.T) {
	t.Run("should have its group and fields", func(t *testing.T) {
		testutil.Equal(t, "group", log.ContextExtractorGroup, "flam.log.context.extractors")
		testutil.Equal(t, "trace path", log.TraceIdFieldConfigPath, "flam.log.context.trace_id")
		testutil.Equal(t, "span path", log.SpanIdFieldConfigPath, "flam.log.context.span_id")
		testutil.Equal(t, "trace field", log.ContextFieldTraceId, "trace_id")
		testutil.Equal(t, "span field", log.ContextFieldSpanId, "span_id")
	})

	t.Run("should provide the logger as the context logger", func(t *testing.T) {
		a := newLogApp(t, container.Bag{}, nil)
		if any(a.contextLogger(t)) != any(a.logger(t)) {
			t.Fatal("the container did not provide the ILogger as the IContextLogger")
		}
	})

	t.Run("should signal and broadcast at each level with the extracted fields", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil, withExtractors(t, &fakeExtractor{field: "x"}))
		testutil.NoError(t, a.boot())
		l := a.contextLogger(t)
		ctx := context.WithValue(context.Background(), ctxKey{}, "v")
		l.SignalContext(ctx, flam.LogNone, "ch", "signal")
		l.SignalFatalContext(ctx, "ch", "signal fatal")
		l.SignalErrorContext(ctx, "ch", "signal error")
		l.SignalWarningContext(ctx, "ch", "signal warning")
		l.SignalNoticeContext(ctx, "ch", "signal notice")
		l.SignalInfoContext(ctx, "ch", "signal info")
		l.SignalDebugContext(ctx, "ch", "signal debug")
		l.BroadcastContext(ctx, flam.LogNone, "broadcast")
		l.BroadcastFatalContext(ctx, "broadcast fatal")
		l.BroadcastErrorContext(ctx, "broadcast error")
		l.BroadcastWarningContext(ctx, "broadcast warning")
		l.BroadcastNoticeContext(ctx, "broadcast notice")
		l.BroadcastInfoContext(ctx, "broadcast info")
		l.BroadcastDebugContext(ctx, "broadcast debug")
		testutil.NoError(t, l.Flush())

		levels := []flam.LogLevel{
			flam.LogNone, flam.LogFatal, flam.LogError, flam.LogWarning, flam.LogNotice, flam.LogInfo, flam.LogDebug,
		}
		want := []written{}
		for _, broadcast := range []bool{false, true} {
			for _, level := range levels {
				w := written{
					timestamp: now, level: level, broadcast: broadcast, channel: "ch", message: "signal",
					ctx: []flam.IBag{container.Bag{"x": "v"}},
				}
				if broadcast {
					w.channel, w.message = "", "broadcast"
				}
				if level != flam.LogNone {
					w.message += " " + level.String()
				}
				want = append(want, w)
			}
		}
		testutil.Equal(t, "written", a.streams.get("a").written(), want)
	})

	t.Run("should merge the fields of every extractor, the bags given winning", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil,
			withExtractors(t, &fakeExtractor{field: "x"}, &fakeExtractor{field: "y"}))
		testutil.NoError(t, a.boot())
		l := a.contextLogger(t)
		ctx := context.WithValue(spanContext(t, traceHex, spanHex), ctxKey{}, "v")
		l.SignalInfoContext(ctx, "ch", "m", container.Bag{"y": "given"}, nil)
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "ctx", got[0].ctx, []flam.IBag{container.Bag{
			"x": "v", "y": "given", log.ContextFieldTraceId: traceHex, log.ContextFieldSpanId: spanHex,
		}})
	})

	t.Run("should skip the extractors extracting nothing", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil, withExtractors(t, &fakeExtractor{field: "x"}))
		testutil.NoError(t, a.boot())
		l := a.contextLogger(t)
		l.SignalInfoContext(context.Background(), "ch", "empty")
		l.BroadcastInfoContext(context.Background(), "given", container.Bag{"a": 1})
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "empty", len(got[0].ctx), 0)
		testutil.Equal(t, "given", got[1].ctx, []flam.IBag{container.Bag{"a": 1}})
	})

	t.Run("should extract nothing from a nil context", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil, withExtractors(t, &fakeExtractor{field: "x"}))
		testutil.NoError(t, a.boot())
		l := a.contextLogger(t)
		//lint:ignore SA1012 a nil context, carrying no field, is what is tested
		l.SignalInfoContext(nil, "ch", "m", container.Bag{"a": 1}) //nolint:staticcheck // as above
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "ctx", got[0].ctx, []flam.IBag{container.Bag{"a": 1}})
	})

	t.Run("should keep the entries of the context free methods without extracted field", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil, withExtractors(t, &fakeExtractor{field: "x"}))
		testutil.NoError(t, a.boot())
		l := a.contextLogger(t)
		l.SignalInfo("ch", "m")
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "ctx", len(a.streams.get("a").written()[0].ctx), 0)
	})
}

func Test_TraceExtractor(t *testing.T) {
	// extractor returns the trace extractor of an application of the
	// defaults.
	extractor := func(t *testing.T, defaults container.Bag) flam.ILogContextExtractor {
		t.Helper()
		list := newLogApp(t, defaults, nil).extractors(t)
		testutil.Equal(t, "extractors", len(list), 1)
		return list[0]
	}

	t.Run("should extract the trace and span ids", func(t *testing.T) {
		got := extractor(t, container.Bag{}).Extract(spanContext(t, traceHex, spanHex))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"trace_id": traceHex, "span_id": spanHex}))
	})

	t.Run("should extract the ids at the configured fields, leaving out the empty ones", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(log.TraceIdFieldConfigPath, "trace"))
		testutil.NoError(t, bag.Set(log.SpanIdFieldConfigPath, ""))
		got := extractor(t, bag).Extract(spanContext(t, traceHex, spanHex))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"trace": traceHex}))

		bag = container.Bag{}
		testutil.NoError(t, bag.Set(log.TraceIdFieldConfigPath, ""))
		testutil.NoError(t, bag.Set(log.SpanIdFieldConfigPath, "span"))
		got = extractor(t, bag).Extract(spanContext(t, traceHex, spanHex))
		testutil.Equal(t, "fields", got, flam.IBag(container.Bag{"span": spanHex}))
	})

	t.Run("should leave out the invalid ids", func(t *testing.T) {
		e := extractor(t, container.Bag{})
		testutil.Equal(t, "trace only", e.Extract(spanContext(t, traceHex, "")), flam.IBag(container.Bag{"trace_id": traceHex}))
		testutil.Equal(t, "span only", e.Extract(spanContext(t, "", spanHex)), flam.IBag(container.Bag{"span_id": spanHex}))
		testutil.Equal(t, "none", e.Extract(context.Background()), flam.IBag(container.Bag{}))
	})

	t.Run("should extract nothing from a nil context", func(t *testing.T) {
		//lint:ignore SA1012 a nil context is what is tested
		testutil.Equal(t, "fields", extractor(t, container.Bag{}).Extract(nil), flam.IBag(nil)) //nolint:staticcheck // as above
	})
}
