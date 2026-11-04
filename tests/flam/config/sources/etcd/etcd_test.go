package etcd

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"go.etcd.io/etcd/client/pkg/v3/transport"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/server/v3/embed"
	"go.uber.org/dig"
	"go.uber.org/zap"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/etcd"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// embedded etcd
// ---------------------------------------------------------------------------

// password is the password of the user of the authenticated cluster.
const password = "etcd-s3cret-password"

// cluster is an embedded etcd server, with a client writing its keys.
type cluster struct {
	endpoint string
	client   *clientv3.Client
}

// freePort returns a local port free when called.
func freePort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	testutil.NoError(t, err)
	defer func() { _ = listener.Close() }()
	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

// startCluster starts an embedded etcd server on random local ports, in a
// temporary directory, with the TLS of its clients when tls is not empty,
// stopped at the end of the test.
func startCluster(t *testing.T, tls transport.TLSInfo) *cluster {
	t.Helper()
	scheme := "http"
	if !tls.Empty() {
		scheme = "https"
	}
	clientURL := url.URL{Scheme: scheme, Host: "127.0.0.1:" + freePort(t)}
	peerURL := url.URL{Scheme: "http", Host: "127.0.0.1:" + freePort(t)}

	cfg := embed.NewConfig()
	cfg.Name = "flam"
	cfg.Dir = t.TempDir()
	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(zap.NewNop())
	cfg.ListenClientUrls = []url.URL{clientURL}
	cfg.AdvertiseClientUrls = []url.URL{clientURL}
	cfg.ListenPeerUrls = []url.URL{peerURL}
	cfg.AdvertisePeerUrls = []url.URL{peerURL}
	cfg.InitialCluster = cfg.Name + "=" + peerURL.String()
	cfg.ClientTLSInfo = tls

	server, err := embed.StartEtcd(cfg)
	testutil.NoError(t, err)
	t.Cleanup(server.Close)
	select {
	case <-server.Server.ReadyNotify():
	case <-time.After(time.Minute):
		t.Fatal("the embedded etcd server did not start")
	}

	clientConfig := clientv3.Config{Endpoints: []string{clientURL.String()}, Logger: zap.NewNop()}
	if !tls.Empty() {
		clientConfig.TLS, err = tls.ClientConfig()
		testutil.NoError(t, err)
	}
	client, err := clientv3.New(clientConfig)
	testutil.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return &cluster{endpoint: clientURL.String(), client: client}
}

// shared is the cluster of the tests without TLS nor authentication, each
// test writing keys of its own.
var (
	sharedOnce sync.Once
	shared     *cluster
)

// put stores the value of the key.
func (c *cluster) put(t *testing.T, key, value string) {
	t.Helper()
	_, err := c.client.Put(context.Background(), key, value)
	testutil.NoError(t, err)
}

// remove deletes the key.
func (c *cluster) remove(t *testing.T, key string) {
	t.Helper()
	_, err := c.client.Delete(context.Background(), key)
	testutil.NoError(t, err)
}

