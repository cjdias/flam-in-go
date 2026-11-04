package servers

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/dig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/servers"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/servers/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// recorder is a logger adaptor recording its reports.
type recorder struct {
	mu      sync.Mutex
	reports []string
	errs    []error
}

func (r *recorder) Id() string { return "rec" }

func (r *recorder) Start(id, address string) { r.record("start:"+id+":"+address, nil) }

func (r *recorder) Error(id string, e error) { r.record("error:"+id, e) }

func (r *recorder) Shutdown(id string) { r.record("shutdown:"+id, nil) }

func (r *recorder) Done(id string) { r.record("done:"+id, nil) }

func (r *recorder) record(report string, e error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, report)
	if e != nil {
		r.errs = append(r.errs, e)
	}
}

// recorded returns the reports and the errors.
func (r *recorder) recorded() ([]string, []error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.reports), slices.Clone(r.errs)
}

// loggerCreator creates the recorder for the "rec" driver, failing for
// the "bad" one.
type loggerCreator struct{ r *recorder }

func (c loggerCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "rec"
}

func (c loggerCreator) Create(bag flam.IBag) (flam.IGrpcLoggerAdaptor, error) {
	if bag.String(factory.ConfigFieldId) == "bad" {
		return nil, errBoom
	}
	return c.r, nil
}

// calls records the calls going through the interceptors.
type calls struct {
	mu    sync.Mutex
	calls []string
}

func (c *calls) add(call string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *calls) recorded() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.calls)
}

// interceptor records the calls going through it as "id:kind:method", or
// intercepts nothing when its id is "nil".
type interceptor struct {
	id    string
	calls *calls
}

func (i interceptor) Id() string { return i.id }

func (i interceptor) Unary() grpc.UnaryServerInterceptor {
	if i.id == "nil" {
		return nil
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		i.calls.add(i.id + ":unary:" + info.FullMethod)
		return handler(ctx, req)
	}
}

func (i interceptor) Stream() grpc.StreamServerInterceptor {
	if i.id == "nil" {
		return nil
	}
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		i.calls.add(i.id + ":stream:" + info.FullMethod)
		return handler(srv, ss)
	}
}

// interceptorCreator creates the interceptors of the "ic" driver, failing
// for the id "bad".
type interceptorCreator struct{ calls *calls }

func (interceptorCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "ic"
}

func (c interceptorCreator) Create(bag flam.IBag) (flam.IGrpcInterceptor, error) {
	id := bag.String(factory.ConfigFieldId)
	if id == "bad" {
		return nil, errBoom
	}
	return interceptor{id: id, calls: c.calls}, nil
}

// echoDesc is the description of the test service, of a unary method,
// Echo, and of a server streaming one, Stream, both answering a health
// check request with a SERVING health check response, so that no protoc is
// needed.
var echoDesc = grpc.ServiceDesc{
	ServiceName: "test.Echo",
	HandlerType: (*any)(nil),
	Methods:     []grpc.MethodDesc{{MethodName: "Echo", Handler: echoUnary}},
	Streams:     []grpc.StreamDesc{{StreamName: "Stream", Handler: echoStream, ServerStreams: true}},
}

func echoUnary(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	req := &grpc_health_v1.HealthCheckRequest{}
	if err := dec(req); err != nil {
		return nil, err
	}
	handler := func(_ context.Context, req any) (any, error) {
		return srv.(*register).echo(req.(*grpc_health_v1.HealthCheckRequest)), nil
	}
	if interceptor == nil {
		return handler(ctx, req)
	}
	return interceptor(ctx, req, &grpc.UnaryServerInfo{Server: srv, FullMethod: "/test.Echo/Echo"}, handler)
}

func echoStream(srv any, stream grpc.ServerStream) error {
	req := &grpc_health_v1.HealthCheckRequest{}
	if err := stream.RecvMsg(req); err != nil {
		return err
	}
	return stream.SendMsg(srv.(*register).echo(req))
}

// register registers the test service on the servers of its group, or
// fails, or registers the descriptions of extra, with nil implementations,
// before the test service.
type register struct {
	group   string
	err     error
	extra   []*grpc.ServiceDesc
	impl    any
	entered chan struct{}
	release chan struct{}
}

func (r *register) Group() string { return r.group }

func (r *register) Register(registrar grpc.ServiceRegistrar) error {
	if r.err != nil {
		return r.err
	}
	for _, desc := range r.extra {
		registrar.RegisterService(desc, r.impl)
	}
	registrar.RegisterService(&echoDesc, r)
	return nil
}

// echo answers the request, waiting for the release for the service
// "wait".
func (r *register) echo(req *grpc_health_v1.HealthCheckRequest) *grpc_health_v1.HealthCheckResponse {
	if req.GetService() == "wait" {
		close(r.entered)
		<-r.release
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}
}

// newRegister returns the register of the group.
func newRegister(group string) *register {
	return &register{group: group, entered: make(chan struct{}), release: make(chan struct{})}
}

// registry is a health registry whose readiness is healthy as told,
// counting its probes.
type registry struct {
	flam.IHealthRegistry

	healthy atomic.Bool
	probes  atomic.Int32
}

func (r *registry) Readiness(context.Context) flam.HealthReport {
	r.probes.Add(1)
	if r.healthy.Load() {
		return flam.HealthReport{Status: flam.HealthStatusUp}
	}
	return flam.HealthReport{Status: flam.HealthStatusDown}
}

// env is the app of a test, with its logger adaptor, the calls through
// its interceptors and its clock.
type env struct {
	app   *testutil.App
	rec   *recorder
	calls *calls
	clock *testutil.Clock
}

