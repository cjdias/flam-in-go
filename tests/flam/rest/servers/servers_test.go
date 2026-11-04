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
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/servers"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/servers/standard"
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

func (c loggerCreator) Create(bag flam.IBag) (flam.IRestLoggerAdaptor, error) {
	if bag.String(factory.ConfigFieldId) == "bad" {
		return nil, errBoom
	}
	return c.r, nil
}

// middleware is a middleware adding its id to the trace header, or
// wrapping into nil when its id is "nil".
type middleware struct{ id string }

func (m middleware) Id() string { return m.id }

func (m middleware) Wrap(_ flam.RestEndpoint, next http.Handler) http.Handler {
	if m.id == "nil" {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Trace", m.id)
		next.ServeHTTP(w, req)
	})
}

// middlewareCreator creates the middlewares of the "mw" driver, failing
// for the id "bad".
type middlewareCreator struct{}

func (middlewareCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "mw"
}

func (middlewareCreator) Create(bag flam.IBag) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)
	if id == "bad" {
		return nil, errBoom
	}
	return middleware{id: id}, nil
}

// register registers the endpoints of its group, a GET of /hello writing
// the service id, and a GET of /wait waiting for release, or fails.
type register struct {
	group   string
	err     error
	entered chan struct{}
	release chan struct{}
}

func (r *register) Group() string { return r.group }

func (r *register) Register(router flam.IRestRouter) error {
	if r.err != nil {
		return r.err
	}
	if err := router.Handle(flam.RestEndpoint{Id: 1, Pattern: "GET /hello"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		endpoint, _ := rest.EndpointOf(req.Context())
		_, _ = io.WriteString(w, r.group+":"+string(rune('0'+endpoint.ServiceId)))
	})); err != nil {
		return err
	}
	return router.Handle(flam.RestEndpoint{Id: 2, Pattern: "GET /wait"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(r.entered)
		<-r.release
		_, _ = io.WriteString(w, "released")
	}))
}

// newRegister returns the register of the group.
func newRegister(group string) *register {
	return &register{group: group, entered: make(chan struct{}), release: make(chan struct{})}
}