// namespace returns a key prefix of its own for the test.
func namespace(t *testing.T) string {
	return "/" + strings.ReplaceAll(t.Name(), "/", "_") + "/"
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// allBundles returns the bundles of the configuration feature with the
// json and yaml parsers, and the etcd source driver.
func allBundles() []flam.IBundle {
	return []flam.IBundle{config.NewBundle(), json.NewBundle(), yaml.NewBundle(), etcd.NewBundle()}
}

// newApp returns a testutil.App providing a flam.IConfig holding the
// defaults of the configurable bundles and the values, with the bundles
// registered: the one of the configuration bundle when registered, or a
// fake one otherwise.
func newApp(t *testing.T, values container.Bag, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
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

// create creates a source of id "etcd" from the fields with the creator of
// the app, closed at the end of the test.
func create(t *testing.T, app *testutil.App, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "etcd", factory.ConfigFieldDriver: etcd.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	s, err := creator(t, app).Create(cfg)
	if err == nil {
		t.Cleanup(func() { _ = s.Close() })
	}
	return s, err
}

// mustCreate creates the reloadable source of the cluster and the fields,
// with every bundle.
func mustCreate(t *testing.T, c *cluster, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	all := container.Bag{etcd.ConfigFieldEndpoints: []any{c.endpoint}}
	for key, value := range fields.All() {
		all[key] = value
	}
	s, err := create(t, newApp(t, nil, allBundles()...), all)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// countParser is a json parser counting its calls.
type countParser struct {
	flam.IConfigParser
	mu    sync.Mutex
	calls int
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

// parserFactory is a parser factory returning the counting parser in place
// of the json one.
type parserFactory struct {
	flam.IConfigParserFactory
	counter *countParser
}

func (f parserFactory) Get(id string) (flam.IConfigParser, error) {
	parser, err := f.IConfigParserFactory.Get(id)
	if err == nil && id == json.Id {
		f.counter.IConfigParser = parser
		return f.counter, nil
	}
	return parser, err
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

// checkError checks that the error wraps the sentinel, and is a
// *flam.Error of the context, holding no password.
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
	if strings.Contains(err.Error(), password) {
		t.Fatalf("error = %v, holds the password", err)
	}
}

// keyContext returns the context of an error of the key.
func keyContext(key string) map[string]any {
	return map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldKey: key}
}

// certificate writes a self-signed certificate of 127.0.0.1, and its key,
// to PEM files of a temporary directory, returning their paths.
func certificate(t *testing.T) (certFile, keyFile string) {
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
	testutil.NoError(t, os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))
	testutil.NoError(t, os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	return certFile, keyFile
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{etcd.Driver, "flam.config.sources.driver.etcd"},
			{etcd.ConfigFieldPriority, "priority"},
			{etcd.ConfigFieldReload, "reload"},
			{etcd.ConfigFieldEndpoints, "endpoints"},
			{etcd.ConfigFieldUsername, "username"},
			{etcd.ConfigFieldPassword, "password"},
			{etcd.ConfigFieldKey, "key"},
			{etcd.ConfigFieldRecursive, "recursive"},
			{etcd.ConfigFieldParser, "parser"},
			{etcd.ConfigFieldTimeout, "timeout"},
			{etcd.ConfigFieldCAFile, "ca_file"},
			{etcd.ConfigFieldCertFile, "cert_file"},
			{etcd.ConfigFieldKeyFile, "key_file"},
			{etcd.ConfigFieldSkipVerify, "skip_verify"},
			{etcd.DefaultPriorityConfigPath, "flam.config.defaults.sources.etcd.priority"},
			{etcd.DefaultReloadConfigPath, "flam.config.defaults.sources.etcd.reload"},
			{etcd.DefaultEndpointsConfigPath, "flam.config.defaults.sources.etcd.endpoints"},
			{etcd.DefaultUsernameConfigPath, "flam.config.defaults.sources.etcd.username"},
			{etcd.DefaultPasswordConfigPath, "flam.config.defaults.sources.etcd.password"},
			{etcd.DefaultParserConfigPath, "flam.config.defaults.sources.etcd.parser"},
			{etcd.DefaultTimeoutConfigPath, "flam.config.defaults.sources.etcd.timeout"},
			{etcd.DefaultCAFileConfigPath, "flam.config.defaults.sources.etcd.ca_file"},
			{etcd.DefaultCertFileConfigPath, "flam.config.defaults.sources.etcd.cert_file"},
			{etcd.DefaultKeyFileConfigPath, "flam.config.defaults.sources.etcd.key_file"},
			{etcd.DefaultSkipVerifyConfigPath, "flam.config.defaults.sources.etcd.skip_verify"},
			{etcd.ErrorFieldID, "id"},
			{etcd.ErrorFieldField, "field"},
			{etcd.ErrorFieldKey, "key"},
			{etcd.ErrorFieldParser, "parser"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			etcd.ErrInvalidField:   "invalid etcd config source field",
			etcd.ErrParser:         "unable to get etcd config source parser",
			etcd.ErrMissingFactory: "factory not provided by the container",
			etcd.ErrTLS:            "unable to load etcd config source certificates",
			etcd.ErrClient:         "unable to create etcd config source client",
			etcd.ErrRequest:        "etcd config request failed",
			etcd.ErrMissingKey:     "etcd config key not found",
			etcd.ErrParse:          "unable to parse etcd config value",
			etcd.ErrTree:           "unable to store etcd config key",
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
		b := etcd.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.etcd")
		if b == etcd.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, etcd.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag, container.Bag{"flam": container.Bag{"config": container.Bag{
			"defaults": container.Bag{"sources": container.Bag{"etcd": container.Bag{
				"priority": 0, "reload": true, "endpoints": []any{"127.0.0.1:2379"}, "username": "",
				"password": "", "parser": "json", "timeout": "10s", "ca_file": "", "cert_file": "",
				"key_file": "", "skip_verify": false,
			}}},
		}}})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := etcd.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, etcd.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		creator(t, newApp(t, nil, etcd.NewBundle()))
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := etcd.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creator(t, newApp(t, nil, etcd.NewBundle()))
	for driver, want := range map[string]bool{
		etcd.Driver:                         true,
		"":                                  false,
		"etcd":                              false,
		"flam.config.sources.driver.consul": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
}

func Test_Creator_InvalidFields(t *testing.T) {
	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			etcd.ConfigFieldPriority:   {"1", 1.5},
			etcd.ConfigFieldReload:     {"yes"},
			etcd.ConfigFieldEndpoints:  {"127.0.0.1:2379", []any{}, []string{}, []any{1}, []any{""}, []string{"a", ""}},
			etcd.ConfigFieldUsername:   {1},
			etcd.ConfigFieldPassword:   {1},
			etcd.ConfigFieldKey:        {nil, 1, ""},
			etcd.ConfigFieldRecursive:  {"yes"},
			etcd.ConfigFieldParser:     {1, ""},
			etcd.ConfigFieldTimeout:    {"soon", "-1s", 10},
			etcd.ConfigFieldCAFile:     {1},
			etcd.ConfigFieldCertFile:   {1},
			etcd.ConfigFieldKeyFile:    {1},
			etcd.ConfigFieldSkipVerify: {"yes"},
		} {
			for _, value := range values {
				fields := container.Bag{etcd.ConfigFieldKey: "config", field: value}
				if value == nil {
					delete(fields, field)
				}
				_, err := create(t, newApp(t, nil, allBundles()...), fields)
				testutil.Error(t, err, etcd.ErrInvalidField,
					"unable to create etcd config source: invalid etcd config source field",
					map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldField: field})
			}
		}
	})

	t.Run("should refuse a certificate file without key file, and the reverse", func(t *testing.T) {
		for field, other := range map[string]string{
			etcd.ConfigFieldKeyFile:  etcd.ConfigFieldCertFile,
			etcd.ConfigFieldCertFile: etcd.ConfigFieldKeyFile,
		} {
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				etcd.ConfigFieldKey: "config", other: "file.pem",
			})
			testutil.Error(t, err, etcd.ErrInvalidField,
				"unable to create etcd config source: invalid etcd config source field",
				map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldField: field})
		}
	})

	t.Run("should refuse the invalid defaults", func(t *testing.T) {
		for path, field := range map[string]string{
			etcd.DefaultPriorityConfigPath:   etcd.ConfigFieldPriority,
			etcd.DefaultReloadConfigPath:     etcd.ConfigFieldReload,
			etcd.DefaultEndpointsConfigPath:  etcd.ConfigFieldEndpoints,
			etcd.DefaultUsernameConfigPath:   etcd.ConfigFieldUsername,
			etcd.DefaultPasswordConfigPath:   etcd.ConfigFieldPassword,
			etcd.DefaultParserConfigPath:     etcd.ConfigFieldParser,
			etcd.DefaultTimeoutConfigPath:    etcd.ConfigFieldTimeout,
			etcd.DefaultCAFileConfigPath:     etcd.ConfigFieldCAFile,
			etcd.DefaultCertFileConfigPath:   etcd.ConfigFieldCertFile,
			etcd.DefaultKeyFileConfigPath:    etcd.ConfigFieldKeyFile,
			etcd.DefaultSkipVerifyConfigPath: etcd.ConfigFieldSkipVerify,
		} {
			app := newApp(t, container.Bag{path: container.Bag{}}, etcd.NewBundle())
			_, err := create(t, app, container.Bag{etcd.ConfigFieldKey: "config"})
			testutil.Error(t, err, etcd.ErrInvalidField,
				"unable to create etcd config source: invalid etcd config source field",
				map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldField: field})
		}
	})

	t.Run("should take the built-in defaults without the bundle defaults", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: container.Bag{}} }))
		testutil.NoError(t, app.RegisterBundle(etcd.NewBundle()))
		// the built-in parser is json, which needs the parser factory
		_, err := create(t, app, container.Bag{etcd.ConfigFieldKey: "config"})
		checkError(t, err, etcd.ErrMissingFactory, map[string]any{
			etcd.ErrorFieldID: "etcd", etcd.ErrorFieldParser: "json",
		})
	})
}

