package redis

import (
	"context"
	"errors"
	"net"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamredis "github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing/redis"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// recorded returns a tracer provider recording its spans in the recorder
// returned.
func recorded() (trace.TracerProvider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	return sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)), recorder
}

// newApp returns a testutil.App with the config, redis, mini connection
// and tracing redis bundles, the tracer provider when not nil, and the
// mini connection "main".
func newApp(t *testing.T, provider trace.TracerProvider) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), flamredis.NewBundle(), mini.NewBundle(), redis.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamredis.ConnectionConfigPath+".main", container.Bag{factory.ConfigFieldDriver: mini.Driver}))
	app := testutil.NewApp()
	if provider != nil {
		testutil.NoError(t, app.Provide(func() trace.TracerProvider { return provider }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() { _ = flamredis.NewBundle().(flam.IClosableBundle).Close(app) })
	return app
}

// connection returns the connection "main" of the connection factory of
// the app, or the error of its creation.
func connection(app *testutil.App) (flam.IRedisConnection, error) {
	var conn flam.IRedisConnection
	var err error
	if invokeErr := app.Invoke(func(f flam.IRedisConnectionFactory) { conn, err = f.Get("main") }); invokeErr != nil {
		return nil, invokeErr
	}
	return conn, err
}

// traced returns the connection "main" of an app tracing with a recorded
// provider, along with the spans it ended once ready, its pool holding a
// connection, so that the commands initializing a connection, such as
// HELLO, are left out.
func traced(t *testing.T) (flam.IRedisConnection, *ended) {
	t.Helper()
	provider, recorder := recorded()
	conn, err := connection(newApp(t, provider))
	testutil.NoError(t, err)
	testutil.NoError(t, conn.Ping(context.Background()).Err())
	return conn, &ended{recorder: recorder, skip: len(recorder.Ended())}
}

// ended reads the spans of a recorder, but the first ones skipped.
type ended struct {
	recorder *tracetest.SpanRecorder
	skip     int
}

// Ended returns the spans ended, but the first ones skipped.
func (e *ended) Ended() []sdktrace.ReadOnlySpan {
	return e.recorder.Ended()[e.skip:]
}

// attributes returns the attributes of the span, by key.
func attributes(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	all := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		all[kv.Key] = kv.Value
	}
	return all
}

// names returns the names of the spans ended.
func names(recorder interface {
	Ended() []sdktrace.ReadOnlySpan
}) []string {
	var all []string
	for _, span := range recorder.Ended() {
		all = append(all, span.Name())
	}
	return all
}

// last returns the last span ended, the pipeline of the initialization of
// a new connection of the pool, if any, ending before.
func last(t *testing.T, recorder interface {
	Ended() []sdktrace.ReadOnlySpan
}) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("no span ended")
	}
	return spans[len(spans)-1]
}