// newApp returns an app with the config and rest bundles, the standard
// server driver, the "rec" logger, the "a", "b" and "bad" middlewares, the
// registers, and the configuration holding the values.
func newApp(t *testing.T, values map[string]any, registers ...flam.IRestEndpointRegister) (*testutil.App, *recorder) {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(rest.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(servers.NewBundle()))
	r := &recorder{}
	testutil.NoError(t, a.Provide(func() flam.IRestLoggerAdaptorCreator { return loggerCreator{r} }, dig.Group(rest.LoggerCreatorGroup)))
	testutil.NoError(t, a.Provide(func() flam.IRestMiddlewareCreator { return middlewareCreator{} }, dig.Group(rest.MiddlewareCreatorGroup)))
	for _, reg := range registers {
		testutil.NoError(t, a.Provide(func() flam.IRestEndpointRegister { return reg }, dig.Group(rest.EndpointGroup)))
	}

	bag := container.Bag{}
	for path, value := range map[string]any{
		rest.LoggerConfigPath + ".rec":     container.Bag{factory.ConfigFieldDriver: "rec"},
		rest.LoggerConfigPath + ".bad":     container.Bag{factory.ConfigFieldDriver: "rec"},
		rest.MiddlewareConfigPath + ".a":   container.Bag{factory.ConfigFieldDriver: "mw"},
		rest.MiddlewareConfigPath + ".b":   container.Bag{factory.ConfigFieldDriver: "mw"},
		rest.MiddlewareConfigPath + ".bad": container.Bag{factory.ConfigFieldDriver: "mw"},
		rest.MiddlewareConfigPath + ".nil": container.Bag{factory.ConfigFieldDriver: "mw"},
	} {
		testutil.NoError(t, bag.Set(path, value))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a, r
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

// get returns the server of the id of the app.
func get(t *testing.T, a flam.IApp, id string) (flam.IRestServer, error) {
	t.Helper()
	var s flam.IRestServer
	var err error
	testutil.NoError(t, a.Invoke(func(f flam.IRestServerFactory) { s, err = f.Get(id) }))
	return s, err
}

// running runs the server, returning the channel of the error of Run once
// the server listens.
func running(t *testing.T, s flam.IRestServer, ctx context.Context) <-chan error {
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

// fetch returns the response body of a GET of the path on the server.
func fetch(t *testing.T, client *http.Client, url string) (string, http.Header) {
	t.Helper()
	resp, err := client.Get(url)
	testutil.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	testutil.NoError(t, err)
	return string(body), resp.Header
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Driver":                             {standard.Driver, "flam.rest.servers.driver.standard"},
		"ConfigFieldAddress":                 {standard.ConfigFieldAddress, "address"},
		"ConfigFieldGroup":                   {standard.ConfigFieldGroup, "group"},
		"ConfigFieldLogger":                  {standard.ConfigFieldLogger, "logger"},
		"ConfigFieldMiddlewares":             {standard.ConfigFieldMiddlewares, "middlewares"},
		"ConfigFieldCertFile":                {standard.ConfigFieldCertFile, "cert_file"},
		"ConfigFieldKeyFile":                 {standard.ConfigFieldKeyFile, "key_file"},
		"ConfigFieldReadHeaderTimeout":       {standard.ConfigFieldReadHeaderTimeout, "read_header_timeout"},
		"ConfigFieldShutdownTimeout":         {standard.ConfigFieldShutdownTimeout, "shutdown_timeout"},
		"ConfigFieldReadTimeout":             {standard.ConfigFieldReadTimeout, "read_timeout"},
		"ConfigFieldWriteTimeout":            {standard.ConfigFieldWriteTimeout, "write_timeout"},
		"ConfigFieldIdleTimeout":             {standard.ConfigFieldIdleTimeout, "idle_timeout"},
		"ConfigFieldMaxHeaderBytes":          {standard.ConfigFieldMaxHeaderBytes, "max_header_bytes"},
		"DefaultReadTimeoutConfigPath":       {standard.DefaultReadTimeoutConfigPath, "flam.rest.defaults.servers.standard.read_timeout"},
		"DefaultWriteTimeoutConfigPath":      {standard.DefaultWriteTimeoutConfigPath, "flam.rest.defaults.servers.standard.write_timeout"},
		"DefaultIdleTimeoutConfigPath":       {standard.DefaultIdleTimeoutConfigPath, "flam.rest.defaults.servers.standard.idle_timeout"},
		"DefaultMaxHeaderBytesConfigPath":    {standard.DefaultMaxHeaderBytesConfigPath, "flam.rest.defaults.servers.standard.max_header_bytes"},
		"DefaultAddressConfigPath":           {standard.DefaultAddressConfigPath, "flam.rest.defaults.servers.standard.address"},
		"DefaultReadHeaderTimeoutConfigPath": {standard.DefaultReadHeaderTimeoutConfigPath, "flam.rest.defaults.servers.standard.read_header_timeout"},
		"DefaultShutdownTimeoutConfigPath":   {standard.DefaultShutdownTimeoutConfigPath, "flam.rest.defaults.servers.standard.shutdown_timeout"},
		"ErrorFieldID":                       {standard.ErrorFieldID, "id"},
		"ErrorFieldField":                    {standard.ErrorFieldField, "field"},
		"ErrorFieldLogger":                   {standard.ErrorFieldLogger, "logger"},
		"ErrorFieldMiddleware":               {standard.ErrorFieldMiddleware, "middleware"},
		"ErrorFieldAddress":                  {standard.ErrorFieldAddress, "address"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrInvalidField": standard.ErrInvalidField,
		"ErrLogger":       standard.ErrLogger,
		"ErrMiddleware":   standard.ErrMiddleware,
		"ErrCertificate":  standard.ErrCertificate,
		"ErrRegister":     standard.ErrRegister,
		"ErrRunning":      standard.ErrRunning,
		"ErrServe":        standard.ErrServe,
		"ErrShutdown":     standard.ErrShutdown,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	testutil.Equal(t, "aggregate Id()", servers.NewBundle().Id(), "flam.rest.servers")
	b := standard.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.rest.servers.standard")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	testutil.Equal(t, "address", bag.Get(standard.DefaultAddressConfigPath), ":8080")
	testutil.Equal(t, "read header timeout", bag.Get(standard.DefaultReadHeaderTimeoutConfigPath), "10s")
	testutil.Equal(t, "shutdown timeout", bag.Get(standard.DefaultShutdownTimeoutConfigPath), "10s")
	testutil.Equal(t, "read timeout", bag.Get(standard.DefaultReadTimeoutConfigPath), "30s")
	testutil.Equal(t, "write timeout", bag.Get(standard.DefaultWriteTimeoutConfigPath), "0s")
	testutil.Equal(t, "idle timeout", bag.Get(standard.DefaultIdleTimeoutConfigPath), "2m0s")
	testutil.Equal(t, "max header bytes", bag.Get(standard.DefaultMaxHeaderBytesConfigPath), 1<<20)

	testutil.Equal(t, "conflict", b.(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}) != nil, true)
}

func Test_Creator(t *testing.T) {
	t.Run("should create a server with the defaults", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": container.Bag{factory.ConfigFieldDriver: standard.Driver}})
		s, err := get(t, a, "api")
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "api")
		testutil.Equal(t, "Address()", s.Address(), ":8080")
		testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
	})

	t.Run("should not create a server of another driver", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": container.Bag{factory.ConfigFieldDriver: "other"}})
		_, err := get(t, a, "api")
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want ErrNoCreator", err)
		}
	})

	t.Run("should read the defaults from the configuration", func(t *testing.T) {
		a, r := newApp(t, map[string]any{
			rest.ServerConfigPath + ".api":            container.Bag{factory.ConfigFieldDriver: standard.Driver},
			standard.DefaultAddressConfigPath:         "127.0.0.1:0",
			rest.DefaultLoggerConfigPath:              "rec",
			rest.DefaultMiddlewaresConfigPath:         []any{"a", "b"},
			rest.ServiceIdConfigPath:                  7,
			standard.DefaultShutdownTimeoutConfigPath: time.Second,
		}, newRegister("api"), newRegister("other"))
		s, err := get(t, a, "api")
		testutil.NoError(t, err)
		testutil.Equal(t, "Address()", s.Address(), "127.0.0.1:0")

		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		body, header := fetch(t, http.DefaultClient, "http://"+s.Address()+"/hello")
		testutil.Equal(t, "body", body, "api:7")
		testutil.Equal(t, "trace", header.Values("Trace"), []string{"a", "b"})
		cancel()
		testutil.NoError(t, wait(t, done))
		reports, _ := r.recorded()
		testutil.Equal(t, "reports", len(reports), 3)
	})

	for name, tc := range map[string]struct {
		fields   map[string]any
		defaults map[string]any
		field    string
	}{
		"address type":             {map[string]any{standard.ConfigFieldAddress: 1}, nil, standard.ConfigFieldAddress},
		"empty address":            {map[string]any{standard.ConfigFieldAddress: ""}, nil, standard.ConfigFieldAddress},
		"address default":          {nil, map[string]any{standard.DefaultAddressConfigPath: 1}, standard.ConfigFieldAddress},
		"group type":               {map[string]any{standard.ConfigFieldGroup: 1}, nil, standard.ConfigFieldGroup},
		"empty group":              {map[string]any{standard.ConfigFieldGroup: ""}, nil, standard.ConfigFieldGroup},
		"logger type":              {map[string]any{standard.ConfigFieldLogger: 1}, nil, standard.ConfigFieldLogger},
		"logger default":           {nil, map[string]any{rest.DefaultLoggerConfigPath: 1}, standard.ConfigFieldLogger},
		"middlewares type":         {map[string]any{standard.ConfigFieldMiddlewares: "a"}, nil, standard.ConfigFieldMiddlewares},
		"middleware type":          {map[string]any{standard.ConfigFieldMiddlewares: []any{"a", 1}}, nil, standard.ConfigFieldMiddlewares},
		"middlewares default":      {nil, map[string]any{rest.DefaultMiddlewaresConfigPath: 1}, standard.ConfigFieldMiddlewares},
		"cert type":                {map[string]any{standard.ConfigFieldCertFile: 1}, nil, standard.ConfigFieldCertFile},
		"key type":                 {map[string]any{standard.ConfigFieldKeyFile: 1}, nil, standard.ConfigFieldKeyFile},
		"cert without key":         {map[string]any{standard.ConfigFieldCertFile: "c"}, nil, standard.ConfigFieldKeyFile},
		"key without cert":         {map[string]any{standard.ConfigFieldKeyFile: "k"}, nil, standard.ConfigFieldKeyFile},
		"read timeout type":        {map[string]any{standard.ConfigFieldReadHeaderTimeout: 1}, nil, standard.ConfigFieldReadHeaderTimeout},
		"read timeout text":        {map[string]any{standard.ConfigFieldReadHeaderTimeout: "soon"}, nil, standard.ConfigFieldReadHeaderTimeout},
		"negative read timeout":    {map[string]any{standard.ConfigFieldReadHeaderTimeout: "-1s"}, nil, standard.ConfigFieldReadHeaderTimeout},
		"read timeout default":     {nil, map[string]any{standard.DefaultReadHeaderTimeoutConfigPath: "x"}, standard.ConfigFieldReadHeaderTimeout},
		"shutdown timeout":         {map[string]any{standard.ConfigFieldShutdownTimeout: -time.Second}, nil, standard.ConfigFieldShutdownTimeout},
		"shutdown timeout default": {nil, map[string]any{standard.DefaultShutdownTimeoutConfigPath: "x"}, standard.ConfigFieldShutdownTimeout},
		"read timeout":             {map[string]any{standard.ConfigFieldReadTimeout: "x"}, nil, standard.ConfigFieldReadTimeout},
		"write timeout":            {map[string]any{standard.ConfigFieldWriteTimeout: "-1s"}, nil, standard.ConfigFieldWriteTimeout},
		"idle timeout":             {map[string]any{standard.ConfigFieldIdleTimeout: 3}, nil, standard.ConfigFieldIdleTimeout},
		"max header bytes type":    {map[string]any{standard.ConfigFieldMaxHeaderBytes: "1"}, nil, standard.ConfigFieldMaxHeaderBytes},
		"zero max header bytes":    {map[string]any{standard.ConfigFieldMaxHeaderBytes: 0}, nil, standard.ConfigFieldMaxHeaderBytes},
		"service id":               {nil, map[string]any{rest.ServiceIdConfigPath: "7"}, rest.ServiceIdConfigPath},
	} {
		t.Run("should fail with an invalid "+name, func(t *testing.T) {
			values := map[string]any{rest.ServerConfigPath + ".api": apiConfig(tc.fields)}
			for path, value := range tc.defaults {
				values[path] = value
			}
			if tc.defaults != nil {
				delete(values[rest.ServerConfigPath+".api"].(container.Bag), standard.ConfigFieldAddress)
			}
			a, _ := newApp(t, values)
			_, err := get(t, a, "api")
			var e *flam.Error
			if !errors.Is(err, standard.ErrInvalidField) || !errors.As(err, &e) {
				t.Fatalf("Get() = %v, want ErrInvalidField", err)
			}
			for errors.As(e.Unwrap(), &e) {
			}
			testutil.Equal(t, "Error()", e.Error(), "unable to create standard rest server: invalid standard rest server field")
			testutil.Context(t, e, map[string]any{standard.ErrorFieldID: "api", standard.ErrorFieldField: tc.field})
		})
	}

	for name, tc := range map[string]struct {
		fields   map[string]any
		sentinel error
		ctx      map[string]any
	}{
		"unknown logger":     {map[string]any{standard.ConfigFieldLogger: "none"}, standard.ErrLogger, map[string]any{standard.ErrorFieldLogger: "none"}},
		"failing logger":     {map[string]any{standard.ConfigFieldLogger: "bad"}, standard.ErrLogger, map[string]any{standard.ErrorFieldLogger: "bad"}},
		"unknown middleware": {map[string]any{standard.ConfigFieldMiddlewares: []string{"a", "none"}}, standard.ErrMiddleware, map[string]any{standard.ErrorFieldMiddleware: "none"}},
		"failing middleware": {map[string]any{standard.ConfigFieldMiddlewares: []string{"bad"}}, standard.ErrMiddleware, map[string]any{standard.ErrorFieldMiddleware: "bad"}},
		"nil middleware":     {map[string]any{standard.ConfigFieldMiddlewares: []string{"nil"}}, standard.ErrMiddleware, map[string]any{}},
		"missing cert files": {map[string]any{standard.ConfigFieldCertFile: "none.pem", standard.ConfigFieldKeyFile: "none.key"}, standard.ErrCertificate, map[string]any{}},
	} {
		t.Run("should fail with "+name, func(t *testing.T) {
			a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(tc.fields)})
			_, err := get(t, a, "api")
			var e *flam.Error
			if !errors.Is(err, tc.sentinel) || !errors.As(err, &e) {
				t.Fatalf("Get() = %v, want %v", err, tc.sentinel)
			}
			for errors.As(e.Unwrap(), &e) && !strings.HasPrefix(e.Error(), "unable to create standard rest server") {
			}
			tc.ctx[standard.ErrorFieldID] = "api"
			testutil.Context(t, e, tc.ctx)
		})
	}

	t.Run("should fail when a register of its group fails", func(t *testing.T) {
		failing := newRegister("api")
		failing.err = errBoom
		other := newRegister("other")
		other.err = errors.New("other")
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(nil)}, failing, other)
		_, err := get(t, a, "api")
		if !errors.Is(err, standard.ErrRegister) || !errors.Is(err, errBoom) {
			t.Fatalf("Get() = %v, want ErrRegister and errBoom", err)
		}
	})

	t.Run("should fail when the endpoints of its group conflict", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{standard.ConfigFieldGroup: "g"})},
			newRegister("g"), newRegister("g"))
		_, err := get(t, a, "api")
		if !errors.Is(err, standard.ErrRegister) || !errors.Is(err, rest.ErrInvalidEndpoint) {
			t.Fatalf("Get() = %v, want ErrRegister and ErrInvalidEndpoint", err)
		}
	})
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
		a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{standard.ConfigFieldLogger: "rec"})},
			newRegister("api"))
		s, _ := get(t, a, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		address := s.Address()
		testutil.Equal(t, "IsRunning()", s.IsRunning(), true)
		body, _ := fetch(t, http.DefaultClient, "http://"+address+"/hello")
		testutil.Equal(t, "body", body, "api:0")

		cancel()
		testutil.NoError(t, wait(t, done))
		testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
		testutil.Equal(t, "Address()", s.Address(), "127.0.0.1:0")
		reports, errs := r.recorded()
		testutil.Equal(t, "reports", reports, []string{"start:api:" + address, "shutdown:api", "done:api"})
		testutil.Equal(t, "errors", len(errs), 0)

		testutil.NoError(t, s.Run(context.Background()))
		testutil.NoError(t, s.Close())
		reports, _ = r.recorded()
		testutil.Equal(t, "closed", len(reports), 3)
	})

	t.Run("should serve until closed", func(t *testing.T) {
		a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{standard.ConfigFieldLogger: "rec"})})
		s, _ := get(t, a, "api")
		var nilCtx context.Context
		done := running(t, s, nilCtx)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
		reports, _ := r.recorded()
		testutil.Equal(t, "reports", reports[1:], []string{"shutdown:api", "done:api"})
	})

	t.Run("should do nothing when closed before running", func(t *testing.T) {
		a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{standard.ConfigFieldLogger: "rec"})})
		s, _ := get(t, a, "api")
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Run(context.Background()))
		reports, _ := r.recorded()
		testutil.Equal(t, "reports", len(reports), 0)
	})

	t.Run("should run without logger", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(nil)})
		s, _ := get(t, a, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		cancel()
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should fail to run twice at once", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(nil)})
		s, _ := get(t, a, "api")
		done := running(t, s, context.Background())
		testutil.Error(t, s.Run(context.Background()), standard.ErrRunning,
			"unable to run standard rest server: standard rest server running", map[string]any{standard.ErrorFieldID: "api"})
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	for name, logger := range map[string]string{"with logger": "rec", "without logger": ""} {
		t.Run("should fail to listen "+name, func(t *testing.T) {
			a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{
				standard.ConfigFieldAddress: "127.0.0.1:x",
				standard.ConfigFieldLogger:  logger,
			})})
			s, _ := get(t, a, "api")
			err := s.Run(context.Background())
			var e *flam.Error
			if !errors.Is(err, standard.ErrServe) || !errors.As(err, &e) {
				t.Fatalf("Run() = %v, want ErrServe", err)
			}
			testutil.Context(t, e, map[string]any{standard.ErrorFieldID: "api", standard.ErrorFieldAddress: "127.0.0.1:x"})
			testutil.Equal(t, "IsRunning()", s.IsRunning(), false)
			if logger != "" {
				reports, errs := r.recorded()
				testutil.Equal(t, "reports", reports, []string{"error:api", "done:api"})
				testutil.Equal(t, "errors", errs, []error{err})
			}
		})
	}

	for name, logger := range map[string]string{"with logger": "rec", "without logger": ""} {
		t.Run("should close the requests not ending in time "+name, func(t *testing.T) {
			reg := newRegister("api")
			a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{
				standard.ConfigFieldLogger:          logger,
				standard.ConfigFieldShutdownTimeout: "10ms",
			})}, reg)
			s, _ := get(t, a, "api")
			done := running(t, s, context.Background())
			fetched := make(chan error, 1)
			go func() {
				resp, err := http.Get("http://" + s.Address() + "/wait")
				if err == nil {
					_ = resp.Body.Close()
				}
				fetched <- err
			}()
			<-reg.entered

			err := s.Close()
			testutil.Error(t, err, standard.ErrShutdown,
				"unable to run standard rest server: unable to shut down standard rest server: context deadline exceeded",
				map[string]any{standard.ErrorFieldID: "api"})
			if <-fetched == nil {
				t.Error("the request under way was not closed")
			}
			close(reg.release)
			testutil.NoError(t, wait(t, done))
			if logger != "" {
				reports, errs := r.recorded()
				testutil.Equal(t, "reports", reports[1:], []string{"shutdown:api", "error:api", "done:api"})
				testutil.Equal(t, "errors", errs, []error{err})
			}
		})
	}

	t.Run("should let the requests under way end", func(t *testing.T) {
		reg := newRegister("api")
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(nil)}, reg)
		s, _ := get(t, a, "api")
		ctx, cancel := context.WithCancel(context.Background())
		done := running(t, s, ctx)
		fetched := make(chan string, 1)
		go func() {
			body, _ := fetch(t, http.DefaultClient, "http://"+s.Address()+"/wait")
			fetched <- body
		}()
		<-reg.entered
		cancel()
		time.Sleep(10 * time.Millisecond)
		close(reg.release)
		testutil.NoError(t, wait(t, done))
		testutil.Equal(t, "body", <-fetched, "released")
	})

	t.Run("should serve https", func(t *testing.T) {
		certFile, keyFile, pool := certificate(t)
		a, _ := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{
			standard.ConfigFieldCertFile: certFile,
			standard.ConfigFieldKeyFile:  keyFile,
		})}, newRegister("api"))
		s, _ := get(t, a, "api")
		done := running(t, s, context.Background())
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
		body, _ := fetch(t, client, "https://"+s.Address()+"/hello")
		testutil.Equal(t, "body", body, "api:0")
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})

	t.Run("should report the errors of the http server to the logger adaptor", func(t *testing.T) {
		certFile, keyFile, _ := certificate(t)
		a, r := newApp(t, map[string]any{rest.ServerConfigPath + ".api": apiConfig(map[string]any{
			standard.ConfigFieldCertFile: certFile,
			standard.ConfigFieldKeyFile:  keyFile,
			standard.ConfigFieldLogger:   "rec",
		})})
		s, _ := get(t, a, "api")
		done := running(t, s, context.Background())
		resp, err := http.Get("http://" + s.Address() + "/hello")
		testutil.NoError(t, err)
		_ = resp.Body.Close()
		testutil.Equal(t, "status", resp.StatusCode, http.StatusBadRequest)
		for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(time.Millisecond) {
			if _, errs := r.recorded(); len(errs) > 0 {
				if !strings.Contains(errs[0].Error(), "TLS handshake error") || strings.HasSuffix(errs[0].Error(), "\n") {
					t.Errorf("error = %q, want the TLS handshake error, on one line", errs[0])
				}
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("no error reported")
			}
		}
		testutil.NoError(t, s.Close())
		testutil.NoError(t, wait(t, done))
	})
}
