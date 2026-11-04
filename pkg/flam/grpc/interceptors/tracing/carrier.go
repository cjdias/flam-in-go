package tracing

import (
	"context"

	"go.opentelemetry.io/otel/propagation"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// stream is the server stream of a streaming call, of the context holding
// its span.
type stream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s *stream) Context() context.Context {
	return s.ctx
}

// carrier is the [propagation.TextMapCarrier] of the metadata of a call,
// whose keys are lower case.
type carrier metadata.MD

// compile time check that carrier implements TextMapCarrier
var _ propagation.TextMapCarrier = carrier(nil)

func (c carrier) Get(
	key string,
) string {
	if values := metadata.MD(c).Get(key); len(values) > 0 {
		return values[0]
	}
	return ""
}

func (c carrier) Set(
	key string,
	value string,
) {
	metadata.MD(c).Set(key, value)
}

func (c carrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}