func Test_Hook(t *testing.T) {
	t.Run("should trace the commands", func(t *testing.T) {
		conn, recorder := traced(t)
		ctx := context.Background()
		testutil.NoError(t, conn.Set(ctx, "key", "value", 0).Err())
		value, err := conn.Get(ctx, "key").Result()
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, "value")

		testutil.Equal(t, "spans", names(recorder), []string{"redis.set", "redis.get"})
		span := recorder.Ended()[1]
		testutil.Equal(t, "kind", span.SpanKind(), trace.SpanKindClient)
		testutil.Equal(t, "scope", span.InstrumentationScope().Name, redis.ScopeName)
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
		testutil.Equal(t, "attributes", attributes(span), map[attribute.Key]attribute.Value{
			"db.system.name":    attribute.StringValue("redis"),
			"db.operation.name": attribute.StringValue("get"),
		})
	})

	t.Run("should be the child of the span of the command context", func(t *testing.T) {
		provider, recorder := recorded()
		conn, err := connection(newApp(t, provider))
		testutil.NoError(t, err)
		testutil.NoError(t, conn.Ping(context.Background()).Err())
		skip := len(recorder.Ended())
		ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
		testutil.NoError(t, conn.Ping(ctx).Err())
		parent.End()
		span := recorder.Ended()[skip]
		testutil.Equal(t, "parent", span.Parent().SpanID(), parent.SpanContext().SpanID())
	})

	t.Run("should not record a missing key", func(t *testing.T) {
		conn, recorder := traced(t)
		if err := conn.Get(context.Background(), "missing").Err(); !errors.Is(err, goredis.Nil) {
			t.Fatalf("Get() = %v, want redis.Nil", err)
		}
		testutil.Equal(t, "status", recorder.Ended()[0].Status().Code, codes.Unset)
	})

	t.Run("should record the errors", func(t *testing.T) {
		conn, recorder := traced(t)
		err := conn.Do(context.Background(), "NOSUCHCOMMAND").Err()
		if err == nil {
			t.Fatalf("Do() = nil, want an error")
		}
		span := recorder.Ended()[0]
		testutil.Equal(t, "name", span.Name(), "redis.nosuchcommand")
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: err.Error()})
		testutil.Equal(t, "events", len(span.Events()), 1)
	})

	t.Run("should trace the pipelines", func(t *testing.T) {
		conn, recorder := traced(t)
		ctx := context.Background()
		_, err := conn.Pipelined(ctx, func(p goredis.Pipeliner) error {
			p.Set(ctx, "a", "1", 0)
			p.Incr(ctx, "a")
			return nil
		})
		testutil.NoError(t, err)
		span := last(t, recorder)
		testutil.Equal(t, "name", span.Name(), "redis.pipeline")
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
		testutil.Equal(t, "attributes", attributes(span), map[attribute.Key]attribute.Value{
			"db.system.name":          attribute.StringValue("redis"),
			"db.operation.name":       attribute.StringValue(redis.OperationPipeline),
			"db.operation.batch.size": attribute.IntValue(2),
		})
	})

	t.Run("should record the errors of the pipelines", func(t *testing.T) {
		conn, recorder := traced(t)
		ctx := context.Background()
		_, err := conn.Pipelined(ctx, func(p goredis.Pipeliner) error {
			p.Set(ctx, "a", "x", 0)
			p.Incr(ctx, "a")
			return nil
		})
		if err == nil {
			t.Fatalf("Pipelined() = nil, want an error")
		}
		span := last(t, recorder)
		testutil.Equal(t, "name", span.Name(), "redis.pipeline")
		testutil.Equal(t, "status", span.Status().Code, codes.Error)
	})

	t.Run("should record the errors of the pipelines after a missing key", func(t *testing.T) {
		conn, recorder := traced(t)
		ctx := context.Background()
		testutil.NoError(t, conn.Set(ctx, "a", "x", 0).Err())
		var incr *goredis.IntCmd
		_, err := conn.Pipelined(ctx, func(p goredis.Pipeliner) error {
			p.Get(ctx, "missing")
			incr = p.Incr(ctx, "a")
			return nil
		})
		if !errors.Is(err, goredis.Nil) || incr.Err() == nil {
			t.Fatalf("Pipelined() = %v, %v, want redis.Nil and an INCR error", err, incr.Err())
		}
		span := last(t, recorder)
		testutil.Equal(t, "name", span.Name(), "redis.pipeline")
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: incr.Err().Error()})
		testutil.Equal(t, "events", len(span.Events()), 1)
	})

	t.Run("should not record the pipelines of missing keys only", func(t *testing.T) {
		conn, recorder := traced(t)
		ctx := context.Background()
		_, err := conn.Pipelined(ctx, func(p goredis.Pipeliner) error {
			p.Get(ctx, "missing")
			p.Get(ctx, "other")
			return nil
		})
		if !errors.Is(err, goredis.Nil) {
			t.Fatalf("Pipelined() = %v, want redis.Nil", err)
		}
		span := last(t, recorder)
		testutil.Equal(t, "name", span.Name(), "redis.pipeline")
		testutil.Equal(t, "status", span.Status().Code, codes.Unset)
	})

	t.Run("should record the error of a pipeline of no failing command", func(t *testing.T) {
		provider, recorder := recorded()
		process := redis.NewHook(provider).ProcessPipelineHook(func(context.Context, []goredis.Cmder) error { return errBoom })
		err := process(context.Background(), []goredis.Cmder{goredis.NewCmd(context.Background(), "ping")})
		testutil.Equal(t, "err", err, errBoom)
		span := last(t, recorder)
		testutil.Equal(t, "status", span.Status(), sdktrace.Status{Code: codes.Error, Description: errBoom.Error()})
	})

	t.Run("should not trace the dials", func(t *testing.T) {
		provider, recorder := recorded()
		h := redis.NewHook(provider)
		dialed := false
		dial := h.DialHook(func(context.Context, string, string) (net.Conn, error) {
			dialed = true
			return nil, errBoom
		})
		_, err := dial(context.Background(), "tcp", "127.0.0.1:1")
		testutil.Equal(t, "err", err, errBoom)
		testutil.Equal(t, "dialed", dialed, true)
		testutil.Equal(t, "spans", len(recorder.Ended()), 0)
	})

	t.Run("should use the global provider when nil", func(t *testing.T) {
		provider, recorder := recorded()
		global := otel.GetTracerProvider()
		otel.SetTracerProvider(provider)
		defer otel.SetTracerProvider(global)
		h := redis.NewHook(nil)
		process := h.ProcessHook(func(context.Context, goredis.Cmder) error { return nil })
		testutil.NoError(t, process(context.Background(), goredis.NewCmd(context.Background(), "ping")))
		testutil.Equal(t, "spans", names(recorder), []string{"redis.ping"})
	})
}