func Test_Creator_Dependencies(t *testing.T) {
	t.Run("should fail without parser factory", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, etcd.NewBundle()), container.Bag{etcd.ConfigFieldKey: "config"})
		testutil.Error(t, err, etcd.ErrParser,
			"unable to create etcd config source: unable to get etcd config source parser: factory not provided by the container",
			map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldParser: "json"})
	})

	t.Run("should not need the parser factory for the string values of a tree", func(t *testing.T) {
		c := sharedCluster(t)
		c.put(t, namespace(t)+"a", "1")
		s, err := create(t, newApp(t, nil, etcd.NewBundle()), container.Bag{
			etcd.ConfigFieldEndpoints: []string{c.endpoint}, etcd.ConfigFieldKey: namespace(t),
			etcd.ConfigFieldRecursive: true, etcd.ConfigFieldParser: "",
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "1"}))
	})

	t.Run("should fail with an unknown parser", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldKey: "config", etcd.ConfigFieldParser: "toml",
		})
		checkError(t, err, etcd.ErrParser, map[string]any{etcd.ErrorFieldID: "etcd", etcd.ErrorFieldParser: "toml"})
	})

	t.Run("should fail when the certificates cannot be loaded", func(t *testing.T) {
		dir := t.TempDir()
		notPEM := filepath.Join(dir, "not.pem")
		testutil.NoError(t, os.WriteFile(notPEM, []byte("not a certificate"), 0o600))
		for name, fields := range map[string]container.Bag{
			"missing ca":   {etcd.ConfigFieldCAFile: filepath.Join(dir, "missing.pem")},
			"invalid ca":   {etcd.ConfigFieldCAFile: notPEM},
			"invalid cert": {etcd.ConfigFieldCertFile: notPEM, etcd.ConfigFieldKeyFile: notPEM},
		} {
			fields[etcd.ConfigFieldKey] = "config"
			_, err := create(t, newApp(t, nil, allBundles()...), fields)
			checkError(t, err, etcd.ErrTLS, map[string]any{etcd.ErrorFieldID: "etcd"})
			if !strings.HasPrefix(err.Error(), "unable to create etcd config source: unable to load etcd config source certificates: ") {
				t.Fatalf("%s: Error() = %q", name, err.Error())
			}
		}
	})
}

