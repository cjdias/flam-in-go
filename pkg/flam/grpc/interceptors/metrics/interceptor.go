package metrics

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
)

// interceptor is the metrics [flam.IGrpcInterceptor], measuring the calls
// it serves on its metrics.
type interceptor struct {
	id       string
	clock    flam.IClock
	handled  *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// compile time check that *interceptor implements IGrpcInterceptor
var _ flam.IGrpcInterceptor = (*interceptor)(nil)

func (i *interceptor) Id() string {
	return i.id
}

// Unary returns the interceptor counting each unary call once served, and
// observing the time taken to serve it, even when its handler panics,
// whose panic then goes on.
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
			i.observe(info.FullMethod, i.clock.Since(start), err, panicked)
		}()
		resp, err = handler(ctx, req)
		panicked = false
		return resp, err
	}
}

// Stream returns the interceptor counting each streaming call once
// served, and observing the time taken to serve it, even when its handler
// panics, whose panic then goes on.
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
			i.observe(info.FullMethod, i.clock.Since(start), err, panicked)
		}()
		err = handler(srv, ss)
		panicked = false
		return err
	}
}

// observe counts the call of the method, and observes its duration, under
// its service, its method, and the name of its code, or CodePanic.
func (i *interceptor) observe(
	fullMethod string,
	duration time.Duration,
	err error,
	panicked bool,
) {
	service, method := flamgrpc.SplitMethod(fullMethod)
	code := status.Code(err).String()
	if panicked {
		code = CodePanic
	}
	i.handled.WithLabelValues(service, method, code).Inc()
	i.duration.WithLabelValues(service, method, code).Observe(duration.Seconds())
}
