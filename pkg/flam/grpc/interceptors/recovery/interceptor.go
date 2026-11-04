package recovery

import (
	"context"
	"fmt"
	"runtime/debug"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// interceptor is the recovery [flam.IGrpcInterceptor], answering the calls
// whose handler panicked with an Internal status, and signaling the panic
// on its channel of the logger.
type interceptor struct {
	id      string
	logger  flam.ILogger
	channel string
	level   flam.LogLevel
}

// compile time check that *interceptor implements IGrpcInterceptor
var _ flam.IGrpcInterceptor = (*interceptor)(nil)

func (i *interceptor) Id() string {
	return i.id
}

// Unary returns the interceptor recovering the panics of the handlers of
// the unary calls.
func (i *interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer i.rescue(info.FullMethod, &err)
		return handler(ctx, req)
	}
}

// Stream returns the interceptor recovering the panics of the handlers of
// the streaming calls.
func (i *interceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		defer i.rescue(info.FullMethod, &err)
		return handler(srv, ss)
	}
}

// rescue recovers the panic of the handler of the call of the method, if
// any, signaling it with the stack of the handler, and setting the error
// of the call to the Internal status of [Message]. It must be deferred, to
// recover the panic.
func (i *interceptor) rescue(
	method string,
	err *error,
) {
	r := recover()
	if r == nil {
		return
	}
	i.logger.Signal(i.level, i.channel, interceptorMessage, container.Bag{
		ContextFieldMethod: method,
		ContextFieldPanic:  fmt.Sprint(r),
		ContextFieldStack:  string(debug.Stack()),
	})
	*err = status.Error(codes.Internal, Message)
}

// interceptorMessage is the message of the entries signaled by the
// interceptor.
const interceptorMessage = "grpc handler panicked"