// ---------------------------------------------------------------------------
// reading
// ---------------------------------------------------------------------------

// sharedCluster returns the cluster shared by the tests without TLS nor
// authentication, started once.
func sharedCluster(t *testing.T) *cluster {
	t.Helper()
	sharedOnce.Do(func() {
		shared = startShared()
	})
	if shared == nil {
		t.Fatal("the shared etcd server did not start")
	}
	return shared
}

// sharedStop stops the shared cluster, when started.
var sharedStop func()

// startShared starts the shared cluster, stopped by TestMain.
func startShared() *cluster {
	dir, err := os.MkdirTemp("", "flam-etcd-")
	if err != nil {
		return nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil
	}
	clientPort := listener.Addr().(*net.TCPAddr).Port
	listener2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil
	}
	peerPort := listener2.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	_ = listener2.Close()

	clientURL := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", clientPort)}
	peerURL := url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", peerPort)}
	cfg := embed.NewConfig()
	cfg.Name = "flam"
	cfg.Dir = dir
	cfg.ZapLoggerBuilder = embed.NewZapLoggerBuilder(zap.NewNop())
	cfg.ListenClientUrls = []url.URL{clientURL}
	cfg.AdvertiseClientUrls = []url.URL{clientURL}
	cfg.ListenPeerUrls = []url.URL{peerURL}
	cfg.AdvertisePeerUrls = []url.URL{peerURL}
	cfg.InitialCluster = cfg.Name + "=" + peerURL.String()
	server, err := embed.StartEtcd(cfg)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil
	}
	select {
	case <-server.Server.ReadyNotify():
	case <-time.After(time.Minute):
		server.Close()
		_ = os.RemoveAll(dir)
		return nil
	}
	client, err := clientv3.New(clientv3.Config{Endpoints: []string{clientURL.String()}, Logger: zap.NewNop()})
	if err != nil {
		server.Close()
		_ = os.RemoveAll(dir)
		return nil
	}
	sharedStop = func() {
		_ = client.Close()
		server.Close()
		_ = os.RemoveAll(dir)
	}
	return &cluster{endpoint: clientURL.String(), client: client}
}