// newEnv returns the env of an app with the config and grpc bundles, the
// standard server driver, a fake clock, the "rec" logger, the "a", "b",
// "nil" and "bad" interceptors, the health registry when not nil, the
// registers, and the configuration holding the values.
func newEnv(t *testing.T, values map[string]any, health flam.IHealthRegistry, registers ...flam.IGrpcServiceRegister) *env {
	t.Helper()
	e := &env{app: testutil.NewApp(), rec: &recorder{}, calls: &calls{}, clock: testutil.NewClock(time.Unix(0, 0))}
	testutil.NoError(t, e.app.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, e.app.RegisterBundle(flamgrpc.NewBundle()))
	testutil.NoError(t, e.app.RegisterBundle(servers.NewBundle()))
	testutil.NoError(t, e.app.Provide(func() flam.IClock { return e.clock }))
	testutil.NoError(t, e.app.Provide(func() flam.IGrpcLoggerAdaptorCreator { return loggerCreator{e.rec} }, dig.Group(flamgrpc.LoggerCreatorGroup)))
	testutil.NoError(t, e.app.Provide(func() flam.IGrpcInterceptorCreator { return interceptorCreator{e.calls} }, dig.Group(flamgrpc.InterceptorCreatorGroup)))
	if health != nil {
		testutil.NoError(t, e.app.Provide(func() flam.IHealthRegistry { return health }))
	}
	for _, reg := range registers {
		testutil.NoError(t, e.app.Provide(func() flam.IGrpcServiceRegister { return reg }, dig.Group(flamgrpc.ServiceGroup)))
	}

	bag := container.Bag{}
	testutil.NoError(t, standard.NewBundle().(flam.IConfigurableBundle).Config(bag))
	for path, value := range map[string]any{
		flamgrpc.LoggerConfigPath + ".rec":      container.Bag{factory.ConfigFieldDriver: "rec"},
		flamgrpc.LoggerConfigPath + ".bad":      container.Bag{factory.ConfigFieldDriver: "rec"},
		flamgrpc.InterceptorConfigPath + ".a":   container.Bag{factory.ConfigFieldDriver: "ic"},
		flamgrpc.InterceptorConfigPath + ".b":   container.Bag{factory.ConfigFieldDriver: "ic"},
		flamgrpc.InterceptorConfigPath + ".nil": container.Bag{factory.ConfigFieldDriver: "ic"},
		flamgrpc.InterceptorConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "ic"},
	} {
		testutil.NoError(t, bag.Set(path, value))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, e.app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return e
}

// apiConfig returns the configuration of a standard server, listening on
// a free local port, with the fields.
func apiConfig(fields map[string]any) container.Bag {
	bag := container.Bag{factory.ConfigFieldDriver: standard.Driver, standard.ConfigFieldAddress: "127.0.0.1:0"}
	for field, value := range fields {
		bag[field] = value
	}
	return bag
}

// api returns the values configuring the server "api" of the fields.
func api(fields map[string]any) map[string]any {
	return map[string]any{flamgrpc.ServerConfigPath + ".api": apiConfig(fields)}
}

// get returns the server of the id of the app.
func get(t *testing.T, a flam.IApp, id string) (flam.IGrpcServer, error) {
	t.Helper()
	var s flam.IGrpcServer
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.IGrpcServerFactory) { s, err = f.Get(id) }))
	return s, err
}

// running runs the server, returning the channel of the error of Run once
// the server listens.
func running(t *testing.T, s flam.IGrpcServer, ctx context.Context) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	for deadline := time.Now().Add(5 * time.Second); strings.HasSuffix(s.Address(), ":0"); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the server did not listen")
		}
	}
	return done
}

// wait returns the error of a Run, failing when it does not end.
func wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return")
		return nil
	}
}

// dial returns the client connection to the server, closed with the test.
func dial(t *testing.T, s flam.IGrpcServer, creds credentials.TransportCredentials) *grpc.ClientConn {
	t.Helper()
	if creds == nil {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(s.Address(), grpc.WithTransportCredentials(creds))
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// echo calls the Echo method of the test service, returning the status of
// the response.
func echo(conn *grpc.ClientConn, service string) (grpc_health_v1.HealthCheckResponse_ServingStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp := &grpc_health_v1.HealthCheckResponse{}
	err := conn.Invoke(ctx, "/test.Echo/Echo", &grpc_health_v1.HealthCheckRequest{Service: service}, resp)
	return resp.GetStatus(), err
}

// stream calls the Stream method of the test service, returning the
// status of the response.
func stream(t *testing.T, conn *grpc.ClientConn) grpc_health_v1.HealthCheckResponse_ServingStatus {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cs, err := conn.NewStream(ctx, &grpc.StreamDesc{ServerStreams: true}, "/test.Echo/Stream")
	testutil.NoError(t, err)
	testutil.NoError(t, cs.SendMsg(&grpc_health_v1.HealthCheckRequest{}))
	testutil.NoError(t, cs.CloseSend())
	resp := &grpc_health_v1.HealthCheckResponse{}
	testutil.NoError(t, cs.RecvMsg(resp))
	return resp.GetStatus()
}

// check returns the status of the service told by the health service of
// the server.
func check(conn *grpc.ClientConn, service string) (grpc_health_v1.HealthCheckResponse_ServingStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := grpc_health_v1.NewHealthClient(conn).Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: service})
	return resp.GetStatus(), err
}

