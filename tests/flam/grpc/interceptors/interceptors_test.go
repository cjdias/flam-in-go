package interceptors

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// fullMethod is the full method of the calls of the tests.
const fullMethod = "/pkg.Users/Get"

// entry is a signaled log entry.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     container.Bag
}

// entries is a flam.ILogger recording the signaled entries.
type entries struct {
	flam.ILogger

	mu       sync.Mutex
	signaled []entry
}

func (e *entries) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.signaled = append(e.signaled, entry{level, channel, message, ctx[0].(container.Bag)})
}

// recorded returns the signaled entries.
func (e *entries) recorded() []entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.signaled)
}

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// create returns the interceptor "main" of the fields, created by the
// factory of an app with the config and grpc bundles, the driver bundle,
// the configuration holding the values, and the providers.
func create(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, providers ...any) (flam.IGrpcInterceptor, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), flamgrpc.NewBundle(), driver}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamgrpc.InterceptorConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, provider := range providers {
		testutil.NoError(t, app.Provide(provider))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var i flam.IGrpcInterceptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IGrpcInterceptorFactory) { i, err = f.Get("main") }))
	return i, err
}

// clock returns the provider of the fake clock.
func clock(c *testutil.Clock) func() flam.IClock {
	return func() flam.IClock { return c }
}

// newClock returns a fake clock.
func newClock() *testutil.Clock {
	return testutil.NewClock(time.Unix(0, 0))
}

// stream is a server stream of its context.
type stream struct {
	grpc.ServerStream

	ctx context.Context
}

func (s stream) Context() context.Context { return s.ctx }

// withPeer returns the context holding the peer of the address.
func withPeer(ctx context.Context, address string) context.Context {
	return peer.NewContext(ctx, &peer.Peer{Addr: &net.TCPAddr{IP: net.ParseIP(address), Port: 5000}})
}

// unary calls the unary interceptor with a handler returning the response
// and the error, after advancing the clock, if any, by a second, or
// panicking with errBoom when panics.
func unary(i flam.IGrpcInterceptor, ctx context.Context, c *testutil.Clock, resp any, err error, panics bool) (any, error) {
	return i.Unary()(ctx, "req", &grpc.UnaryServerInfo{FullMethod: fullMethod}, func(context.Context, any) (any, error) {
		if c != nil {
			c.Advance(time.Second)
		}
		if panics {
			panic(errBoom)
		}
		return resp, err
	})
}

// streaming calls the stream interceptor with a handler returning the
// error, after advancing the clock, if any, by a second, or panicking with
// errBoom when panics.
func streaming(i flam.IGrpcInterceptor, ctx context.Context, c *testutil.Clock, err error, panics bool) error {
	return i.Stream()("srv", stream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: fullMethod}, func(any, grpc.ServerStream) error {
		if c != nil {
			c.Advance(time.Second)
		}
		if panics {
			panic(errBoom)
		}
		return err
	})
}

func Test_Bundle(t *testing.T) {
	b := interceptors.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.interceptors")

	app := testutil.NewApp()
	testutil.NoError(t, app.RegisterBundle(b))
	testutil.Equal(t, "calls", app.Calls(), []string{
		"bundle:flam.grpc.interceptors",
		"bundle:flam.grpc.interceptors.recovery", "provider/1",
		"bundle:flam.grpc.interceptors.log", "provider/1",
		"bundle:flam.grpc.interceptors.tracing", "provider/1",
		"bundle:flam.grpc.interceptors.metrics", "provider/1",
	})
}