func TestMain(m *testing.M) {
	code := m.Run()
	if sharedStop != nil {
		sharedStop()
	}
	os.Exit(code)
}

func Test_Source_Key(t *testing.T) {
	c := sharedCluster(t)

	t.Run("should read and parse the key", func(t *testing.T) {
		key := namespace(t) + "app.json"
		c.put(t, key, `{"server": {"port": 8080}}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key, etcd.ConfigFieldPriority: 5})
		testutil.Equal(t, "Id()", s.Id(), "etcd")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
	})

	t.Run("should parse with the configured parser", func(t *testing.T) {
		key := namespace(t) + "app.yaml"
		c.put(t, key, "a:\n  b: 1\n")
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key, etcd.ConfigFieldParser: yaml.Id})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": container.Bag{"b": 1}}))
	})

	t.Run("should read the key alone, not the keys it prefixes", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		c.put(t, key+"/b", `{"b": 1}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should not be reloadable when its reload is deactivated", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		s, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: key, etcd.ConfigFieldReload: false,
		})
		testutil.NoError(t, err)
		if _, ok := s.(flam.IConfigReloadableSource); ok {
			t.Fatal("the source is reloadable")
		}
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{}`)
		app := newApp(t, container.Bag{
			etcd.DefaultReloadConfigPath: false, etcd.DefaultPriorityConfigPath: 7,
			etcd.DefaultEndpointsConfigPath: []string{c.endpoint},
		}, allBundles()...)
		s, err := create(t, app, container.Bag{etcd.ConfigFieldKey: key})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), 7)
		if _, ok := s.(flam.IConfigReloadableSource); ok {
			t.Fatal("the source is reloadable")
		}
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key})
		testutil.NoError(t, s.Bag().Set("a", 2))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should not bound the request with a zero timeout", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key, etcd.ConfigFieldTimeout: "0s"})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})
}

func Test_Source_Tree(t *testing.T) {
	c := sharedCluster(t)

	t.Run("should merge the parsed values of the keys under the prefix", func(t *testing.T) {
		ns := namespace(t)
		c.put(t, ns+"app/", "")
		c.put(t, ns+"app/db", `{"user": "app"}`)
		c.put(t, ns+"app/db/primary", `{"host": "db1", "port": 5432}`)
		c.put(t, ns+"app/http/server", `{"port": 8080}`)
		c.put(t, ns+"application", `{"other": true}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: ns + "app", etcd.ConfigFieldRecursive: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{
			"db": container.Bag{
				"user":    "app",
				"primary": container.Bag{"host": "db1", "port": 5432},
			},
			"http": container.Bag{"server": container.Bag{"port": 8080}},
		}))
	})

	t.Run("should store the string values without parser", func(t *testing.T) {
		ns := namespace(t)
		c.put(t, ns+"db/host", "db1")
		c.put(t, ns+"db/port", "5432")
		c.put(t, ns+"name", "")
		s := mustCreate(t, c, container.Bag{
			etcd.ConfigFieldKey: ns, etcd.ConfigFieldRecursive: true, etcd.ConfigFieldParser: "",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{
			"db": container.Bag{"host": "db1", "port": "5432"}, "name": "",
		}))
	})

	t.Run("should be empty for an empty tree", func(t *testing.T) {
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: namespace(t), etcd.ConfigFieldRecursive: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should fail for a key that cannot be stored", func(t *testing.T) {
		for i, keys := range [][]string{{"a//b"}, {"a", "a/b"}} {
			ns := namespace(t) + strconv.Itoa(i) + "/"
			for _, key := range keys {
				c.put(t, ns+key, "1")
			}
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: ns,
				etcd.ConfigFieldRecursive: true, etcd.ConfigFieldParser: "",
			})
			checkError(t, err, etcd.ErrTree, keyContext(ns+keys[len(keys)-1]))
			if !errors.Is(err, container.ErrInvalidPath) && !errors.Is(err, container.ErrPathConflict) {
				t.Fatalf("error = %v, want the error of the bag", err)
			}
		}
	})

	t.Run("should fail for a parsed value conflicting with a string", func(t *testing.T) {
		ns := namespace(t)
		c.put(t, ns+"a", `{"b": 1}`)
		c.put(t, ns+"a/b/c", `{"d": 1}`)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: ns, etcd.ConfigFieldRecursive: true,
		})
		checkError(t, err, etcd.ErrTree, keyContext(ns+"a/b/c"))
	})

	t.Run("should fail for a value the parser refuses", func(t *testing.T) {
		ns := namespace(t)
		c.put(t, ns+"a", `{}`)
		c.put(t, ns+"b", `[1]`)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: ns, etcd.ConfigFieldRecursive: true,
		})
		checkError(t, err, etcd.ErrParse, map[string]any{
			etcd.ErrorFieldID: "etcd", etcd.ErrorFieldKey: ns + "b", etcd.ErrorFieldParser: "json",
		})
	})
}