// eventually waits for the health service of the server to tell the
// status of the service.
func eventually(t *testing.T, conn *grpc.ClientConn, service string, want grpc_health_v1.HealthCheckResponse_ServingStatus) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
		if got, err := check(conn, service); err == nil && got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the status of %q is not %v", service, want)
		}
	}
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Driver":                                  {standard.Driver, "flam.grpc.servers.driver.standard"},
		"ConfigFieldAddress":                      {standard.ConfigFieldAddress, "address"},
		"ConfigFieldGroup":                        {standard.ConfigFieldGroup, "group"},
		"ConfigFieldLogger":                       {standard.ConfigFieldLogger, "logger"},
		"ConfigFieldInterceptors":                 {standard.ConfigFieldInterceptors, "interceptors"},
		"ConfigFieldCertFile":                     {standard.ConfigFieldCertFile, "cert_file"},
		"ConfigFieldKeyFile":                      {standard.ConfigFieldKeyFile, "key_file"},
		"ConfigFieldConnectionTimeout":            {standard.ConfigFieldConnectionTimeout, "connection_timeout"},
		"ConfigFieldMaxRecvMsgSize":               {standard.ConfigFieldMaxRecvMsgSize, "max_recv_msg_size"},
		"ConfigFieldMaxSendMsgSize":               {standard.ConfigFieldMaxSendMsgSize, "max_send_msg_size"},
		"ConfigFieldKeepaliveTime":                {standard.ConfigFieldKeepaliveTime, "keepalive_time"},
		"ConfigFieldKeepaliveTimeout":             {standard.ConfigFieldKeepaliveTimeout, "keepalive_timeout"},
		"ConfigFieldKeepaliveMinTime":             {standard.ConfigFieldKeepaliveMinTime, "keepalive_min_time"},
		"ConfigFieldKeepalivePermitWithoutStream": {standard.ConfigFieldKeepalivePermitWithoutStream, "keepalive_permit_without_stream"},
		"ConfigFieldMaxConnectionIdle":            {standard.ConfigFieldMaxConnectionIdle, "max_connection_idle"},
		"ConfigFieldMaxConnectionAge":             {standard.ConfigFieldMaxConnectionAge, "max_connection_age"},
		"ConfigFieldMaxConnectionAgeGrace":        {standard.ConfigFieldMaxConnectionAgeGrace, "max_connection_age_grace"},
		"ConfigFieldReflection":                   {standard.ConfigFieldReflection, "reflection"},
		"ConfigFieldHealth":                       {standard.ConfigFieldHealth, "health"},
		"ConfigFieldHealthInterval":               {standard.ConfigFieldHealthInterval, "health_interval"},
		"ConfigFieldShutdownTimeout":              {standard.ConfigFieldShutdownTimeout, "shutdown_timeout"},
		"DefaultAddressConfigPath":                {standard.DefaultAddressConfigPath, "flam.grpc.defaults.servers.standard.address"},
		"DefaultConnectionTimeoutConfigPath":      {standard.DefaultConnectionTimeoutConfigPath, "flam.grpc.defaults.servers.standard.connection_timeout"},
		"DefaultMaxRecvMsgSizeConfigPath":         {standard.DefaultMaxRecvMsgSizeConfigPath, "flam.grpc.defaults.servers.standard.max_recv_msg_size"},
		"DefaultMaxSendMsgSizeConfigPath":         {standard.DefaultMaxSendMsgSizeConfigPath, "flam.grpc.defaults.servers.standard.max_send_msg_size"},
		"DefaultKeepaliveTimeConfigPath":          {standard.DefaultKeepaliveTimeConfigPath, "flam.grpc.defaults.servers.standard.keepalive_time"},
		"DefaultKeepaliveTimeoutConfigPath":       {standard.DefaultKeepaliveTimeoutConfigPath, "flam.grpc.defaults.servers.standard.keepalive_timeout"},
		"DefaultKeepaliveMinTimeConfigPath":       {standard.DefaultKeepaliveMinTimeConfigPath, "flam.grpc.defaults.servers.standard.keepalive_min_time"},
		"DefaultKeepalivePermitWithoutStreamConfigPath": {standard.DefaultKeepalivePermitWithoutStreamConfigPath,
			"flam.grpc.defaults.servers.standard.keepalive_permit_without_stream"},
		"DefaultMaxConnectionIdleConfigPath":     {standard.DefaultMaxConnectionIdleConfigPath, "flam.grpc.defaults.servers.standard.max_connection_idle"},
		"DefaultMaxConnectionAgeConfigPath":      {standard.DefaultMaxConnectionAgeConfigPath, "flam.grpc.defaults.servers.standard.max_connection_age"},
		"DefaultMaxConnectionAgeGraceConfigPath": {standard.DefaultMaxConnectionAgeGraceConfigPath, "flam.grpc.defaults.servers.standard.max_connection_age_grace"},
		"DefaultReflectionConfigPath":            {standard.DefaultReflectionConfigPath, "flam.grpc.defaults.servers.standard.reflection"},
		"DefaultHealthConfigPath":                {standard.DefaultHealthConfigPath, "flam.grpc.defaults.servers.standard.health"},
		"DefaultHealthIntervalConfigPath":        {standard.DefaultHealthIntervalConfigPath, "flam.grpc.defaults.servers.standard.health_interval"},
		"DefaultShutdownTimeoutConfigPath":       {standard.DefaultShutdownTimeoutConfigPath, "flam.grpc.defaults.servers.standard.shutdown_timeout"},
		"ErrorFieldID":                           {standard.ErrorFieldID, "id"},
		"ErrorFieldField":                        {standard.ErrorFieldField, "field"},
		"ErrorFieldLogger":                       {standard.ErrorFieldLogger, "logger"},
		"ErrorFieldInterceptor":                  {standard.ErrorFieldInterceptor, "interceptor"},
		"ErrorFieldAddress":                      {standard.ErrorFieldAddress, "address"},
		"ErrorFieldService":                      {standard.ErrorFieldService, "service"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, tc := range map[string]struct {
		err  error
		text string
	}{
		"ErrInvalidField":   {standard.ErrInvalidField, "invalid standard grpc server field"},
		"ErrLogger":         {standard.ErrLogger, "unable to get standard grpc server logger"},
		"ErrInterceptor":    {standard.ErrInterceptor, "unable to get standard grpc server interceptor"},
		"ErrCertificate":    {standard.ErrCertificate, "unable to load standard grpc server certificate"},
		"ErrRegister":       {standard.ErrRegister, "unable to register standard grpc server services"},
		"ErrInvalidService": {standard.ErrInvalidService, "invalid standard grpc server service"},
		"ErrRunning":        {standard.ErrRunning, "standard grpc server running"},
		"ErrServe":          {standard.ErrServe, "unable to serve standard grpc server"},
		"ErrShutdown":       {standard.ErrShutdown, "unable to shut down standard grpc server"},
	} {
		testutil.Equal(t, name, tc.err.Error(), tc.text)
	}
}

