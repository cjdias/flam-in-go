package log

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// interceptor is the log [flam.IGrpcInterceptor], signaling each call it
// serves on its channel of the logger.
type interceptor struct {
	id         string
	logger     flam.ILogger
	clock      flam.IClock
	channel    string
	level      flam.LogLevel
	levelError flam.LogLevel
}

// compile time check that *interceptor implements IGrpcInterceptor
var _ flam.IGrpcInterceptor = (*interceptor)(nil)

func (i *interceptor) Id() string {
	return i.id
}

// Unary returns the interceptor signaling each unary call once served,
// even when its handler panics, whose panic then goes on.
func (i *interceptor) Unary() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := i.clock.Now()
		panicked := true
		defer func() {
			i.signal(ctx, info.FullMethod, i.clock.Since(start), err, panicked)
		}()
		resp, err = handler(ctx, req)
		panicked = false
		return resp, err
	}
}

// Stream returns the interceptor signaling each streaming call once
// served, even when its handler panics, whose panic then goes on.
func (i *interceptor) Stream() grpc.StreamServerInterceptor {
	return func(
		srv any,
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) (err error) {
		start := i.clock.Now()
		panicked := true
		defer func() {
			i.signal(ss.Context(), info.FullMethod, i.clock.Since(start), err, panicked)
		}()
		err = handler(srv, ss)
		panicked = false
		return err
	}
}

// signal signals the call of the method, at the error level for a server
// error or a handler that panicked, and at the level otherwise, with its
// code, its duration, its error, and the address of the client the
// context holds, if any.
func (i *interceptor) signal(
	ctx context.Context,
	method string,
	duration time.Duration,
	err error,
	panicked bool,
) {
	code := status.Code(err)
	bag := container.Bag{
		ContextFieldMethod:   method,
		ContextFieldCode:     code.String(),
		ContextFieldDuration: duration,
	}
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		bag[ContextFieldPeer] = p.Addr.String()
	}
	level := i.level
	switch {
	case panicked:
		bag[ContextFieldCode] = CodePanic
		bag[ContextFieldPanicked] = true
		level = i.levelError
	case err != nil:
		bag[ContextFieldError] = err
		if flamgrpc.IsServerError(code) {
			level = i.levelError
		}
	}
	i.logger.Signal(level, i.channel, interceptorMessage, bag)
}

// interceptorMessage is the message of the entries signaled by the
// interceptor.
const interceptorMessage = "grpc call served"
