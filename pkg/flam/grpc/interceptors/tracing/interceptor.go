package tracing

import (
	"context"
	"strconv"
	"strings"

	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// interceptor is the tracing [flam.IGrpcInterceptor], serving each call
// within a server span.
type interceptor struct {
	id         string
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
}

// compile time check that *interceptor implements IGrpcInterceptor
var _ flam.IGrpcInterceptor = (*interceptor)(nil)

func (i *interceptor) Id() string {
	return i.id
}

// Unary returns the interceptor serving each unary call within a server
// span, given to the handler in its context.
func (i *interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		ctx, span := i.start(ctx, info.FullMethod)
		panicked := true
		defer func() {
			i.end(span, err, panicked)
		}()
		resp, err = handler(ctx, req)
		panicked = false
		return resp, err
	}
}

// Stream returns the interceptor serving each streaming call within a
// server span, given to the handler in the context of its stream.
func (i *interceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		ctx, span := i.start(ss.Context(), info.FullMethod)
		panicked := true
		defer func() {
			i.end(span, err, panicked)
		}()
		err = handler(srv, &stream{ServerStream: ss, ctx: ctx})
		panicked = false
		return err
	}
}

// start starts the server span of the call of the method, the child of
// the span of the context the propagator extracts from the metadata of the
// call, if any, named by the method, such as "pkg.Users/Get", and returns
// the context holding it.
func (i *interceptor) start(
	ctx context.Context,
	fullMethod string,
) (context.Context, trace.Span) {
	md, _ := metadata.FromIncomingContext(ctx)
	ctx = i.propagator.Extract(ctx, carrier(md))
	method := strings.TrimPrefix(fullMethod, "/")
	return i.tracer.Start(ctx, method,
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			semconv.RPCSystemNameGRPC,
			semconv.RPCMethod(method),
		),
	)
}

// end records the code of the status of the call on the span, then ends
// it, a server error, as flamgrpc.IsServerError tells, or a handler that
// panicked, setting the status of the span to an error, a panic being
// recorded as ErrPanicked and taken for an Internal status.
func (i *interceptor) end(
	span trace.Span,
	err error,
	panicked bool,
) {
	defer span.End()
	code := status.Code(err)
	if panicked {
		code = codes.Internal
		span.RecordError(ErrPanicked)
	}
	name := interceptorCode(code)
	span.SetAttributes(semconv.RPCResponseStatusCode(name))
	if flamgrpc.IsServerError(code) {
		span.SetAttributes(semconv.ErrorTypeKey.String(name))
		span.SetStatus(otelcodes.Error, name)
	}
}

// interceptorCode returns the name of the code in the semantic
// conventions, such as "DEADLINE_EXCEEDED", or its number, as text, for an
// unknown one.
func interceptorCode(
	code codes.Code,
) string {
	if int(code) < len(interceptorCodes) {
		return interceptorCodes[code]
	}
	return strconv.Itoa(int(code))
}

// interceptorCodes are the names of the codes, by code.
var interceptorCodes = []string{
	codes.OK:                 "OK",
	codes.Canceled:           "CANCELLED",
	codes.Unknown:            "UNKNOWN",
	codes.InvalidArgument:    "INVALID_ARGUMENT",
	codes.DeadlineExceeded:   "DEADLINE_EXCEEDED",
	codes.NotFound:           "NOT_FOUND",
	codes.AlreadyExists:      "ALREADY_EXISTS",
	codes.PermissionDenied:   "PERMISSION_DENIED",
	codes.ResourceExhausted:  "RESOURCE_EXHAUSTED",
	codes.FailedPrecondition: "FAILED_PRECONDITION",
	codes.Aborted:            "ABORTED",
	codes.OutOfRange:         "OUT_OF_RANGE",
	codes.Unimplemented:      "UNIMPLEMENTED",
	codes.Internal:           "INTERNAL",
	codes.Unavailable:        "UNAVAILABLE",
	codes.DataLoss:           "DATA_LOSS",
	codes.Unauthenticated:    "UNAUTHENTICATED",
}