func Test_Bundle(t *testing.T) {
	testutil.Equal(t, "aggregate Id()", servers.NewBundle().Id(), "flam.grpc.servers")
	b := standard.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.servers.standard")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		standard.DefaultAddressConfigPath:                      ":9090",
		standard.DefaultConnectionTimeoutConfigPath:            "2m0s",
		standard.DefaultMaxRecvMsgSizeConfigPath:               4 << 20,
		standard.DefaultMaxSendMsgSizeConfigPath:               4 << 20,
		standard.DefaultKeepaliveTimeConfigPath:                "2h0m0s",
		standard.DefaultKeepaliveTimeoutConfigPath:             "20s",
		standard.DefaultKeepaliveMinTimeConfigPath:             "5m0s",
		standard.DefaultKeepalivePermitWithoutStreamConfigPath: false,
		standard.DefaultMaxConnectionIdleConfigPath:            "0s",
		standard.DefaultMaxConnectionAgeConfigPath:             "0s",
		standard.DefaultMaxConnectionAgeGraceConfigPath:        "0s",
		standard.DefaultReflectionConfigPath:                   false,
		standard.DefaultHealthConfigPath:                       true,
		standard.DefaultHealthIntervalConfigPath:               "5s",
		standard.DefaultShutdownTimeoutConfigPath:              "10s",
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}

	testutil.Equal(t, "conflict", b.(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}) != nil, true)
}

