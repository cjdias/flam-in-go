package consul

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	flamjson "github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/consul"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// token is the ACL token of the sources of the tests.
const token = "consul-acl-token"

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// noEnv clears the environment variables the Consul client reads, for the
// test.
func noEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"CONSUL_HTTP_ADDR", "CONSUL_HTTP_TOKEN", "CONSUL_HTTP_TOKEN_FILE", "CONSUL_HTTP_AUTH",
		"CONSUL_HTTP_SSL", "CONSUL_HTTP_SSL_VERIFY", "CONSUL_CACERT", "CONSUL_CAPATH",
		"CONSUL_CLIENT_CERT", "CONSUL_CLIENT_KEY", "CONSUL_TLS_SERVER_NAME", "CONSUL_NAMESPACE",
		"CONSUL_PARTITION",
	} {
		t.Setenv(name, "")
		testutil.NoError(t, os.Unsetenv(name))
	}
}

// allBundles returns the bundles of the configuration feature with the
// json and yaml parsers, and the consul source driver.
func allBundles() []flam.IBundle {
	return []flam.IBundle{config.NewBundle(), flamjson.NewBundle(), yaml.NewBundle(), consul.NewBundle()}
}

// newApp returns a testutil.App providing a flam.IConfig holding the
// defaults of the configurable bundles and the values, with the bundles
// registered: the one of the configuration bundle when registered, or a
// fake one otherwise.
func newApp(t *testing.T, values container.Bag, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	noEnv(t)
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values.All() {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	if app.HasBundle(config.NewBundle().Id()) {
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	} else {
		testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	}
	return app
}

// creator returns the single source creator of the app.
func creator(t *testing.T, app *testutil.App) flam.IConfigSourceCreator {
	t.Helper()
	var creators []flam.IConfigSourceCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.IConfigSourceCreator `group:"flam.config.sources.creator"`
	}) {
		creators = args.Creators
	}))
	testutil.Equal(t, "creators", len(creators), 1)
	return creators[0]
}

// create creates a source of id "consul" from the fields with the creator
// of the app, closed at the end of the test.
func create(t *testing.T, app *testutil.App, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "consul", factory.ConfigFieldDriver: consul.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	s, err := creator(t, app).Create(cfg)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// mustCreate creates the reloadable source of the server and the fields,
// with every bundle.
func mustCreate(t *testing.T, srv *server, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	all := container.Bag{consul.ConfigFieldAddress: srv.URL}
	for key, value := range fields.All() {
		all[key] = value
	}
	s, err := create(t, newApp(t, nil, allBundles()...), all)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// request is a request received by the fake server.
type request struct {
	path  string
	query string
	token string
}

// server is a fake Consul agent serving the KV HTTP API of its pairs, with
// the X-Consul-Index header holding its index, unless zero.
type server struct {
	*httptest.Server

	mu       sync.Mutex
	pairs    map[string]string
	index    uint64
	status   int // answered instead of the pairs, unless zero
	delay    time.Duration
	requests []request
}

// newServer returns a started fake server of the pairs, at index 1.
func newServer(t *testing.T, pairs map[string]string) *server {
	t.Helper()
	s := &server{pairs: pairs, index: 1}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, request{
		path: r.URL.Path, query: r.URL.RawQuery, token: r.Header.Get("X-Consul-Token"),
	})
	status, delay, index := s.status, s.delay, s.index
	_, key, found := strings.Cut(r.URL.Path, "/v1/kv/")
	type pair struct {
		Key         string
		Value       []byte
		ModifyIndex uint64
	}
	var pairs []pair
	for k, v := range s.pairs {
		if k == key || (r.URL.Query().Has("recurse") && strings.HasPrefix(k, key)) {
			pairs = append(pairs, pair{Key: k, Value: []byte(v), ModifyIndex: index})
		}
	}
	s.mu.Unlock()

	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
	}
	if index > 0 {
		w.Header().Set("X-Consul-Index", strconv.FormatUint(index, 10))
	}
	switch {
	case status != 0:
		w.WriteHeader(status)
		_, _ = w.Write([]byte("boom"))
		return
	case !found || len(pairs) == 0:
		w.WriteHeader(http.StatusNotFound)
		return
	}
	slices.SortFunc(pairs, func(a, b pair) int { return strings.Compare(a.Key, b.Key) })
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(pairs)
}