func Test_Source_Errors(t *testing.T) {
	c := sharedCluster(t)

	t.Run("should fail for a missing key", func(t *testing.T) {
		key := namespace(t) + "missing"
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: key,
		})
		testutil.Error(t, err, etcd.ErrMissingKey,
			"unable to create etcd config source: etcd config key not found", keyContext(key))
	})

	t.Run("should fail for a value the parser refuses", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{`)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: key,
		})
		checkError(t, err, etcd.ErrParse, map[string]any{
			etcd.ErrorFieldID: "etcd", etcd.ErrorFieldKey: key, etcd.ErrorFieldParser: "json",
		})
	})

	t.Run("should fail for a cluster that cannot be reached, within the timeout", func(t *testing.T) {
		start := time.Now()
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{"127.0.0.1:" + freePort(t)}, etcd.ConfigFieldKey: "config",
			etcd.ConfigFieldTimeout: "200ms", etcd.ConfigFieldPassword: password,
		})
		checkError(t, err, etcd.ErrRequest, keyContext("config"))
		if elapsed := time.Since(start); elapsed > 10*time.Second {
			t.Fatalf("Create() took %v", elapsed)
		}
	})
}

// ---------------------------------------------------------------------------
// reload
// ---------------------------------------------------------------------------

func Test_Source_Reload(t *testing.T) {
	c := sharedCluster(t)

	t.Run("should keep its values, without parsing, while the keys are unchanged", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		app, counter := countingApp(t)
		s, err := create(t, app, container.Bag{etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: key})
		testutil.NoError(t, err)
		reloadable := s.(flam.IConfigReloadableSource)

		c.put(t, namespace(t)+"other", `{}`)
		changed, err := reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "parses", counter.parses(), 1)

		c.put(t, key, `{"a": 1}`)
		changed, err = reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "parses", counter.parses(), 2)

		c.put(t, key, `{"a": 2}`)
		changed, err = reloadable.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 2}))
	})

	t.Run("should see the deletes and puts of a tree", func(t *testing.T) {
		ns := namespace(t)
		c.put(t, ns+"a", "1")
		c.put(t, ns+"b", "2")
		s := mustCreate(t, c, container.Bag{
			etcd.ConfigFieldKey: ns, etcd.ConfigFieldRecursive: true, etcd.ConfigFieldParser: "",
		})
		c.remove(t, ns+"a")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"b": "2"}))

		c.put(t, ns+"c", "3")
		changed, err = s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"b": "2", "c": "3"}))
	})

	t.Run("should keep its values when the reload fails", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key})

		c.put(t, key, `{`)
		changed, err := s.Reload()
		checkError(t, err, etcd.ErrParse, map[string]any{
			etcd.ErrorFieldID: "etcd", etcd.ErrorFieldKey: key, etcd.ErrorFieldParser: "json",
		})
		testutil.Equal(t, "changed", changed, false)
		if !strings.HasPrefix(err.Error(), "unable to reload etcd config source: ") {
			t.Fatalf("Error() = %q", err.Error())
		}

		c.remove(t, key)
		_, err = s.Reload()
		checkError(t, err, etcd.ErrMissingKey, keyContext(key))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))

		c.put(t, key, `{"a": 3}`)
		changed, err = s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 3}))
	})

	t.Run("should fail to reload once closed", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key})
		testutil.NoError(t, s.Close())
		_, err := s.Reload()
		checkError(t, err, etcd.ErrRequest, keyContext(key))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 0}`)
		s := mustCreate(t, c, container.Bag{etcd.ConfigFieldKey: key})
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				c.put(t, key, `{"a": `+strconv.Itoa(i)+`}`)
				_, err := s.Reload()
				testutil.NoError(t, err)
				_ = s.Bag()
			})
		}
		wg.Wait()
	})

	t.Run("should be reloaded by the configuration", func(t *testing.T) {
		key := namespace(t) + "app"
		c.put(t, key, `{"a": 1}`)
		app := newApp(t, container.Bag{
			config.SourceConfigPath: container.Bag{"remote": container.Bag{
				factory.ConfigFieldDriver: etcd.Driver,
				etcd.ConfigFieldEndpoints: []any{c.endpoint},
				etcd.ConfigFieldKey:       key,
			}},
		}, allBundles()...)
		testutil.NoError(t, app.Invoke(func(cfg flam.IConfig, f flam.IConfigSourceFactory) error {
			s, err := f.Get("remote")
			if err != nil {
				return err
			}
			if err := cfg.AddSource(s); err != nil {
				return err
			}
			testutil.Equal(t, "a", cfg.Int("a"), 1)
			c.put(t, key, `{"a": 2}`)
			if err := cfg.ReloadSources(); err != nil {
				return err
			}
			testutil.Equal(t, "a", cfg.Int("a"), 2)
			return f.Close()
		}))
	})
}