func Test_Creator(t *testing.T) {
	t.Run("should create a server with the defaults", func(t *testing.T) {
		e := newEnv(t, map[string]any{flamgrpc.ServerConfigPath + ".api": container.Bag{factory.ConfigFieldDriver: standard.Driver}}, nil)
		s, err := get(t, e.app, "api")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "api")
		testutil.Equal(t, "Address()", s.Address(), ":9090")
		testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
	})

	t.Run("should create a server with the built-in defaults", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(flamgrpc.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(servers.NewBundle()))
		testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(flamgrpc.ServerConfigPath+".api", container.Bag{factory.ConfigFieldDriver: standard.Driver}))
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		s, err := get(t, a, "api")
		testutil.NoError(t, err)
		testutil.Equal(t, "Address()", s.Address(), ":9090")
	})

	t.Run("should not create a server of another driver", func(t *testing.T) {
		e := newEnv(t, map[string]any{flamgrpc.ServerConfigPath + ".api": container.Bag{factory.ConfigFieldDriver: "other"}}, nil)
		_, err := get(t, e.app, "api")
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want ErrNoCreator", err)
		}
	})

	t.Run("should read the defaults from the configuration", func(t *testing.T) {
		e := newEnv(t, map[string]any{
			flamgrpc.ServerConfigPath + ".api":        container.Bag{factory.ConfigFieldDriver: standard.Driver},
			standard.DefaultAddressConfigPath:         "127.0.0.1:0",
			flamgrpc.DefaultLoggerConfigPath:          "rec",
			flamgrpc.DefaultInterceptorsConfigPath:    []any{"a", "nil", "b"},
			standard.DefaultShutdownTimeoutConfigPath: time.Second,
		}, nil, newRegister("api"), newRegister("other"))
		s, err := get(t, e.app, "api")
		testutil.NoError(t, err)
		testutil.Equal(t, "Address()", s.Address(), "127.0.0.1:0")

		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		conn := dial(t, s, nil)
		got, err := echo(conn, "")
		testutil.NoError(t, err)
		testutil.Equal(t, "echo", got, grpc_health_v1.HealthCheckResponse_SERVING)
		testutil.Equal(t, "stream", stream(t, conn), grpc_health_v1.HealthCheckResponse_SERVING)
		testutil.Equal(t, "calls", e.calls.recorded(), []string{
			"a:unary:/test.Echo/Echo", "b:unary:/test.Echo/Echo",
			"a:stream:/test.Echo/Stream", "b:stream:/test.Echo/Stream",
		})
		cancel()
		testutil.NoError(t, wait(t, done))
		reports, _ := e.rec.recorded()
		testutil.Equal(t, "reports", len(reports), 3)
	})

	t.Run("should serve the services of its group only", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldGroup: "g"}), nil, newRegister("api"))
		s, err := get(t, e.app, "api")
		testutil.NoError(t, err)
		done := running(t, s, context.Background())
		_, err = echo(dial(t, s, nil), "")
		testutil.Equal(t, "code", status.Code(err), codes.Unimplemented)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	for name, tc := range map[string]struct {
		fields   map[string]any
		defaults map[string]any
		field    string
	}{
		"address type":                   {map[string]any{standard.ConfigFieldAddress: 1}, nil, standard.ConfigFieldAddress},
		"empty address":                  {map[string]any{standard.ConfigFieldAddress: ""}, nil, standard.ConfigFieldAddress},
		"address default":                {nil, map[string]any{standard.DefaultAddressConfigPath: 1}, standard.ConfigFieldAddress},
		"group type":                     {map[string]any{standard.ConfigFieldGroup: 1}, nil, standard.ConfigFieldGroup},
		"empty group":                    {map[string]any{standard.ConfigFieldGroup: ""}, nil, standard.ConfigFieldGroup},
		"logger type":                    {map[string]any{standard.ConfigFieldLogger: 1}, nil, standard.ConfigFieldLogger},
		"logger default":                 {nil, map[string]any{flamgrpc.DefaultLoggerConfigPath: 1}, standard.ConfigFieldLogger},
		"interceptors type":              {map[string]any{standard.ConfigFieldInterceptors: "a"}, nil, standard.ConfigFieldInterceptors},
		"interceptor type":               {map[string]any{standard.ConfigFieldInterceptors: []any{"a", 1}}, nil, standard.ConfigFieldInterceptors},
		"interceptors default":           {nil, map[string]any{flamgrpc.DefaultInterceptorsConfigPath: 1}, standard.ConfigFieldInterceptors},
		"cert type":                      {map[string]any{standard.ConfigFieldCertFile: 1}, nil, standard.ConfigFieldCertFile},
		"key type":                       {map[string]any{standard.ConfigFieldKeyFile: 1}, nil, standard.ConfigFieldKeyFile},
		"cert without key":               {map[string]any{standard.ConfigFieldCertFile: "c"}, nil, standard.ConfigFieldKeyFile},
		"key without cert":               {map[string]any{standard.ConfigFieldKeyFile: "k"}, nil, standard.ConfigFieldKeyFile},
		"connection timeout type":        {map[string]any{standard.ConfigFieldConnectionTimeout: 1}, nil, standard.ConfigFieldConnectionTimeout},
		"zero connection timeout":        {map[string]any{standard.ConfigFieldConnectionTimeout: "0s"}, nil, standard.ConfigFieldConnectionTimeout},
		"connection timeout default":     {nil, map[string]any{standard.DefaultConnectionTimeoutConfigPath: "x"}, standard.ConfigFieldConnectionTimeout},
		"max recv size type":             {map[string]any{standard.ConfigFieldMaxRecvMsgSize: "1"}, nil, standard.ConfigFieldMaxRecvMsgSize},
		"zero max recv size":             {map[string]any{standard.ConfigFieldMaxRecvMsgSize: 0}, nil, standard.ConfigFieldMaxRecvMsgSize},
		"max recv size default":          {nil, map[string]any{standard.DefaultMaxRecvMsgSizeConfigPath: -1}, standard.ConfigFieldMaxRecvMsgSize},
		"max send size type":             {map[string]any{standard.ConfigFieldMaxSendMsgSize: 1.5}, nil, standard.ConfigFieldMaxSendMsgSize},
		"zero max send size":             {map[string]any{standard.ConfigFieldMaxSendMsgSize: 0}, nil, standard.ConfigFieldMaxSendMsgSize},
		"keepalive time":                 {map[string]any{standard.ConfigFieldKeepaliveTime: "soon"}, nil, standard.ConfigFieldKeepaliveTime},
		"keepalive time default":         {nil, map[string]any{standard.DefaultKeepaliveTimeConfigPath: 1}, standard.ConfigFieldKeepaliveTime},
		"keepalive timeout":              {map[string]any{standard.ConfigFieldKeepaliveTimeout: "-1s"}, nil, standard.ConfigFieldKeepaliveTimeout},
		"keepalive min time":             {map[string]any{standard.ConfigFieldKeepaliveMinTime: -time.Second}, nil, standard.ConfigFieldKeepaliveMinTime},
		"max connection idle":            {map[string]any{standard.ConfigFieldMaxConnectionIdle: true}, nil, standard.ConfigFieldMaxConnectionIdle},
		"max connection age":             {map[string]any{standard.ConfigFieldMaxConnectionAge: "x"}, nil, standard.ConfigFieldMaxConnectionAge},
		"max connection age grace":       {map[string]any{standard.ConfigFieldMaxConnectionAgeGrace: 3}, nil, standard.ConfigFieldMaxConnectionAgeGrace},
		"shutdown timeout":               {map[string]any{standard.ConfigFieldShutdownTimeout: -time.Second}, nil, standard.ConfigFieldShutdownTimeout},
		"shutdown timeout default":       {nil, map[string]any{standard.DefaultShutdownTimeoutConfigPath: "x"}, standard.ConfigFieldShutdownTimeout},
		"permit without stream":          {map[string]any{standard.ConfigFieldKeepalivePermitWithoutStream: "yes"}, nil, standard.ConfigFieldKeepalivePermitWithoutStream},
		"permit without stream default":  {nil, map[string]any{standard.DefaultKeepalivePermitWithoutStreamConfigPath: 1}, standard.ConfigFieldKeepalivePermitWithoutStream},
		"reflection":                     {map[string]any{standard.ConfigFieldReflection: "yes"}, nil, standard.ConfigFieldReflection},
		"health":                         {map[string]any{standard.ConfigFieldHealth: 1}, nil, standard.ConfigFieldHealth},
		"health interval type":           {map[string]any{standard.ConfigFieldHealthInterval: 1}, nil, standard.ConfigFieldHealthInterval},
		"zero health interval":           {map[string]any{standard.ConfigFieldHealthInterval: time.Duration(0)}, nil, standard.ConfigFieldHealthInterval},
		"health interval default":        {nil, map[string]any{standard.DefaultHealthIntervalConfigPath: "x"}, standard.ConfigFieldHealthInterval},
		"negative health interval field": {map[string]any{standard.ConfigFieldHealthInterval: "-1s"}, nil, standard.ConfigFieldHealthInterval},
	} {
		t.Run("should fail with an invalid "+name, func(t *testing.T) {
			values := api(tc.fields)
			for path, value := range tc.defaults {
				values[path] = value
			}
			if tc.defaults != nil {
				delete(values[flamgrpc.ServerConfigPath+".api"].(container.Bag), standard.ConfigFieldAddress)
			}
			e := newEnv(t, values, nil)
			_, err := get(t, e.app, "api")
			var fe *flam.Error
			if !errors.Is(err, standard.ErrInvalidField) || !errors.As(err, &fe) {
				t.Fatalf("Get() = %v, want ErrInvalidField", err)
			}
			for errors.As(fe.Unwrap(), &fe) {
			}
			testutil.Equal(t, "Error()", fe.Error(), "unable to create standard grpc server: invalid standard grpc server field")
			testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "api", standard.ErrorFieldField: tc.field})
		})
	}

	for name, tc := range map[string]struct {
		fields   map[string]any
		sentinel error
		ctx      map[string]any
	}{
		"unknown logger":      {map[string]any{standard.ConfigFieldLogger: "none"}, standard.ErrLogger, map[string]any{standard.ErrorFieldLogger: "none"}},
		"failing logger":      {map[string]any{standard.ConfigFieldLogger: "bad"}, standard.ErrLogger, map[string]any{standard.ErrorFieldLogger: "bad"}},
		"unknown interceptor": {map[string]any{standard.ConfigFieldInterceptors: []string{"a", "none"}}, standard.ErrInterceptor, map[string]any{standard.ErrorFieldInterceptor: "none"}},
		"failing interceptor": {map[string]any{standard.ConfigFieldInterceptors: []string{"bad"}}, standard.ErrInterceptor, map[string]any{standard.ErrorFieldInterceptor: "bad"}},
		"missing cert files":  {map[string]any{standard.ConfigFieldCertFile: "none.pem", standard.ConfigFieldKeyFile: "none.key"}, standard.ErrCertificate, map[string]any{}},
	} {
		t.Run("should fail with "+name, func(t *testing.T) {
			e := newEnv(t, api(tc.fields), nil)
			_, err := get(t, e.app, "api")
			var fe *flam.Error
			if !errors.Is(err, tc.sentinel) || !errors.As(err, &fe) {
				t.Fatalf("Get() = %v, want %v", err, tc.sentinel)
			}
			for errors.As(fe.Unwrap(), &fe) && !strings.HasPrefix(fe.Error(), "unable to create standard grpc server") {
			}
			tc.ctx[standard.ErrorFieldID] = "api"
			testutil.Context(t, fe, tc.ctx)
		})
	}

	t.Run("should fail when a register of its group fails", func(t *testing.T) {
		failing := newRegister("api")
		failing.err = errBoom
		other := newRegister("other")
		other.err = errors.New("other")
		e := newEnv(t, api(nil), nil, failing, other)
		_, err := get(t, e.app, "api")
		if !errors.Is(err, standard.ErrRegister) || !errors.Is(err, errBoom) {
			t.Fatalf("Get() = %v, want ErrRegister and errBoom", err)
		}
	})

	healthDesc := &grpc_health_v1.Health_ServiceDesc
	reflectionDesc := &grpc_reflection_v1.ServerReflection_ServiceDesc
	readerDesc := &grpc.ServiceDesc{ServiceName: "test.Reader", HandlerType: (*io.Reader)(nil)}
	for name, tc := range map[string]struct {
		fields   map[string]any
		register func(*register)
		service  string
	}{
		"a service registered twice": {nil, func(r *register) {
			r.extra = []*grpc.ServiceDesc{&echoDesc}
		}, "test.Echo"},
		"the health service registered by a register": {nil, func(r *register) {
			r.extra = []*grpc.ServiceDesc{healthDesc}
		}, "grpc.health.v1.Health"},
		"the reflection service registered by a register": {map[string]any{standard.ConfigFieldReflection: true}, func(r *register) {
			r.extra = []*grpc.ServiceDesc{reflectionDesc}
		}, "grpc.reflection.v1.ServerReflection"},
		"an implementation not of its handler type": {nil, func(r *register) {
			r.extra, r.impl = []*grpc.ServiceDesc{readerDesc, healthDesc}, struct{}{}
		}, "test.Reader"},
		"an implementation of no handler type": {nil, func(r *register) {
			r.extra, r.impl = []*grpc.ServiceDesc{{ServiceName: "test.None"}}, struct{}{}
		}, "test.None"},
		"an implementation of a handler type not an interface": {nil, func(r *register) {
			r.extra, r.impl = []*grpc.ServiceDesc{{ServiceName: "test.Int", HandlerType: (*int)(nil)}}, 1
		}, "test.Int"},
		"an implementation of a handler type not a pointer": {nil, func(r *register) {
			r.extra, r.impl = []*grpc.ServiceDesc{{ServiceName: "test.Value", HandlerType: 1}}, 1
		}, "test.Value"},
	} {
		t.Run("should fail with "+name, func(t *testing.T) {
			reg := newRegister("api")
			tc.register(reg)
			e := newEnv(t, api(tc.fields), nil, reg)
			_, err := get(t, e.app, "api")
			var fe *flam.Error
			if !errors.Is(err, standard.ErrRegister) || !errors.Is(err, standard.ErrInvalidService) || !errors.As(err, &fe) {
				t.Fatalf("Get() = %v, want ErrRegister and ErrInvalidService", err)
			}
			for errors.As(fe.Unwrap(), &fe) && !strings.HasPrefix(fe.Error(), "unable to create standard grpc server") {
			}
			testutil.Equal(t, "Error()", fe.Error(),
				"unable to create standard grpc server: unable to register standard grpc server services: invalid standard grpc server service")
			testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "api", standard.ErrorFieldService: tc.service})
		})
	}

	t.Run("should register a service of a nil implementation", func(t *testing.T) {
		reg := newRegister("api")
		reg.extra = []*grpc.ServiceDesc{readerDesc}
		e := newEnv(t, api(nil), nil, reg)
		_, err := get(t, e.app, "api")
		testutil.NoError(t, err)
	})

	t.Run("should serve the services of a register registering the health service without health", func(t *testing.T) {
		reg := &healthRegister{server: grpchealth.NewServer()}
		reg.server.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		e := newEnv(t, api(map[string]any{standard.ConfigFieldHealth: false}), nil, reg)
		s, err := get(t, e.app, "api")
		testutil.NoError(t, err)
		done := running(t, s, context.Background())
		got, err := check(dial(t, s, nil), "")
		testutil.NoError(t, err)
		testutil.Equal(t, "status", got, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})
}