// put stores the value of the key and, when bump, raises the index.
func (s *server) put(key, value string, bump bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pairs[key] = value
	if bump {
		s.index++
	}
}

// remove deletes the key and raises the index.
func (s *server) remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pairs, key)
	s.index++
}

// set changes the index, status and delay of the server.
func (s *server) set(index uint64, status int, delay time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.index, s.status, s.delay = index, status, delay
}

// last returns the last request received.
func (s *server) last() request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests[len(s.requests)-1]
}

// count returns the number of requests received.
func (s *server) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

// countParser is a json parser counting its calls.
type countParser struct {
	flam.IConfigParser
	mu    sync.Mutex
	calls int
}

// countingApp returns an app of every bundle whose json parser counts its
// calls.
func countingApp(t *testing.T) (*testutil.App, *countParser) {
	t.Helper()
	app := newApp(t, nil, allBundles()...)
	counter := &countParser{}
	testutil.NoError(t, app.Decorate(func(f flam.IConfigParserFactory) flam.IConfigParserFactory {
		return parserFactory{IConfigParserFactory: f, counter: counter}
	}))
	return app, counter
}

// parserFactory is a parser factory returning the counting parser in place
// of the json one.
type parserFactory struct {
	flam.IConfigParserFactory
	counter *countParser
}

func (f parserFactory) Get(id string) (flam.IConfigParser, error) {
	parser, err := f.IConfigParserFactory.Get(id)
	if err == nil && id == flamjson.Id {
		f.counter.IConfigParser = parser
		return f.counter, nil
	}
	return parser, err
}

func (p *countParser) Parse(reader io.Reader) (flam.IBag, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	return p.IConfigParser.Parse(reader)
}

// parses returns the number of calls of the parser.
func (p *countParser) parses() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

// checkError checks that the error wraps the sentinel, and is a
// *flam.Error of the context, holding no token.
func checkError(t *testing.T, err error, sentinel error, ctx map[string]any) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	testutil.Context(t, e, ctx)
	if strings.Contains(err.Error(), token) {
		t.Fatalf("error = %v, holds the token", err)
	}
}

// keyContext returns the context of an error of the key.
func keyContext(key string) map[string]any {
	return map[string]any{consul.ErrorFieldID: "consul", consul.ErrorFieldKey: key}
}

