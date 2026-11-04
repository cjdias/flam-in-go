package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.uber.org/dig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var (
	errBoom        = errors.New("boom")
	errInterceptor = errors.New("interceptor")
)

// server is a gRPC server of a fake creator.
type server struct {
	id     string
	closes int
	err    error
}

func (s *server) Id() string                { return s.id }
func (s *server) Run(context.Context) error { return nil }
func (s *server) IsRunning() bool           { return false }
func (s *server) Address() string           { return "" }
func (s *server) Close() error              { s.closes++; return s.err }

// serverCreator creates the servers of the "fake" driver, failing for the
// id "bad".
type serverCreator struct{}

func (serverCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (serverCreator) Create(bag flam.IBag) (flam.IGrpcServer, error) {
	id := bag.String(factory.ConfigFieldId)
	if id == "bad" {
		return nil, errBoom
	}
	return &server{id: id}, nil
}

// closingInterceptor is an interceptor failing to close.
type closingInterceptor struct{}

func (closingInterceptor) Id() string                           { return "i" }
func (closingInterceptor) Unary() grpc.UnaryServerInterceptor   { return nil }
func (closingInterceptor) Stream() grpc.StreamServerInterceptor { return nil }
func (closingInterceptor) Close() error                         { return errInterceptor }

// newApp returns an app with the config and grpc bundles, the fake server
// creator, and the configuration holding the servers of the ids.
func newApp(t *testing.T, ids ...string) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(flamgrpc.NewBundle()))
	testutil.NoError(t, a.Provide(func() flam.IGrpcServerCreator { return serverCreator{} }, dig.Group(flamgrpc.ServerCreatorGroup)))
	bag := container.Bag{}
	for _, id := range ids {
		testutil.NoError(t, bag.Set(flamgrpc.ServerConfigPath+"."+id, container.Bag{factory.ConfigFieldDriver: "fake"}))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// processesArgs are the processes of the kennel.
type processesArgs struct {
	dig.In

	Processes []flam.IWatchdogProcess `group:"flam.watchdog.processes"`
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"ServerConfigPath":              {flamgrpc.ServerConfigPath, "flam.grpc.servers"},
		"ServerCreatorGroup":            {flamgrpc.ServerCreatorGroup, "flam.grpc.servers.creator"},
		"LoggerConfigPath":              {flamgrpc.LoggerConfigPath, "flam.grpc.loggers"},
		"LoggerCreatorGroup":            {flamgrpc.LoggerCreatorGroup, "flam.grpc.loggers.creator"},
		"InterceptorConfigPath":         {flamgrpc.InterceptorConfigPath, "flam.grpc.interceptors"},
		"InterceptorCreatorGroup":       {flamgrpc.InterceptorCreatorGroup, "flam.grpc.interceptors.creator"},
		"ServiceGroup":                  {flamgrpc.ServiceGroup, "flam.grpc.services"},
		"DefaultLoggerConfigPath":       {flamgrpc.DefaultLoggerConfigPath, "flam.grpc.defaults.servers.logger"},
		"DefaultInterceptorsConfigPath": {flamgrpc.DefaultInterceptorsConfigPath, "flam.grpc.defaults.servers.interceptors"},
		"ErrorFieldID":                  {flamgrpc.ErrorFieldID, "id"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	testutil.Equal(t, "ErrServer", flamgrpc.ErrServer.Error(), "unable to get grpc server")
}

func Test_SplitMethod(t *testing.T) {
	for fullMethod, want := range map[string][2]string{
		"/pkg.Users/Get":   {"pkg.Users", "Get"},
		"pkg.Users/Get":    {"pkg.Users", "Get"},
		"/a.b/c.D/Get":     {"a.b/c.D", "Get"},
		"/Get":             {"", "Get"},
		"":                 {"", ""},
		"/pkg.Users/":      {"pkg.Users", ""},
		"/grpc.health.v1/": {"grpc.health.v1", ""},
	} {
		service, method := flamgrpc.SplitMethod(fullMethod)
		testutil.Equal(t, fullMethod, [2]string{service, method}, want)
	}
}

func Test_IsServerError(t *testing.T) {
	servers := map[codes.Code]bool{
		codes.Unknown: true, codes.DeadlineExceeded: true, codes.Unimplemented: true,
		codes.Internal: true, codes.Unavailable: true, codes.DataLoss: true,
	}
	for code := codes.OK; code <= codes.Unauthenticated+1; code++ {
		testutil.Equal(t, code.String(), flamgrpc.IsServerError(code), servers[code])
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories", func(t *testing.T) {
		b := flamgrpc.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.grpc")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(b))
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-5:],
			[]string{"bundle:flam.grpc", "provider/0", "provider/0", "provider/0", "provider/1"})
		testutil.NoError(t, a.Invoke(func(
			servers flam.IGrpcServerFactory,
			loggers flam.IGrpcLoggerAdaptorFactory,
			interceptors flam.IGrpcInterceptorFactory,
			args processesArgs,
		) {
			testutil.Equal(t, "servers", servers.Available(), []string{})
			testutil.Equal(t, "loggers", loggers.Available(), []string{})
			testutil.Equal(t, "interceptors", interceptors.Available(), []string{})
			testutil.Equal(t, "processes", len(args.Processes), 0)
		}))
	})

	t.Run("should provide the servers as processes", func(t *testing.T) {
		a := newApp(t, "web", "api")
		testutil.NoError(t, a.Invoke(func(args processesArgs, servers flam.IGrpcServerFactory) {
			ids := map[string]flam.IWatchdogProcess{}
			for _, p := range args.Processes {
				ids[p.Id()] = p
			}
			testutil.Equal(t, "processes", len(ids), 2)
			for _, id := range []string{"api", "web"} {
				stored, _ := servers.Lookup(id)
				testutil.Equal(t, id+" stored", ids[id], flam.IWatchdogProcess(stored))
			}
		}))
	})

	t.Run("should fail to provide a server failing to be created", func(t *testing.T) {
		a := newApp(t, "api", "bad")
		err := a.Invoke(func(processesArgs) {})
		var e *flam.Error
		if !errors.Is(err, flamgrpc.ErrServer) || !errors.Is(err, errBoom) || !errors.As(err, &e) {
			t.Fatalf("Invoke() = %v, want ErrServer and errBoom", err)
		}
		testutil.Equal(t, "id", e.Context()[flamgrpc.ErrorFieldID], "bad")
	})

	t.Run("should close the factories", func(t *testing.T) {
		a := newApp(t, "api")
		var api flam.IGrpcServer
		testutil.NoError(t, a.Invoke(func(servers flam.IGrpcServerFactory, interceptors flam.IGrpcInterceptorFactory) {
			api, _ = servers.Get("api")
			api.(*server).err = errBoom
			testutil.NoError(t, interceptors.Store("i", closingInterceptor{}))
		}))

		err := flamgrpc.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errBoom) || !errors.Is(err, errInterceptor) {
			t.Fatalf("Close() = %v, want the errors of the server and of the interceptor", err)
		}
		testutil.Equal(t, "server closes", api.(*server).closes, 1)
		testutil.NoError(t, flamgrpc.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(flamgrpc.NewBundle()))
		if err := flamgrpc.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})

	t.Run("should add the servers to the kennel", func(t *testing.T) {
		a := newApp(t, "api")
		testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
		testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
		testutil.NoError(t, a.Invoke(func(k flam.IKennel) {
			testutil.Equal(t, "Available()", k.Available(), []string{"api"})
		}))
	})
}