// healthRegister registers its own health service.
type healthRegister struct{ server *grpchealth.Server }

func (r *healthRegister) Group() string { return "api" }

func (r *healthRegister) Register(registrar grpc.ServiceRegistrar) error {
	grpc_health_v1.RegisterHealthServer(registrar, r.server)
	return nil
}

// certificate writes a self-signed certificate of 127.0.0.1 and its key to
// files of a temporary directory, returning their paths and the pool
// trusting it.
func certificate(t *testing.T) (string, string, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testutil.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	testutil.NoError(t, err)
	keyDer, err := x509.MarshalECPrivateKey(key)
	testutil.NoError(t, err)

	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	testutil.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	testutil.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o600))
	cert, err := x509.ParseCertificate(der)
	testutil.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return certFile, keyFile, pool
}

func Test_Server(t *testing.T) {
	t.Run("should serve until its context is done", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldLogger: "rec"}), nil, newRegister("api"))
		s, _ := get(t, e.app, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		address := s.Address()
		testutil.Equal(t, "IsRunning()", s.IsRunning(), true)
		got, err := echo(dial(t, s, nil), "")
		testutil.NoError(t, err)
		testutil.Equal(t, "echo", got, grpc_health_v1.HealthCheckResponse_SERVING)

		cancel()
		testutil.NoError(t, wait(t, done))
		testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
		testutil.Equal(t, "Address()", s.Address(), "127.0.0.1:0")
		reports, errs := e.rec.recorded()
		testutil.Equal(t, "reports", reports, []string{"start:api:" + address, "shutdown:api", "done:api"})
		testutil.Equal(t, "errors", len(errs), 0)

		testutil.NoError(t, s.Run(context.Background()))
		testutil.NoError(t, s.Close())
		reports, _ = e.rec.recorded()
		testutil.Equal(t, "closed", len(reports), 3)
	})

	t.Run("should serve until closed", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldLogger: "rec"}), nil)
		s, _ := get(t, e.app, "api")
		var nilCtx context.Context
		done := running(t, s, nilCtx)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
		reports, _ := e.rec.recorded()
		testutil.Equal(t, "reports", reports[1:], []string{"shutdown:api", "done:api"})
	})

	t.Run("should do nothing when closed before running", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldLogger: "rec"}), nil)
		s, _ := get(t, e.app, "api")
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Run(context.Background()))
		reports, _ := e.rec.recorded()
		testutil.Equal(t, "reports", len(reports), 0)
	})

	t.Run("should run without logger nor health service", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldHealth: false}), nil)
		s, _ := get(t, e.app, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		_, err := check(dial(t, s, nil), "")
		testutil.Equal(t, "code", status.Code(err), codes.Unimplemented)
		cancel()
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should fail to run twice at once", func(t *testing.T) {
		e := newEnv(t, api(nil), nil)
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		testutil.Error(t, s.Run(context.Background()), standard.ErrRunning,
			"unable to run standard grpc server: standard grpc server running", map[string]any{standard.ErrorFieldID: "api"})
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	for name, logger := range map[string]string{"with logger": "rec", "without logger": ""} {
		t.Run("should fail to listen "+name, func(t *testing.T) {
			e := newEnv(t, api(map[string]any{
				standard.ConfigFieldAddress: "127.0.0.1:x",
				standard.ConfigFieldLogger:  logger,
			}), nil)
			s, _ := get(t, e.app, "api")
			err := s.Run(context.Background())
			var fe *flam.Error
			if !errors.Is(err, standard.ErrServe) || !errors.As(err, &fe) {
				t.Fatalf("Run() = %v, want ErrServe", err)
			}
			testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "api", standard.ErrorFieldAddress: "127.0.0.1:x"})
			testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
			if logger != "" {
				reports, errs := e.rec.recorded()
				testutil.Equal(t, "reports", reports, []string{"error:api", "done:api"})
				testutil.Equal(t, "errors", errs, []error{err})
			}
		})
	}

	for name, logger := range map[string]string{"with logger": "rec", "without logger": ""} {
		t.Run("should stop the calls not ending in time "+name, func(t *testing.T) {
			reg := newRegister("api")
			e := newEnv(t, api(map[string]any{
				standard.ConfigFieldLogger:          logger,
				standard.ConfigFieldShutdownTimeout: "10ms",
			}), nil, reg)
			s, _ := get(t, e.app, "api")
			done := running(t, s, context.Background())
			called := make(chan error, 1)
			conn := dial(t, s, nil)
			go func() {
				_, err := echo(conn, "wait")
				called <- err
			}()
			<-reg.entered

			err := s.Close()
			testutil.Error(t, err, standard.ErrShutdown,
				"unable to run standard grpc server: unable to shut down standard grpc server: context deadline exceeded",
				map[string]any{standard.ErrorFieldID: "api"})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("Close() = %v, want it to wrap context.DeadlineExceeded", err)
			}
			if <-called == nil {
				t.Error("the call under way was not stopped")
			}
			close(reg.release)
			testutil.NoError(t, wait(t, done))
			if logger != "" {
				reports, errs := e.rec.recorded()
				testutil.Equal(t, "reports", reports[1:], []string{"shutdown:api", "error:api", "done:api"})
				testutil.Equal(t, "errors", errs, []error{err})
			}
		})
	}

	t.Run("should stop at once without shutdown timeout", func(t *testing.T) {
		reg := newRegister("api")
		e := newEnv(t, api(map[string]any{standard.ConfigFieldShutdownTimeout: "0s"}), nil, reg)
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		called := make(chan error, 1)
		conn := dial(t, s, nil)
		go func() {
			_, err := echo(conn, "wait")
			called <- err
		}()
		<-reg.entered
		testutil.NoError(t, s.Close())
		if <-called == nil {
			t.Error("the call under way was not stopped")
		}
		close(reg.release)
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should let the calls under way end", func(t *testing.T) {
		reg := newRegister("api")
		e := newEnv(t, api(nil), nil, reg)
		s, _ := get(t, e.app, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		type result struct {
			status grpc_health_v1.HealthCheckResponse_ServingStatus
			err    error
		}
		called := make(chan result, 1)
		conn := dial(t, s, nil)
		go func() {
			got, err := echo(conn, "wait")
			called <- result{got, err}
		}()
		<-reg.entered
		cancel()
		time.Sleep(10 * time.Millisecond)
		close(reg.release)
		testutil.NoError(t, wait(t, done))
		testutil.Equal(t, "call", <-called, result{grpc_health_v1.HealthCheckResponse_SERVING, nil})
	})

	t.Run("should serve over TLS", func(t *testing.T) {
		certFile, keyFile, pool := certificate(t)
		e := newEnv(t, api(map[string]any{
			standard.ConfigFieldCertFile: certFile,
			standard.ConfigFieldKeyFile:  keyFile,
		}), nil, newRegister("api"))
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		conn := dial(t, s, credentials.NewTLS(&tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}))
		got, err := echo(conn, "")
		testutil.NoError(t, err)
		testutil.Equal(t, "echo", got, grpc_health_v1.HealthCheckResponse_SERVING)
		_, err = echo(dial(t, s, nil), "")
		testutil.Equal(t, "plain text code", status.Code(err), codes.Unavailable)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should serve the reflection service", func(t *testing.T) {
		e := newEnv(t, api(map[string]any{standard.ConfigFieldReflection: true}), nil, newRegister("api"))
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		info, err := grpc_reflection_v1.NewServerReflectionClient(dial(t, s, nil)).ServerReflectionInfo(ctx)
		testutil.NoError(t, err)
		testutil.NoError(t, info.Send(&grpc_reflection_v1.ServerReflectionRequest{
			MessageRequest: &grpc_reflection_v1.ServerReflectionRequest_ListServices{},
		}))
		resp, err := info.Recv()
		testutil.NoError(t, err)
		var names []string
		for _, service := range resp.GetListServicesResponse().GetService() {
			names = append(names, service.GetName())
		}
		sort.Strings(names)
		testutil.Equal(t, "services", names, []string{
			"grpc.health.v1.Health", "grpc.reflection.v1.ServerReflection", "grpc.reflection.v1alpha.ServerReflection", "test.Echo",
		})
		testutil.NoError(t, info.CloseSend())
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})
}