// ---------------------------------------------------------------------------
// authentication
// ---------------------------------------------------------------------------

func Test_Source_Authentication(t *testing.T) {
	c := startCluster(t, transport.TLSInfo{})
	ctx := context.Background()
	_, err := c.client.UserAdd(ctx, "root", password)
	testutil.NoError(t, err)
	_, err = c.client.UserGrantRole(ctx, "root", "root")
	testutil.NoError(t, err)
	_, err = c.client.Put(ctx, "config", `{"auth": true}`)
	testutil.NoError(t, err)
	_, err = c.client.AuthEnable(ctx)
	testutil.NoError(t, err)

	t.Run("should authenticate with the user", func(t *testing.T) {
		s := mustCreate(t, c, container.Bag{
			etcd.ConfigFieldKey: "config", etcd.ConfigFieldUsername: "root", etcd.ConfigFieldPassword: password,
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"auth": true}))
	})

	t.Run("should fail for a password the cluster refuses", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: "config",
			etcd.ConfigFieldUsername: "root", etcd.ConfigFieldPassword: "wrong",
		})
		checkError(t, err, etcd.ErrClient, map[string]any{etcd.ErrorFieldID: "etcd"})
	})

	t.Run("should fail without the user", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: "config",
		})
		checkError(t, err, etcd.ErrRequest, keyContext("config"))
	})
}

// ---------------------------------------------------------------------------
// TLS
// ---------------------------------------------------------------------------

func Test_Source_TLS(t *testing.T) {
	certFile, keyFile := certificate(t)
	c := startCluster(t, transport.TLSInfo{
		CertFile: certFile, KeyFile: keyFile, TrustedCAFile: certFile, ClientCertAuth: true,
	})
	_, err := c.client.Put(context.Background(), "config", `{"tls": true}`)
	testutil.NoError(t, err)

	t.Run("should trust the certificate authorities and present the client certificate", func(t *testing.T) {
		s := mustCreate(t, c, container.Bag{
			etcd.ConfigFieldKey: "config", etcd.ConfigFieldCAFile: certFile,
			etcd.ConfigFieldCertFile: certFile, etcd.ConfigFieldKeyFile: keyFile,
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"tls": true}))
	})

	t.Run("should skip the verification when asked to", func(t *testing.T) {
		s := mustCreate(t, c, container.Bag{
			etcd.ConfigFieldKey: "config", etcd.ConfigFieldSkipVerify: true,
			etcd.ConfigFieldCertFile: certFile, etcd.ConfigFieldKeyFile: keyFile,
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"tls": true}))
	})

	t.Run("should talk TLS to an https endpoint, verifying it", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			etcd.ConfigFieldEndpoints: []any{c.endpoint}, etcd.ConfigFieldKey: "config",
			etcd.ConfigFieldTimeout: "500ms",
		})
		checkError(t, err, etcd.ErrRequest, keyContext("config"))
	})
}