// certificate writes a self-signed certificate of 127.0.0.1, and its key,
// to PEM files of a temporary directory, returning their paths and the
// TLS certificate.
func certificate(t *testing.T) (certFile, keyFile string, cert tls.Certificate) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testutil.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "flam test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:              []string{"localhost"},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	testutil.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	testutil.NoError(t, err)

	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	testutil.NoError(t, os.WriteFile(certFile, certPEM, 0o600))
	testutil.NoError(t, os.WriteFile(keyFile, keyPEM, 0o600))
	cert, err = tls.X509KeyPair(certPEM, keyPEM)
	testutil.NoError(t, err)
	return certFile, keyFile, cert
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{consul.Driver, "flam.config.sources.driver.consul"},
			{consul.ConfigFieldPriority, "priority"},
			{consul.ConfigFieldReload, "reload"},
			{consul.ConfigFieldAddress, "address"},
			{consul.ConfigFieldToken, "token"},
			{consul.ConfigFieldDatacenter, "datacenter"},
			{consul.ConfigFieldKey, "key"},
			{consul.ConfigFieldRecursive, "recursive"},
			{consul.ConfigFieldParser, "parser"},
			{consul.ConfigFieldTimeout, "timeout"},
			{consul.ConfigFieldCAFile, "ca_file"},
			{consul.ConfigFieldCertFile, "cert_file"},
			{consul.ConfigFieldKeyFile, "key_file"},
			{consul.ConfigFieldSkipVerify, "skip_verify"},
			{consul.DefaultPriorityConfigPath, "flam.config.defaults.sources.consul.priority"},
			{consul.DefaultReloadConfigPath, "flam.config.defaults.sources.consul.reload"},
			{consul.DefaultAddressConfigPath, "flam.config.defaults.sources.consul.address"},
			{consul.DefaultTokenConfigPath, "flam.config.defaults.sources.consul.token"},
			{consul.DefaultDatacenterConfigPath, "flam.config.defaults.sources.consul.datacenter"},
			{consul.DefaultParserConfigPath, "flam.config.defaults.sources.consul.parser"},
			{consul.DefaultTimeoutConfigPath, "flam.config.defaults.sources.consul.timeout"},
			{consul.DefaultCAFileConfigPath, "flam.config.defaults.sources.consul.ca_file"},
			{consul.DefaultCertFileConfigPath, "flam.config.defaults.sources.consul.cert_file"},
			{consul.DefaultKeyFileConfigPath, "flam.config.defaults.sources.consul.key_file"},
			{consul.DefaultSkipVerifyConfigPath, "flam.config.defaults.sources.consul.skip_verify"},
			{consul.ErrorFieldID, "id"},
			{consul.ErrorFieldField, "field"},
			{consul.ErrorFieldKey, "key"},
			{consul.ErrorFieldParser, "parser"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			consul.ErrInvalidField:   "invalid consul config source field",
			consul.ErrParser:         "unable to get consul config source parser",
			consul.ErrMissingFactory: "factory not provided by the container",
			consul.ErrClient:         "unable to create consul config source client",
			consul.ErrRequest:        "consul config request failed",
			consul.ErrMissingKey:     "consul config key not found",
			consul.ErrParse:          "unable to parse consul config value",
			consul.ErrTree:           "unable to store consul config key",
		} {
			testutil.Equal(t, "message", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := consul.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.consul")
		if b == consul.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, consul.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag, container.Bag{"flam": container.Bag{"config": container.Bag{
			"defaults": container.Bag{"sources": container.Bag{"consul": container.Bag{
				"priority": 0, "reload": true, "address": "http://127.0.0.1:8500", "token": "",
				"datacenter": "", "parser": "json", "timeout": "10s", "ca_file": "", "cert_file": "",
				"key_file": "", "skip_verify": false,
			}}},
		}}})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := consul.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, consul.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		creator(t, newApp(t, nil, consul.NewBundle()))
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := consul.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creator(t, newApp(t, nil, consul.NewBundle()))
	for driver, want := range map[string]bool{
		consul.Driver:                     true,
		"":                                false,
		"consul":                          false,
		"flam.config.sources.driver.rest": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
}

func Test_Creator_InvalidFields(t *testing.T) {
	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			consul.ConfigFieldPriority:   {"1", 1.5},
			consul.ConfigFieldReload:     {"yes"},
			consul.ConfigFieldAddress:    {1, "", "consul:8500", "ftp://consul", "http://", "http://%zz", "http://consul?a=1", "http://consul#top"},
			consul.ConfigFieldToken:      {1},
			consul.ConfigFieldDatacenter: {1},
			consul.ConfigFieldKey:        {nil, 1, "", "/config"},
			consul.ConfigFieldRecursive:  {"yes"},
			consul.ConfigFieldParser:     {1, ""},
			consul.ConfigFieldTimeout:    {"soon", "-1s", 10},
			consul.ConfigFieldCAFile:     {1},
			consul.ConfigFieldCertFile:   {1},
			consul.ConfigFieldKeyFile:    {1},
			consul.ConfigFieldSkipVerify: {"yes"},
		} {
			for _, value := range values {
				fields := container.Bag{consul.ConfigFieldKey: "config", field: value}
				if value == nil {
					delete(fields, field)
				}
				_, err := create(t, newApp(t, nil, allBundles()...), fields)
				testutil.Error(t, err, consul.ErrInvalidField,
					"unable to create consul config source: invalid consul config source field",
					map[string]any{consul.ErrorFieldID: "consul", consul.ErrorFieldField: field})
			}
		}
	})

	t.Run("should refuse a certificate file without key file, and the reverse", func(t *testing.T) {
		for field, other := range map[string]string{
			consul.ConfigFieldKeyFile:  consul.ConfigFieldCertFile,
			consul.ConfigFieldCertFile: consul.ConfigFieldKeyFile,
		} {
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				consul.ConfigFieldKey: "config", other: "file.pem",
			})
			testutil.Error(t, err, consul.ErrInvalidField,
				"unable to create consul config source: invalid consul config source field",
				map[string]any{consul.ErrorFieldID: "consul", consul.ErrorFieldField: field})
		}
	})

	t.Run("should refuse the invalid defaults", func(t *testing.T) {
		for path, field := range map[string]string{
			consul.DefaultPriorityConfigPath:   consul.ConfigFieldPriority,
			consul.DefaultReloadConfigPath:     consul.ConfigFieldReload,
			consul.DefaultAddressConfigPath:    consul.ConfigFieldAddress,
			consul.DefaultTokenConfigPath:      consul.ConfigFieldToken,
			consul.DefaultDatacenterConfigPath: consul.ConfigFieldDatacenter,
			consul.DefaultParserConfigPath:     consul.ConfigFieldParser,
			consul.DefaultTimeoutConfigPath:    consul.ConfigFieldTimeout,
			consul.DefaultCAFileConfigPath:     consul.ConfigFieldCAFile,
			consul.DefaultCertFileConfigPath:   consul.ConfigFieldCertFile,
			consul.DefaultKeyFileConfigPath:    consul.ConfigFieldKeyFile,
			consul.DefaultSkipVerifyConfigPath: consul.ConfigFieldSkipVerify,
		} {
			app := newApp(t, container.Bag{path: []any{}}, consul.NewBundle())
			_, err := create(t, app, container.Bag{consul.ConfigFieldKey: "config"})
			testutil.Error(t, err, consul.ErrInvalidField,
				"unable to create consul config source: invalid consul config source field",
				map[string]any{consul.ErrorFieldID: "consul", consul.ErrorFieldField: field})
		}
	})

	t.Run("should take the built-in defaults without the bundle defaults", func(t *testing.T) {
		noEnv(t)
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: container.Bag{}} }))
		testutil.NoError(t, app.RegisterBundle(consul.NewBundle()))
		// the built-in parser is json, which needs the parser factory
		_, err := create(t, app, container.Bag{consul.ConfigFieldKey: "config"})
		checkError(t, err, consul.ErrMissingFactory, map[string]any{
			consul.ErrorFieldID: "consul", consul.ErrorFieldParser: "json",
		})
	})
}