// stub is a connection creator of the tests, returning its connection and
// error.
type stub struct {
	conn flam.IRedisConnection
	err  error
}

func (s stub) Accept(flam.IBag) bool {
	return true
}

func (s stub) Create(flam.IBag) (flam.IRedisConnection, error) {
	return s.conn, s.err
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the decorator", func(t *testing.T) {
		b := redis.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.tracing.redis")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.tracing.redis", "decorator/0"})
	})

	t.Run("should trace with the provider of the tracing bundle", func(t *testing.T) {
		app := newApp(t, nil)
		testutil.NoError(t, app.RegisterBundle(tracing.NewBundle()))
		conn, err := connection(app)
		testutil.NoError(t, err)
		testutil.NoError(t, conn.Ping(context.Background()).Err())
	})

	t.Run("should fail without tracer provider", func(t *testing.T) {
		if _, err := connection(newApp(t, nil)); err == nil {
			t.Errorf("connection() = nil error, want one")
		}
	})

	t.Run("should return the creation failures as they are", func(t *testing.T) {
		provider, _ := recorded()
		for name, s := range map[string]stub{
			"error": {err: errBoom},
			"nil":   {},
		} {
			app := testutil.NewApp()
			testutil.NoError(t, app.Provide(func() trace.TracerProvider { return provider }))
			testutil.NoError(t, app.Provide(func() flam.IRedisConnectionCreator { return s },
				dig.Group(flamredis.ConnectionCreatorGroup)))
			testutil.NoError(t, app.RegisterBundle(redis.NewBundle()))
			testutil.NoError(t, app.Invoke(func(args struct {
				dig.In
				Creators []flam.IRedisConnectionCreator `group:"flam.redis.connections.creator"`
			}) {
				testutil.Equal(t, name+" accept", args.Creators[0].Accept(container.Bag{}), true)
				conn, err := args.Creators[0].Create(container.Bag{})
				testutil.Equal(t, name+" conn", conn, s.conn)
				testutil.Equal(t, name+" err", err, s.err)
			}))
		}
	})
}
