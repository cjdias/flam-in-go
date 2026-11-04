package redis

import (
	"context"
	"errors"

	goredis "github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// NewHook returns the go-redis hook tracing the commands of the clients it
// is added to, with AddHook, with the tracer of [ScopeName] of the
// provider, or of the global provider of OpenTelemetry when nil, read when
// the hook is created.
//
// Each command runs within a client span, a child of the span of the
// context of the command, named by the name of the command, such as
// "redis.get", and each pipeline, or transaction, within a span named
// "redis.pipeline", holding the number of its commands. The spans hold the
// attributes of the database semantic conventions of OpenTelemetry: the
// "redis" system and the operation, the name of the command, its
// arguments, which may hold private data, being left out. A command
// failing, other than with the redis.Nil reply of a missing key, records
// its error and sets the status of the span to an error, as does a
// pipeline of a failing command, with the error of its first command
// failing other than with redis.Nil.
func NewHook(
	provider trace.TracerProvider,
) goredis.Hook {
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	return &hook{tracer: provider.Tracer(ScopeName)}
}

// hook is the go-redis hook returned by NewHook.
type hook struct {
	tracer trace.Tracer
}

// compile time check that *hook implements Hook
var _ goredis.Hook = (*hook)(nil)

// DialHook returns next, as the connections are not traced.
func (h *hook) DialHook(
	next goredis.DialHook,
) goredis.DialHook {
	return next
}

// ProcessHook returns the hook running each command within a span.
func (h *hook) ProcessHook(
	next goredis.ProcessHook,
) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error {
		ctx, span := h.start(ctx, cmd.Name())
		defer span.End()
		err := next(ctx, cmd)
		h.end(span, err)
		return err
	}
}

// ProcessPipelineHook returns the hook running each pipeline within a
// span.
func (h *hook) ProcessPipelineHook(
	next goredis.ProcessPipelineHook,
) goredis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []goredis.Cmder) error {
		ctx, span := h.start(ctx, OperationPipeline, semconv.DBOperationBatchSize(len(cmds)))
		defer span.End()
		err := next(ctx, cmds)
		h.end(span, pipelineErr(cmds, err))
		return err
	}
}

// start starts the span of the operation, with the attributes.
func (h *hook) start(
	ctx context.Context,
	operation string,
	attributes ...attribute.KeyValue,
) (context.Context, trace.Span) {
	return h.tracer.Start(ctx, SpanNamePrefix+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(semconv.DBSystemNameRedis, semconv.DBOperationName(operation)),
		trace.WithAttributes(attributes...),
	)
}

// end records the error on the span, unless nil or redis.Nil.
func (h *hook) end(
	span trace.Span,
	err error,
) {
	if err != nil && !errors.Is(err, goredis.Nil) {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

// pipelineErr returns the error of the first command failing other than
// with redis.Nil, as the error of a pipeline is the one of its first
// failing command, which may be redis.Nil, or else the error of the
// pipeline.
func pipelineErr(
	cmds []goredis.Cmder,
	err error,
) error {
	for _, cmd := range cmds {
		if cmdErr := cmd.Err(); cmdErr != nil && !errors.Is(cmdErr, goredis.Nil) {
			return cmdErr
		}
	}
	return err
}