func Test_Creator_Dependencies(t *testing.T) {
	t.Run("should fail without parser factory", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, consul.NewBundle()), container.Bag{consul.ConfigFieldKey: "config"})
		testutil.Error(t, err, consul.ErrParser,
			"unable to create consul config source: unable to get consul config source parser: factory not provided by the container",
			map[string]any{consul.ErrorFieldID: "consul", consul.ErrorFieldParser: "json"})
		checkError(t, err, consul.ErrMissingFactory, map[string]any{
			consul.ErrorFieldID: "consul", consul.ErrorFieldParser: "json",
		})
	})

	t.Run("should not need the parser factory for the string values of a tree", func(t *testing.T) {
		srv := newServer(t, map[string]string{"app/a": "1"})
		s, err := create(t, newApp(t, nil, consul.NewBundle()), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "app",
			consul.ConfigFieldRecursive: true, consul.ConfigFieldParser: "",
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "1"}))
	})

	t.Run("should fail with an unknown parser", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldKey: "config", consul.ConfigFieldParser: "toml",
		})
		checkError(t, err, consul.ErrParser, map[string]any{
			consul.ErrorFieldID: "consul", consul.ErrorFieldParser: "toml",
		})
	})

	t.Run("should fail when the client cannot be created", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldKey: "config", consul.ConfigFieldCAFile: filepath.Join(t.TempDir(), "missing.pem"),
		})
		checkError(t, err, consul.ErrClient, map[string]any{consul.ErrorFieldID: "consul"})
		if !strings.HasPrefix(err.Error(), "unable to create consul config source: unable to create consul config source client: ") {
			t.Fatalf("Error() = %q", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// reading
// ---------------------------------------------------------------------------

func Test_Source_Key(t *testing.T) {
	t.Run("should read and parse the key", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config/app.json": `{"server": {"port": 8080}}`})
		s := mustCreate(t, srv, container.Bag{
			consul.ConfigFieldKey: "config/app.json", consul.ConfigFieldPriority: 5,
		})
		testutil.Equal(t, "Id()", s.Id(), "consul")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
		testutil.Equal(t, "request", srv.last(), request{path: "/v1/kv/config/app.json"})
	})

	t.Run("should parse with the configured parser", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": "a:\n  b: 1\n"})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config", consul.ConfigFieldParser: yaml.Id})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": container.Bag{"b": 1}}))
	})

	t.Run("should send the token, the datacenter and the path prefix", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{}`})
		mustCreate(t, srv, container.Bag{
			consul.ConfigFieldAddress: srv.URL + "/gateway/", consul.ConfigFieldKey: "config",
			consul.ConfigFieldToken: token, consul.ConfigFieldDatacenter: "dc2",
		})
		testutil.Equal(t, "request", srv.last(), request{path: "/gateway/v1/kv/config", query: "dc=dc2", token: token})
	})

	t.Run("should take the token of the environment when not configured", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{}`})
		app := newApp(t, nil, allBundles()...)
		t.Setenv("CONSUL_HTTP_TOKEN", "from-env")
		_, err := create(t, app, container.Bag{consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config"})
		testutil.NoError(t, err)
		testutil.Equal(t, "token", srv.last().token, "from-env")
	})

	t.Run("should not be reloadable when its reload is deactivated", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		s, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config", consul.ConfigFieldReload: false,
		})
		testutil.NoError(t, err)
		if _, ok := s.(flam.IConfigReloadableSource); ok {
			t.Fatal("the source is reloadable")
		}
		testutil.Equal(t, "Id()", s.Id(), "consul")
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.NoError(t, s.Close())
	})

	t.Run("should take the reload and priority defaults", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{}`})
		app := newApp(t, container.Bag{
			consul.DefaultReloadConfigPath: false, consul.DefaultPriorityConfigPath: 7,
			consul.DefaultAddressConfigPath: srv.URL,
		}, allBundles()...)
		s, err := create(t, app, container.Bag{consul.ConfigFieldKey: "config"})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), 7)
		if _, ok := s.(flam.IConfigReloadableSource); ok {
			t.Fatal("the source is reloadable")
		}
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config"})
		testutil.NoError(t, s.Bag().Set("a", 2))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})
}

func Test_Source_Tree(t *testing.T) {
	t.Run("should merge the parsed values of the keys under the prefix", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"app/":            "",
			"app/db":          `{"user": "app"}`,
			"app/db/":         "",
			"app/db/primary":  `{"host": "db1", "port": 5432}`,
			"app/http/server": `{"port": 8080}`,
			"application":     `{"other": true}`,
		})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "app", consul.ConfigFieldRecursive: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{
			"db": container.Bag{
				"user":    "app",
				"primary": container.Bag{"host": "db1", "port": 5432},
			},
			"http": container.Bag{"server": container.Bag{"port": 8080}},
		}))
		testutil.Equal(t, "request", srv.last(), request{path: "/v1/kv/app/", query: "recurse="})
	})

	t.Run("should store the string values without parser", func(t *testing.T) {
		srv := newServer(t, map[string]string{
			"app/db/host": "db1", "app/db/port": "5432", "app/name": "",
		})
		s := mustCreate(t, srv, container.Bag{
			consul.ConfigFieldKey: "app/", consul.ConfigFieldRecursive: true, consul.ConfigFieldParser: "",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{
			"db": container.Bag{"host": "db1", "port": "5432"}, "name": "",
		}))
	})

	t.Run("should be empty for an empty tree", func(t *testing.T) {
		srv := newServer(t, map[string]string{})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "app", consul.ConfigFieldRecursive: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should fail for a key that cannot be stored", func(t *testing.T) {
		for name, pairs := range map[string]map[string]string{
			"app/a//b": {"app/a//b": "1"},
			"app/a/b":  {"app/a": "1", "app/a/b": "2"},
		} {
			srv := newServer(t, pairs)
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "app",
				consul.ConfigFieldRecursive: true, consul.ConfigFieldParser: "",
			})
			checkError(t, err, consul.ErrTree, keyContext(name))
			if !errors.Is(err, container.ErrInvalidPath) && !errors.Is(err, container.ErrPathConflict) {
				t.Fatalf("error = %v, want the error of the bag", err)
			}
		}
	})

	t.Run("should fail for a parsed value conflicting with a string", func(t *testing.T) {
		srv := newServer(t, map[string]string{"app/a": `{"b": 1}`, "app/a/b/c": `{"d": 1}`})
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "app", consul.ConfigFieldRecursive: true,
		})
		checkError(t, err, consul.ErrTree, keyContext("app/a/b/c"))
	})

	t.Run("should fail for a value the parser refuses", func(t *testing.T) {
		srv := newServer(t, map[string]string{"app/a": `{}`, "app/b": `[1]`})
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "app", consul.ConfigFieldRecursive: true,
		})
		checkError(t, err, consul.ErrParse, map[string]any{
			consul.ErrorFieldID: "consul", consul.ErrorFieldKey: "app/b", consul.ErrorFieldParser: "json",
		})
	})
}

func Test_Source_Errors(t *testing.T) {
	t.Run("should fail for a missing key", func(t *testing.T) {
		srv := newServer(t, map[string]string{"other": `{}`})
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config",
		})
		testutil.Error(t, err, consul.ErrMissingKey,
			"unable to create consul config source: consul config key not found", keyContext("config"))
	})

	t.Run("should fail for a value the parser refuses", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{`})
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config",
		})
		checkError(t, err, consul.ErrParse, map[string]any{
			consul.ErrorFieldID: "consul", consul.ErrorFieldKey: "config", consul.ErrorFieldParser: "json",
		})
	})

	t.Run("should fail for an unexpected status", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{}`})
		srv.set(1, http.StatusForbidden, 0)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config", consul.ConfigFieldToken: token,
		})
		checkError(t, err, consul.ErrRequest, keyContext("config"))
		if !strings.Contains(err.Error(), "403") {
			t.Fatalf("Error() = %q, want the status", err.Error())
		}
	})

	t.Run("should fail for an agent that cannot be reached", func(t *testing.T) {
		srv := newServer(t, nil)
		srv.Close()
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config",
		})
		checkError(t, err, consul.ErrRequest, keyContext("config"))
	})

	t.Run("should bound the request with the timeout", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{}`})
		srv.set(1, 0, time.Minute)
		start := time.Now()
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config", consul.ConfigFieldTimeout: "50ms",
		})
		checkError(t, err, consul.ErrRequest, keyContext("config"))
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Fatalf("Create() took %v", elapsed)
		}
	})

	t.Run("should not bound the request with a zero timeout", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		srv.set(1, 0, 100*time.Millisecond)
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config", consul.ConfigFieldTimeout: time.Duration(0)})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})
}