func Test_Health(t *testing.T) {
	t.Run("should tell the server serving without registry", func(t *testing.T) {
		e := newEnv(t, api(nil), nil, newRegister("api"))
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		conn := dial(t, s, nil)
		for _, service := range []string{"", "test.Echo", "grpc.health.v1.Health"} {
			got, err := check(conn, service)
			testutil.NoError(t, err)
			testutil.Equal(t, service, got, grpc_health_v1.HealthCheckResponse_SERVING)
		}
		_, err := check(conn, "test.Unknown")
		testutil.Equal(t, "unknown code", status.Code(err), codes.NotFound)
		testutil.Equal(t, "timers", e.clock.Timers(), 0)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should tell the readiness of the registry, probed each interval", func(t *testing.T) {
		r := &registry{}
		e := newEnv(t, api(map[string]any{standard.ConfigFieldHealthInterval: "1m"}), r, newRegister("api"))
		s, _ := get(t, e.app, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		conn := dial(t, s, nil)
		for deadline := time.Now().Add(5 * time.Second); r.probes.Load() == 0; time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("the registry was not probed")
			}
		}
		eventually(t, conn, "", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		eventually(t, conn, "test.Echo", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

		r.healthy.Store(true)
		e.clock.Advance(time.Minute)
		eventually(t, conn, "", grpc_health_v1.HealthCheckResponse_SERVING)
		eventually(t, conn, "test.Echo", grpc_health_v1.HealthCheckResponse_SERVING)

		r.healthy.Store(false)
		e.clock.Advance(time.Minute)
		eventually(t, conn, "", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

		cancel()
		testutil.NoError(t, wait(t, done))
		testutil.Equal(t, "timers", e.clock.Timers(), 0)
	})

	t.Run("should end the watches when the server stops", func(t *testing.T) {
		e := newEnv(t, api(nil), nil, newRegister("api"))
		s, _ := get(t, e.app, "api")
		done := running(t, s, context.Background())
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		watch, err := grpc_health_v1.NewHealthClient(dial(t, s, nil)).Watch(ctx, &grpc_health_v1.HealthCheckRequest{Service: "test.Echo"})
		testutil.NoError(t, err)
		resp, err := watch.Recv()
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.GetStatus(), grpc_health_v1.HealthCheckResponse_SERVING)

		start := time.Now()
		testutil.NoError(t, s.Close())
		if elapsed := time.Since(start); elapsed > 5*time.Second {
			t.Errorf("Close() took %v, held by the watch", elapsed)
		}
		testutil.NoError(t, wait(t, done))
		for {
			if _, err := watch.Recv(); err != nil {
				break
			}
		}
	})
}
