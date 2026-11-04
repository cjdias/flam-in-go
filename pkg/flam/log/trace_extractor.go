package log

import (
	"context"

	"go.opentelemetry.io/otel/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// newTraceExtractor returns the extractor of the ids of the OpenTelemetry
// trace and span of a context, stored at the fields read from
// [TraceIdFieldConfigPath] and [SpanIdFieldConfigPath]. It is a provider
// registered by the bundle returned from [NewBundle], in the
// [ContextExtractorGroup] group.
func newTraceExtractor(
	config flam.IConfig,
) flam.ILogContextExtractor {
	return &traceExtractor{
		traceField: config.String(TraceIdFieldConfigPath, ContextFieldTraceId),
		spanField:  config.String(SpanIdFieldConfigPath, ContextFieldSpanId),
	}
}

// traceExtractor is the [flam.ILogContextExtractor] of the ids of the
// trace and span of the span context held by a context, each left out
// when invalid, or when its field is empty.
type traceExtractor struct {
	traceField string
	spanField  string
}

// compile time check that *traceExtractor implements ILogContextExtractor
var _ flam.ILogContextExtractor = (*traceExtractor)(nil)

func (e *traceExtractor) Extract(
	ctx context.Context,
) flam.IBag {
	if ctx == nil {
		return nil
	}
	span := trace.SpanContextFromContext(ctx)
	fields := container.Bag{}
	if e.traceField != "" && span.HasTraceID() {
		fields[e.traceField] = span.TraceID().String()
	}
	if e.spanField != "" && span.HasSpanID() {
		fields[e.spanField] = span.SpanID().String()
	}
	return fields
}