// ---------------------------------------------------------------------------
// reload
// ---------------------------------------------------------------------------

func Test_Source_Reload(t *testing.T) {
	t.Run("should keep its values, without parsing, while the index is unchanged", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		app, counter := countingApp(t)
		s, err := create(t, app, container.Bag{consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config"})
		testutil.NoError(t, err)
		reloadable := s.(flam.IConfigReloadableSource)
		testutil.Equal(t, "parses", counter.parses(), 1)

		srv.put("config", `{"a": 2}`, false)
		changed, err := reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "parses", counter.parses(), 1)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.Equal(t, "requests", srv.count(), 2)
	})

	t.Run("should take the new values when the index changes", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config"})

		srv.put("config", `{"a": 2}`, true)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 2}))

		srv.put("other", `{}`, true)
		changed, err = s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should parse on every reload without index", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		srv.set(0, 0, 0)
		app, counter := countingApp(t)
		s, err := create(t, app, container.Bag{consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config"})
		testutil.NoError(t, err)
		reloadable := s.(flam.IConfigReloadableSource)

		changed, err := reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		srv.put("config", `{"a": 2}`, false)
		changed, err = reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "parses", counter.parses(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 2}))
	})

	t.Run("should reload a tree", func(t *testing.T) {
		srv := newServer(t, map[string]string{"app/a": "1", "app/b": "2"})
		s := mustCreate(t, srv, container.Bag{
			consul.ConfigFieldKey: "app", consul.ConfigFieldRecursive: true, consul.ConfigFieldParser: "",
		})
		srv.remove("app/b")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "1"}))
	})

	t.Run("should keep its values when the reload fails", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config"})

		for name, change := range map[string]func(){
			"missing": func() { srv.remove("config") },
			"invalid": func() { srv.put("config", `{`, true) },
			"status":  func() { srv.set(9, http.StatusInternalServerError, 0) },
		} {
			change()
			changed, err := s.Reload()
			if err == nil || changed {
				t.Fatalf("%s: Reload() = %v, %v, want an error", name, changed, err)
			}
			var e *flam.Error
			if !errors.As(err, &e) || !strings.HasPrefix(err.Error(), "unable to reload consul config source: ") {
				t.Fatalf("%s: Reload() = %v", name, err)
			}
			testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		}

		srv.set(10, 0, 0)
		srv.put("config", `{"a": 3}`, true)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 3}))
	})

	t.Run("should remain usable once closed", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config"})
		testutil.NoError(t, s.Close())
		srv.put("config", `{"a": 2}`, true)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 0}`})
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config"})
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				srv.put("config", `{"a": `+strconv.Itoa(i)+`}`, true)
				_, err := s.Reload()
				testutil.NoError(t, err)
				_ = s.Bag()
			})
		}
		wg.Wait()
	})

	t.Run("should be reloaded by the configuration", func(t *testing.T) {
		srv := newServer(t, map[string]string{"config": `{"a": 1}`})
		app := newApp(t, container.Bag{
			config.SourceConfigPath: container.Bag{"remote": container.Bag{
				factory.ConfigFieldDriver: consul.Driver,
				consul.ConfigFieldAddress: srv.URL,
				consul.ConfigFieldKey:     "config",
			}},
		}, allBundles()...)
		testutil.NoError(t, app.Invoke(func(c flam.IConfig, f flam.IConfigSourceFactory) error {
			s, err := f.Get("remote")
			if err != nil {
				return err
			}
			if err := c.AddSource(s); err != nil {
				return err
			}
			testutil.Equal(t, "a", c.Int("a"), 1)
			srv.put("config", `{"a": 2}`, true)
			if err := c.ReloadSources(); err != nil {
				return err
			}
			testutil.Equal(t, "a", c.Int("a"), 2)
			return f.Close()
		}))
	})
}

// ---------------------------------------------------------------------------
// TLS
// ---------------------------------------------------------------------------

func Test_Source_TLS(t *testing.T) {
	certFile, keyFile, cert := certificate(t)
	pool := x509.NewCertPool()
	pool.AddCert(cert.Leaf)

	newTLSServer := func(t *testing.T, mutual bool) *server {
		t.Helper()
		s := &server{pairs: map[string]string{"config": `{"tls": true}`}, index: 1}
		s.Server = httptest.NewUnstartedServer(http.HandlerFunc(s.serve))
		s.Config.ErrorLog = log.New(io.Discard, "", 0) // the refused handshakes
		s.TLS = &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
		if mutual {
			s.TLS.ClientAuth = tls.RequireAndVerifyClientCert
			s.TLS.ClientCAs = pool
		}
		s.StartTLS()
		t.Cleanup(s.Close)
		return s
	}

	t.Run("should trust the certificate authorities of the file", func(t *testing.T) {
		srv := newTLSServer(t, false)
		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config", consul.ConfigFieldCAFile: certFile})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"tls": true}))
	})

	t.Run("should present the client certificate", func(t *testing.T) {
		srv := newTLSServer(t, true)
		s := mustCreate(t, srv, container.Bag{
			consul.ConfigFieldKey: "config", consul.ConfigFieldCAFile: certFile,
			consul.ConfigFieldCertFile: certFile, consul.ConfigFieldKeyFile: keyFile,
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"tls": true}))

		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config", consul.ConfigFieldCAFile: certFile,
		})
		checkError(t, err, consul.ErrRequest, keyContext("config"))
	})

	t.Run("should skip the verification when asked to", func(t *testing.T) {
		srv := newTLSServer(t, false)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			consul.ConfigFieldAddress: srv.URL, consul.ConfigFieldKey: "config",
		})
		checkError(t, err, consul.ErrRequest, keyContext("config"))

		s := mustCreate(t, srv, container.Bag{consul.ConfigFieldKey: "config", consul.ConfigFieldSkipVerify: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"tls": true}))
	})
}
