package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeConfig is a flam.IConfig reading Has, Get and Bag from a bag; its
// other methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

func (c fakeConfig) Bag(path string, def ...flam.IBag) flam.IBag { return c.bag.Bag(path, def...) }

// allBundles returns the bundles of the configuration feature with the
// json and yaml parsers, and the rest source driver.
func allBundles() []flam.IBundle {
	return []flam.IBundle{config.NewBundle(), json.NewBundle(), yaml.NewBundle(), rest.NewBundle()}
}

// newApp returns a testutil.App providing a fake flam.IConfig holding the
// defaults of the configurable bundles and the values, with the bundles
// registered.
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
	// the config bundle provides the configuration: give it the defaults
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

// create creates a source of id "rest" from the fields with the creator of
// the app.
func create(t *testing.T, app *testutil.App, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "rest", factory.ConfigFieldDriver: rest.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creator(t, app).Create(cfg)
}

// mustCreate creates the source of the fields.
func mustCreate(t *testing.T, app *testutil.App, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	s, err := create(t, app, fields)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// endpoint is a test HTTP server answering with its current response, and
// recording the requests it receives.
type endpoint struct {
	*httptest.Server

	mu          sync.Mutex
	status      int
	contentType string
	body        string
	requests    []*http.Request
}

// newEndpoint returns a started endpoint answering 200 with the JSON body.
func newEndpoint(t *testing.T, body string) *endpoint {
	t.Helper()
	e := &endpoint{status: http.StatusOK, contentType: "application/json", body: body}
	e.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.requests = append(e.requests, r.Clone(r.Context()))
		if e.contentType != "" {
			w.Header().Set("Content-Type", e.contentType)
		}
		w.WriteHeader(e.status)
		_, _ = io.WriteString(w, e.body)
	}))
	t.Cleanup(e.Close)
	return e
}

// answer changes the response of the endpoint.
func (e *endpoint) answer(status int, contentType, body string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.status, e.contentType, e.body = status, contentType, body
}

// last returns the last request received.
func (e *endpoint) last() *http.Request {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.requests[len(e.requests)-1]
}

// clientFunc is a flam.IConfigRestClient calling a function.
type clientFunc func(*http.Request) (*http.Response, error)

func (f clientFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

// clientCreatorFunc is a flam.IConfigRestClientCreator calling a function.
type clientCreatorFunc func() (flam.IConfigRestClient, error)

func (f clientCreatorFunc) Create() (flam.IConfigRestClient, error) { return f() }

// withClient replaces the client creator of the app with one returning the
// client and error.
func withClient(t *testing.T, app *testutil.App, client flam.IConfigRestClient, err error) {
	t.Helper()
	testutil.NoError(t, app.Decorate(func(flam.IConfigRestClientCreator) flam.IConfigRestClientCreator {
		return clientCreatorFunc(func() (flam.IConfigRestClient, error) { return client, err })
	}))
}

// closeRecorder is a response body recording whether it was closed.
type closeRecorder struct {
	io.Reader
	closed atomic.Bool
}

func (r *closeRecorder) Close() error {
	r.closed.Store(true)
	return nil
}

// docParser is a parser returning its document, whatever the content.
type docParser struct {
	doc flam.IBag
}

func (p *docParser) Id() string                         { return "doc" }
func (p *docParser) Close() error                       { return nil }
func (p *docParser) Parse(io.Reader) (flam.IBag, error) { return p.doc, nil }

// docParserCreator creates a docParser for the "doc" driver.
type docParserCreator struct {
	doc flam.IBag
}

func (c *docParserCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "doc"
}

func (c *docParserCreator) Create(flam.IBag) (flam.IConfigParser, error) {
	return &docParser{doc: c.doc}, nil
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{rest.Driver, "flam.config.sources.driver.rest"},
			{rest.ConfigFieldPriority, "priority"},
			{rest.ConfigFieldReload, "reload"},
			{rest.DefaultReloadConfigPath, "flam.config.defaults.sources.rest.reload"},
			{rest.ConfigFieldURI, "uri"},
			{rest.ConfigFieldMethod, "method"},
			{rest.ConfigFieldHeaders, "headers"},
			{rest.ConfigFieldPath, "path"},
			{rest.ConfigFieldTimestampPath, "timestamp_path"},
			{rest.ConfigFieldTimeout, "timeout"},
			{rest.ConfigFieldMaxSize, "max_size"},
			{rest.DefaultTimeoutConfigPath, "flam.config.defaults.sources.rest.timeout"},
			{rest.DefaultMaxSizeConfigPath, "flam.config.defaults.sources.rest.max_size"},
			{rest.ErrorFieldMaxSize, "max_size"},
			{rest.ConfigFieldParser, "parser"},
			{rest.DefaultPriorityConfigPath, "flam.config.defaults.sources.rest.priority"},
			{rest.DefaultParserConfigPath, "flam.config.defaults.sources.rest.parser"},
			{rest.ErrorFieldID, "id"},
			{rest.ErrorFieldField, "field"},
			{rest.ErrorFieldMethod, "method"},
			{rest.ErrorFieldURI, "uri"},
			{rest.ErrorFieldStatus, "status"},
			{rest.ErrorFieldContentType, "content_type"},
			{rest.ErrorFieldParser, "parser"},
			{rest.ErrorFieldPath, "path"},
			{rest.ErrorFieldTimestampPath, "timestamp_path"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			rest.ErrInvalidField:   "invalid rest config source field",
			rest.ErrClient:         "unable to create rest config source client",
			rest.ErrNilClient:      "nil rest config client",
			rest.ErrParser:         "unable to get rest config source parser",
			rest.ErrMissingFactory: "factory not provided by the container",
			rest.ErrRequest:        "rest config request failed",
			rest.ErrStatus:         "unexpected rest config response status",
			rest.ErrContentType:    "unsupported rest config response content type",
			rest.ErrParse:          "unable to parse rest config response",
			rest.ErrResponsePath:   "no config bag at the rest response path",
			rest.ErrTimestamp:      "no timestamp at the rest response timestamp path",
			rest.ErrTooLarge:       "rest config response too large",
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
		b := rest.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.rest")
		if b == rest.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, rest.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "defaults", bag.Get("flam.config.defaults.sources.rest"), container.Bag{
			"priority": 0, "reload": true, "parser": "", "timeout": 30 * time.Second, "max_size": 10 << 20,
		})
		testutil.Equal(t, "sources", bag.Has(config.SourceConfigPath), false)
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := rest.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a client creator and a creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, rest.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "provider/1"})
	})

	t.Run("should provide a client creator of new http clients with a timeout", func(t *testing.T) {
		app := newApp(t, nil, rest.NewBundle())
		testutil.NoError(t, app.Invoke(func(c flam.IConfigRestClientCreator) {
			first, err := c.Create()
			testutil.NoError(t, err)
			second, err := c.Create()
			testutil.NoError(t, err)
			client, ok := first.(*http.Client)
			if !ok {
				t.Fatalf("Create() = %T, want *http.Client", first)
			}
			testutil.Equal(t, "Timeout", client.Timeout.String(), "30s")
			if first == second {
				t.Fatal("Create() returned the same client twice")
			}
		}))
	})

	t.Run("should not need the configuration bundle to provide the creator", func(t *testing.T) {
		creator(t, newApp(t, nil, rest.NewBundle()))
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := rest.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creator(t, newApp(t, nil, rest.NewBundle()))
	for driver, want := range map[string]bool{
		rest.Driver:                       true,
		"":                                false,
		"rest":                            false,
		"flam.config.sources.driver.file": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
	testutil.Equal(t, "no driver", c.Accept(container.Bag{}), false)
}

func Test_Creator_Create(t *testing.T) {
	t.Run("should request the whole document with a GET by default", func(t *testing.T) {
		e := newEndpoint(t, `{"server": {"port": 8080}}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL + "/config", rest.ConfigFieldPriority: 5,
		})
		testutil.Equal(t, "Id()", s.Id(), "rest")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
		testutil.Equal(t, "method", e.last().Method, http.MethodGet)
		testutil.Equal(t, "path", e.last().URL.Path, "/config")
	})

	t.Run("should send the method and headers", func(t *testing.T) {
		e := newEndpoint(t, `{}`)
		mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI:    e.URL + "/config?env=prod",
			rest.ConfigFieldMethod: http.MethodPost,
			rest.ConfigFieldHeaders: map[string]any{
				"Authorization": "Bearer token", "x-custom": "value",
			},
		})
		req := e.last()
		testutil.Equal(t, "method", req.Method, http.MethodPost)
		testutil.Equal(t, "query", req.URL.RawQuery, "env=prod")
		testutil.Equal(t, "Authorization", req.Header.Get("Authorization"), "Bearer token")
		testutil.Equal(t, "X-Custom", req.Header.Get("X-Custom"), "value")
	})

	t.Run("should take the bag at the path of the response", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"version": 3}, "data": {"config": {"a": 1, "b": {"c": 2}}}}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data.config",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1, "b": container.Bag{"c": 2}}))
	})

	contentTypes := []struct {
		contentType string
		body        string
	}{
		{"application/json", `{"a": 1}`},
		{"application/json; charset=utf-8", `{"a": 1}`},
		{"APPLICATION/JSON", `{"a": 1}`},
		{"application/vnd.config+json", `{"a": 1}`},
		{"application/yaml", "a: 1"},
		{"application/x-yaml", "a: 1"},
		{"text/yaml; charset=utf-8", "a: 1"},
		{"application/vnd.config+yaml", "a: 1"},
	}
	for _, c := range contentTypes {
		t.Run("should name the parser after the content type "+c.contentType, func(t *testing.T) {
			e := newEndpoint(t, "")
			e.answer(http.StatusOK, c.contentType, c.body)
			s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
			testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		})
	}

	t.Run("should use the configured parser whatever the content type", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusOK, "text/plain", "a: 1")
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldParser: yaml.Id,
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusOK, "", "a: 1")
		app := newApp(t, container.Bag{
			rest.DefaultPriorityConfigPath: 3, rest.DefaultParserConfigPath: yaml.Id,
		}, allBundles()...)
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.Equal(t, "Priority()", s.Priority(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should use the built-in defaults when the configuration has none", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		app := newApp(t, nil, allBundles()...)
		testutil.NoError(t, app.Decorate(func(flam.IConfig) flam.IConfig {
			bag := container.Bag{}
			for _, b := range []flam.IBundle{json.NewBundle(), yaml.NewBundle()} {
				testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
			}
			return fakeConfig{bag: bag}
		}))
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.Equal(t, "Priority()", s.Priority(), 0)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should send the requests with the client of the creator", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		var got *http.Request
		withClient(t, app, clientFunc(func(req *http.Request) (*http.Response, error) {
			got = req
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"fake": true}`)),
			}, nil
		}), nil)
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: "https://config.example/app"})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"fake": true}))
		testutil.Equal(t, "uri", got.URL.String(), "https://config.example/app")
	})

	t.Run("should be created through the source factory", func(t *testing.T) {
		e := newEndpoint(t, `{"server": {"port": 8080}}`)
		app := newApp(t, container.Bag{
			config.SourceConfigPath + ".remote": container.Bag{
				factory.ConfigFieldDriver: rest.Driver,
				rest.ConfigFieldURI:       e.URL,
			},
		}, allBundles()...)
		testutil.NoError(t, app.Invoke(func(f flam.IConfigSourceFactory) {
			s, err := f.Get("remote")
			testutil.NoError(t, err)
			testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
		}))
	})
}

func Test_Creator_InvalidFields(t *testing.T) {
	scenarios := []struct {
		name   string
		fields container.Bag
		field  string
	}{
		{"a string priority", container.Bag{rest.ConfigFieldPriority: "1"}, "priority"},
		{"a nil priority", container.Bag{rest.ConfigFieldPriority: nil}, "priority"},
		{"a missing uri", container.Bag{}, "uri"},
		{"an empty uri", container.Bag{rest.ConfigFieldURI: ""}, "uri"},
		{"a uri that is not a string", container.Bag{rest.ConfigFieldURI: 1}, "uri"},
		{"a relative uri", container.Bag{rest.ConfigFieldURI: "/config"}, "uri"},
		{"a uri with no host", container.Bag{rest.ConfigFieldURI: "http:///config"}, "uri"},
		{"a malformed uri", container.Bag{rest.ConfigFieldURI: "http://host:port"}, "uri"},
		{"an empty method", container.Bag{rest.ConfigFieldMethod: ""}, "method"},
		{"a malformed method", container.Bag{rest.ConfigFieldMethod: "GET POST"}, "method"},
		{"a method that is not a string", container.Bag{rest.ConfigFieldMethod: 1}, "method"},
		{"headers that are not a bag", container.Bag{rest.ConfigFieldHeaders: []any{"a"}}, "headers"},
		{"a header that is not a string", container.Bag{rest.ConfigFieldHeaders: container.Bag{"A": 1}}, "headers"},
		{"a header with no name", container.Bag{rest.ConfigFieldHeaders: container.Bag{"": "a"}}, "headers"},
		{"a malformed path", container.Bag{rest.ConfigFieldPath: "a..b"}, "path"},
		{"a path that is not a string", container.Bag{rest.ConfigFieldPath: 1}, "path"},
		{"a malformed timestamp path", container.Bag{rest.ConfigFieldTimestampPath: "a."}, "timestamp_path"},
		{"a timestamp path that is not a string", container.Bag{rest.ConfigFieldTimestampPath: 1}, "timestamp_path"},
		{"a nil timestamp path", container.Bag{rest.ConfigFieldTimestampPath: nil}, "timestamp_path"},
		{"a negative timeout", container.Bag{rest.ConfigFieldTimeout: -time.Second}, "timeout"},
		{"a negative timeout string", container.Bag{rest.ConfigFieldTimeout: "-1s"}, "timeout"},
		{"a malformed timeout string", container.Bag{rest.ConfigFieldTimeout: "soon"}, "timeout"},
		{"an int timeout", container.Bag{rest.ConfigFieldTimeout: 10}, "timeout"},
		{"a nil timeout", container.Bag{rest.ConfigFieldTimeout: nil}, "timeout"},
		{"a zero max size", container.Bag{rest.ConfigFieldMaxSize: 0}, "max_size"},
		{"a negative max size", container.Bag{rest.ConfigFieldMaxSize: -1}, "max_size"},
		{"a string max size", container.Bag{rest.ConfigFieldMaxSize: "1"}, "max_size"},
		{"a nil max size", container.Bag{rest.ConfigFieldMaxSize: nil}, "max_size"},
		{"a parser that is not a string", container.Bag{rest.ConfigFieldParser: 1}, "parser"},
	}
	for _, s := range scenarios {
		t.Run("should refuse "+s.name, func(t *testing.T) {
			fields := container.Bag{rest.ConfigFieldURI: "http://localhost/config"}
			for key, value := range s.fields.All() {
				fields[key] = value
			}
			if s.name == "a missing uri" {
				delete(fields, rest.ConfigFieldURI)
			}
			source, err := create(t, newApp(t, nil, allBundles()...), fields)
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, rest.ErrInvalidField,
				"unable to create rest config source: invalid rest config source field",
				map[string]any{"id": "rest", "field": s.field})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		app := newApp(t, container.Bag{rest.DefaultParserConfigPath: 1}, allBundles()...)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
		testutil.Error(t, err, rest.ErrInvalidField,
			"unable to create rest config source: invalid rest config source field",
			map[string]any{"id": "rest", "field": "parser"})
	})

	t.Run("should accept nil headers as none", func(t *testing.T) {
		e := newEndpoint(t, `{}`)
		mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldHeaders: nil,
		})
	})
}

func Test_Creator_Dependencies(t *testing.T) {
	t.Run("should fail for an unknown configured parser", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: "http://localhost", rest.ConfigFieldParser: "toml",
		})
		testutil.Error(t, err, rest.ErrParser,
			"unable to create rest config source: unable to get rest config source parser: "+
				"unable to generate resource: unknown factory resource id",
			map[string]any{"id": "rest", "parser": "toml"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatal("the error does not wrap the factory error")
		}
	})

	t.Run("should fail without a parser factory", func(t *testing.T) {
		_, err := create(t, newApp(t, nil, rest.NewBundle()), container.Bag{rest.ConfigFieldURI: "http://localhost"})
		testutil.Error(t, err, rest.ErrParser,
			"unable to create rest config source: unable to get rest config source parser: "+
				"factory not provided by the container",
			map[string]any{"id": "rest", "parser": ""})
		if !errors.Is(err, rest.ErrMissingFactory) {
			t.Fatal("the error does not wrap ErrMissingFactory")
		}
	})

	t.Run("should fail when the client cannot be created", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		boom := errors.New("boom")
		withClient(t, app, nil, boom)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
		testutil.Error(t, err, rest.ErrClient,
			"unable to create rest config source: unable to create rest config source client: boom",
			map[string]any{"id": "rest"})
		if !errors.Is(err, boom) {
			t.Fatal("the error does not wrap the creator error")
		}
	})

	t.Run("should fail for a nil client", func(t *testing.T) {
		for name, client := range map[string]flam.IConfigRestClient{
			"nil": nil, "nil pointer": (*http.Client)(nil),
		} {
			app := newApp(t, nil, allBundles()...)
			withClient(t, app, client, nil)
			_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
			if !errors.Is(err, rest.ErrClient) || !errors.Is(err, rest.ErrNilClient) {
				t.Errorf("%s: Create() = %v, want ErrClient wrapping ErrNilClient", name, err)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// responses
// ---------------------------------------------------------------------------

func Test_Responses_Errors(t *testing.T) {
	t.Run("should fail when the request fails", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		boom := errors.New("connection refused")
		withClient(t, app, clientFunc(func(*http.Request) (*http.Response, error) { return nil, boom }), nil)
		source, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost/config"})
		if source != nil {
			t.Fatal("Create() returned a source with an error")
		}
		testutil.Error(t, err, rest.ErrRequest,
			"unable to create rest config source: rest config request failed: connection refused",
			map[string]any{"id": "rest", "method": "GET", "uri": "http://localhost/config"})
		if !errors.Is(err, boom) {
			t.Fatal("the error does not wrap the client error")
		}
	})

	t.Run("should fail when the client returns no response nor error", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		withClient(t, app, clientFunc(func(*http.Request) (*http.Response, error) { return nil, nil }), nil)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost/config"})
		testutil.Error(t, err, rest.ErrNoResponse,
			"unable to create rest config source: rest config request failed: no rest config response",
			map[string]any{"id": "rest", "method": "GET", "uri": "http://localhost/config"})
	})

	t.Run("should send the Host header as the host of the request", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldHeaders: container.Bag{"Host": "config.example"},
		})
		testutil.Equal(t, "Host", e.last().Host, "config.example")
	})

	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError, http.StatusNotModified, http.StatusBadRequest} {
		t.Run(fmt.Sprintf("should fail for the status %d", status), func(t *testing.T) {
			e := newEndpoint(t, `{"a": 1}`)
			e.answer(status, "application/json", `{"a": 1}`)
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
			testutil.Error(t, err, rest.ErrStatus,
				"unable to create rest config source: unexpected rest config response status",
				map[string]any{"id": "rest", "method": "GET", "uri": e.URL, "status": status})
		})
	}

	t.Run("should accept any 2xx status", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusAccepted, "application/json", `{"a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	for _, contentType := range []string{"", "text/plain", "application/xml", "invalid;;", "application/jsonx"} {
		t.Run(fmt.Sprintf("should fail without a parser for the content type %q", contentType), func(t *testing.T) {
			e := newEndpoint(t, "")
			e.answer(http.StatusOK, contentType, `{"a": 1}`)
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
			want := contentType
			if contentType == "" {
				want = "text/plain; charset=utf-8" // sniffed by the server
			}
			testutil.Error(t, err, rest.ErrContentType,
				"unable to create rest config source: unsupported rest config response content type",
				map[string]any{"id": "rest", "uri": e.URL, "content_type": want})
		})
	}

	t.Run("should fail when the parser named by the content type is not available", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusOK, "application/yaml", "a: 1")
		app := newApp(t, nil, config.NewBundle(), json.NewBundle(), rest.NewBundle())
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: e.URL})
		if !errors.Is(err, rest.ErrParser) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Create() = %v, want ErrParser wrapping factory.ErrUnknownID", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "rest", "parser": "yaml"})
	})

	t.Run("should fail for a body the parser refuses", func(t *testing.T) {
		e := newEndpoint(t, `[1]`)
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.Error(t, err, rest.ErrParse,
			"unable to create rest config source: unable to parse rest config response: "+
				"unable to parse json config: json config document is not an object",
			map[string]any{"id": "rest", "uri": e.URL, "parser": "json"})
		if !errors.Is(err, json.ErrNotObject) {
			t.Fatal("the error does not wrap the parser error")
		}
	})

	for _, path := range []string{"missing", "data.missing", "data.value", "data.list", "data.null"} {
		t.Run("should fail for no bag at the path "+path, func(t *testing.T) {
			e := newEndpoint(t, `{"data": {"value": 1, "list": [{"a": 1}], "null": null}}`)
			_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: path,
			})
			testutil.Error(t, err, rest.ErrResponsePath,
				"unable to create rest config source: no config bag at the rest response path",
				map[string]any{"id": "rest", "uri": e.URL, "path": path})
		})
	}

	t.Run("should close the response body, on success and on failure", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		var bodies []*closeRecorder
		var mu sync.Mutex
		status := http.StatusOK
		withClient(t, app, clientFunc(func(*http.Request) (*http.Response, error) {
			mu.Lock()
			defer mu.Unlock()
			body := &closeRecorder{Reader: strings.NewReader(`{"a": 1}`)}
			bodies = append(bodies, body)
			return &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       body,
			}, nil
		}), nil)
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
		status = http.StatusInternalServerError
		_, err := s.Reload()
		if !errors.Is(err, rest.ErrStatus) {
			t.Fatalf("Reload() = %v, want ErrStatus", err)
		}
		for i, body := range bodies {
			testutil.Equal(t, fmt.Sprint("closed ", i), body.closed.Load(), true)
		}
	})
}

func Test_Responses_Documents(t *testing.T) {
	newDocApp := func(t *testing.T, doc flam.IBag) *testutil.App {
		t.Helper()
		app := newApp(t, container.Bag{
			config.ParserConfigPath + ".doc": container.Bag{factory.ConfigFieldDriver: "doc"},
		}, allBundles()...)
		testutil.NoError(t, app.Provide(func() flam.IConfigParserCreator { return &docParserCreator{doc: doc} },
			dig.Group(config.ParserCreatorGroup)))
		return app
	}

	t.Run("should take a map[string]any at the path, as a copy", func(t *testing.T) {
		inner := map[string]any{"a": map[string]any{"b": 1}}
		e := newEndpoint(t, "")
		s := mustCreate(t, newDocApp(t, container.Bag{"data": inner}), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldParser: "doc", rest.ConfigFieldPath: "data",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": map[string]any{"b": 1}}))
		inner["a"].(map[string]any)["b"] = 2
		testutil.Equal(t, "a.b", s.Bag().Int("a.b"), 1)
	})

	t.Run("should take a nil map[string]any at the path as empty", func(t *testing.T) {
		e := newEndpoint(t, "")
		s := mustCreate(t, newDocApp(t, container.Bag{"data": map[string]any(nil)}), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldParser: "doc", rest.ConfigFieldPath: "data",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should take a nil bag at the path as empty", func(t *testing.T) {
		e := newEndpoint(t, "")
		s := mustCreate(t, newDocApp(t, container.Bag{"data": container.Bag(nil)}), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldParser: "doc", rest.ConfigFieldPath: "data",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

func Test_Source(t *testing.T) {
	t.Run("should close nothing, more than once", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		e := newEndpoint(t, `{"a": {"b": 1}}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.NoError(t, s.Bag().Set("a.b", 2))
		testutil.Equal(t, "a.b", s.Bag().Int("a.b"), 1)
	})

	t.Run("should not change with the configuration it was created from", func(t *testing.T) {
		e := newEndpoint(t, `{}`)
		headers := container.Bag{"X-Token": "first"}
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldHeaders: headers,
		})
		headers["X-Token"] = "changed"
		_, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "X-Token", e.last().Header.Get("X-Token"), "first")
	})
}

func Test_Source_Reload(t *testing.T) {
	t.Run("should report no change when the response did not change", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		e.answer(http.StatusOK, "application/json", `{ "a" : 1 }`)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should request the configuration again, with the parser of the new content type", func(t *testing.T) {
		e := newEndpoint(t, `{"data": {"a": 1}}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data",
		})
		e.answer(http.StatusOK, "application/yaml", "data:\n  a: 2\n  b: 3")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 2, "b": 3}))
	})

	t.Run("should keep its values when the request fails", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		e.answer(http.StatusServiceUnavailable, "application/json", `{}`)
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, rest.ErrStatus,
			"unable to reload rest config source: unexpected rest config response status",
			map[string]any{"id": "rest", "method": "GET", "uri": e.URL, "status": http.StatusServiceUnavailable})
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should fail when the parser factory is closed", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		app := newApp(t, nil, allBundles()...)
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: e.URL})
		testutil.NoError(t, app.Invoke(func(f flam.IConfigParserFactory) { testutil.NoError(t, f.Close()) }))
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, rest.ErrParser) || !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Reload() = %v, want ErrParser wrapping factory.ErrClosed", err)
		}
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		e := newEndpoint(t, `{"port": 8080}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				e.answer(http.StatusOK, "application/json", fmt.Sprintf(`{"port": %d}`, []int{8080, 9090}[i%2]))
			})
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
			wg.Go(func() {
				if port := s.Bag().Int("port"); port != 8080 && port != 9090 {
					t.Errorf("port = %d, want 8080 or 9090", port)
				}
			})
		}
		wg.Wait()
	})
}

// restSource creates a source of an endpoint answering {"a": 1}, of
// priority 4, with the extra fields, in an app with the reload default when
// not nil.
func restSource(t *testing.T, reloadDefault any, extra container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	values := container.Bag{}
	if reloadDefault != nil {
		values[rest.DefaultReloadConfigPath] = reloadDefault
	}
	e := newEndpoint(t, `{"a": 1}`)
	fields := container.Bag{rest.ConfigFieldURI: e.URL, rest.ConfigFieldPriority: 4}
	for key, value := range extra.All() {
		fields[key] = value
	}
	return create(t, newApp(t, values, allBundles()...), fields)
}

// ---------------------------------------------------------------------------
// reload activation
// ---------------------------------------------------------------------------

// isReloadable reports whether the source implements
// flam.IConfigReloadableSource.
func isReloadable(s flam.IConfigSource) bool {
	_, ok := s.(flam.IConfigReloadableSource)
	return ok
}

func Test_ReloadActivation(t *testing.T) {
	t.Run("should be reloadable by default", func(t *testing.T) {
		s, err := restSource(t, nil, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should be reloadable when activated", func(t *testing.T) {
		s, err := restSource(t, nil, container.Bag{rest.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should not be reloadable when deactivated, still being a source", func(t *testing.T) {
		s, err := restSource(t, nil, container.Bag{rest.ConfigFieldReload: false})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)
		testutil.Equal(t, "Id()", s.Id(), "rest")
		testutil.Equal(t, "Priority()", s.Priority(), 4)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.NoError(t, s.Bag().Set("x", 1)) // a copy
		testutil.Equal(t, "Bag() after a change of a copy", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should take the default stored in the configuration", func(t *testing.T) {
		s, err := restSource(t, false, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)

		s, err = restSource(t, false, container.Bag{rest.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable with the field", isReloadable(s), true)
	})

	for _, value := range []any{"false", 0, nil} {
		t.Run(fmt.Sprintf("should refuse the reload %#v", value), func(t *testing.T) {
			s, err := restSource(t, nil, container.Bag{rest.ConfigFieldReload: value})
			if s != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, rest.ErrInvalidField,
				"unable to create rest config source: invalid rest config source field",
				map[string]any{"id": "rest", "field": "reload"})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		_, err := restSource(t, "no", nil)
		testutil.Error(t, err, rest.ErrInvalidField,
			"unable to create rest config source: invalid rest config source field",
			map[string]any{"id": "rest", "field": "reload"})
	})
}

// ---------------------------------------------------------------------------
// timestamp
// ---------------------------------------------------------------------------

// stampedSource creates a source of the endpoint taking the configuration
// at "data" and its timestamp at "meta.at".
func stampedSource(t *testing.T, e *endpoint) flam.IConfigReloadableSource {
	t.Helper()
	return mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
		rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data", rest.ConfigFieldTimestampPath: "meta.at",
	})
}

// reload reloads the source, failing the test on an error, and checks the
// change reported and the values.
func reload(t *testing.T, s flam.IConfigReloadableSource, changed bool, values container.Bag) {
	t.Helper()
	got, err := s.Reload()
	testutil.NoError(t, err)
	testutil.Equal(t, "changed", got, changed)
	testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(values))
}

func Test_Source_Timestamp(t *testing.T) {
	t.Run("should keep its values while the timestamp does not change", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": "2026-10-29T10:00:00Z"}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))

		e.answer(http.StatusOK, "application/json", `{"meta": {"at": "2026-10-29T10:00:00Z"}, "data": {"a": 2}}`)
		reload(t, s, false, container.Bag{"a": 1})
		testutil.Equal(t, "requests", len(e.requests), 2)
	})

	t.Run("should not look at the configuration path while the timestamp does not change", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 1}}`)
		reload(t, s, false, container.Bag{"a": 1})
	})

	t.Run("should load the values of a new timestamp", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": "2026-10-29T10:00:00Z"}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": "2026-10-29T11:00:00Z"}, "data": {"a": 2}}`)
		reload(t, s, true, container.Bag{"a": 2})

		e.answer(http.StatusOK, "application/json", `{"meta": {"at": "2026-10-29T11:00:00Z"}, "data": {"a": 3}}`)
		reload(t, s, false, container.Bag{"a": 2})
	})

	t.Run("should report no change for a new timestamp of the same values, and keep it", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 2}, "data": {"a": 1}}`)
		reload(t, s, false, container.Bag{"a": 1})

		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 2}, "data": {"a": 5}}`)
		reload(t, s, false, container.Bag{"a": 1})
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 3}, "data": {"a": 5}}`)
		reload(t, s, true, container.Bag{"a": 5})
	})

	t.Run("should load the values of an older timestamp, as a rollback", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 2}, "data": {"a": 2}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 1}, "data": {"a": 1}}`)
		reload(t, s, true, container.Bag{"a": 1})
	})

	t.Run("should take a timestamp decoded as another type as a change", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": "1"}, "data": {"a": 2}}`)
		reload(t, s, true, container.Bag{"a": 2})
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 1.5}, "data": {"a": 3}}`)
		reload(t, s, true, container.Bag{"a": 3})
	})

	t.Run("should compare the timestamps of responses of different media types", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": "2026-10-29T10:00:00Z"}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/yaml", "meta:\n  at: 2026-10-29T10:00:00Z\ndata:\n  a: 2")
		reload(t, s, false, container.Bag{"a": 1})
	})

	t.Run("should take a timestamp inside the configuration", func(t *testing.T) {
		e := newEndpoint(t, `{"version": "v1", "a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldTimestampPath: "version",
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"version": "v1", "a": 1}))
		e.answer(http.StatusOK, "application/json", `{"version": "v1", "a": 2}`)
		reload(t, s, false, container.Bag{"version": "v1", "a": 1})
		e.answer(http.StatusOK, "application/json", `{"version": "v2", "a": 2}`)
		reload(t, s, true, container.Bag{"version": "v2", "a": 2})
	})

	t.Run("should compare no timestamp without timestamp path", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data", rest.ConfigFieldTimestampPath: "",
		})
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 1}, "data": {"a": 2}}`)
		reload(t, s, true, container.Bag{"a": 2})
	})

	for name, body := range map[string]string{
		"missing":   `{"data": {"a": 1}}`,
		"null":      `{"meta": {"at": null}, "data": {"a": 1}}`,
		"a bool":    `{"meta": {"at": true}, "data": {"a": 1}}`,
		"a bag":     `{"meta": {"at": {"b": 1}}, "data": {"a": 1}}`,
		"a list":    `{"meta": {"at": [1]}, "data": {"a": 1}}`,
		"no object": `{"meta": 1, "data": {"a": 1}}`,
	} {
		t.Run("should fail to create the source for a timestamp "+name, func(t *testing.T) {
			e := newEndpoint(t, body)
			s, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
				rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data", rest.ConfigFieldTimestampPath: "meta.at",
			})
			if s != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, rest.ErrTimestamp,
				"unable to create rest config source: no timestamp at the rest response timestamp path",
				map[string]any{"id": "rest", "uri": e.URL, "timestamp_path": "meta.at"})
		})
	}

	t.Run("should keep its values and timestamp when the timestamp is missing on reload", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"data": {"a": 2}}`)
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, rest.ErrTimestamp,
			"unable to reload rest config source: no timestamp at the rest response timestamp path",
			map[string]any{"id": "rest", "uri": e.URL, "timestamp_path": "meta.at"})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))

		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 1}, "data": {"a": 2}}`)
		reload(t, s, false, container.Bag{"a": 1})
	})

	t.Run("should keep its timestamp when the configuration path is missing on reload", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s := stampedSource(t, e)
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 2}}`)
		_, err := s.Reload()
		if !errors.Is(err, rest.ErrResponsePath) {
			t.Fatalf("Reload() = %v, want ErrResponsePath", err)
		}
		e.answer(http.StatusOK, "application/json", `{"meta": {"at": 2}, "data": {"a": 2}}`)
		reload(t, s, true, container.Bag{"a": 2})
	})

	t.Run("should not change a source whose reload is deactivated", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 1}, "data": {"a": 1}}`)
		s, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldPath: "data",
			rest.ConfigFieldTimestampPath: "meta.at", rest.ConfigFieldReload: false,
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		e := newEndpoint(t, `{"meta": {"at": 0}, "data": {"port": 8080}}`)
		s := stampedSource(t, e)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				e.answer(http.StatusOK, "application/json",
					fmt.Sprintf(`{"meta": {"at": %d}, "data": {"port": %d}}`, i%2, []int{8080, 9090}[i%2]))
			})
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
			wg.Go(func() {
				if port := s.Bag().Int("port"); port != 8080 && port != 9090 {
					t.Errorf("port = %d, want 8080 or 9090", port)
				}
			})
		}
		wg.Wait()
	})
}

// ---------------------------------------------------------------------------
// timeout and maximum size
// ---------------------------------------------------------------------------

// stalledEndpoint returns the URL of a server answering a request once its
// context is done, or after a second.
func stalledEndpoint(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"a": 1}`)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

// noTimeoutClient replaces the client of the app with one with no timeout.
func noTimeoutClient(t *testing.T, app *testutil.App) {
	t.Helper()
	withClient(t, app, &http.Client{}, nil)
}

func Test_Source_Timeout(t *testing.T) {
	t.Run("should bound the requests of a client with no timeout", func(t *testing.T) {
		uri := stalledEndpoint(t)
		for name, timeout := range map[string]any{"a duration": 50 * time.Millisecond, "a string": "50ms"} {
			app := newApp(t, nil, allBundles()...)
			noTimeoutClient(t, app)
			start := time.Now()
			_, err := create(t, app, container.Bag{rest.ConfigFieldURI: uri, rest.ConfigFieldTimeout: timeout})
			if !errors.Is(err, rest.ErrRequest) || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%s: Create() = %v, want ErrRequest wrapping context.DeadlineExceeded", name, err)
			}
			if elapsed := time.Since(start); elapsed > 900*time.Millisecond {
				t.Fatalf("%s: Create() took %v", name, elapsed)
			}
		}
	})

	t.Run("should take the default stored in the configuration", func(t *testing.T) {
		app := newApp(t, container.Bag{rest.DefaultTimeoutConfigPath: "50ms"}, allBundles()...)
		noTimeoutClient(t, app)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: stalledEndpoint(t)})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Create() = %v, want context.DeadlineExceeded", err)
		}
	})

	t.Run("should bound the reloads", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		app := newApp(t, nil, allBundles()...)
		var stall atomic.Bool
		withClient(t, app, clientFunc(func(req *http.Request) (*http.Response, error) {
			if stall.Load() {
				<-req.Context().Done()
				return nil, req.Context().Err()
			}
			return http.DefaultClient.Do(req)
		}), nil)
		s := mustCreate(t, app, container.Bag{rest.ConfigFieldURI: e.URL, rest.ConfigFieldTimeout: "50ms"})
		stall.Store(true)
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Reload() = %v, want context.DeadlineExceeded", err)
		}
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should leave the client as the only bound with a zero timeout", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		withClient(t, app, &http.Client{Timeout: 50 * time.Millisecond}, nil)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: stalledEndpoint(t), rest.ConfigFieldTimeout: "0s"})
		if !errors.Is(err, rest.ErrRequest) {
			t.Fatalf("Create() = %v, want ErrRequest", err)
		}

		var deadline atomic.Bool
		app = newApp(t, nil, allBundles()...)
		withClient(t, app, clientFunc(func(req *http.Request) (*http.Response, error) {
			_, ok := req.Context().Deadline()
			deadline.Store(ok)
			return http.DefaultClient.Do(req)
		}), nil)
		e := newEndpoint(t, `{"a": 1}`)
		mustCreate(t, app, container.Bag{rest.ConfigFieldURI: e.URL, rest.ConfigFieldTimeout: time.Duration(0)})
		testutil.Equal(t, "deadline", deadline.Load(), false)
	})
}

func Test_Source_MaxSize(t *testing.T) {
	t.Run("should accept a body of the maximum size", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldMaxSize: len(`{"a": 1}`),
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should fail for a larger body", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 12}`)
		s, err := create(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldMaxSize: len(`{"a": 1}`),
		})
		if s != nil {
			t.Fatal("Create() returned a source with an error")
		}
		testutil.Error(t, err, rest.ErrTooLarge,
			"unable to create rest config source: rest config response too large",
			map[string]any{"id": "rest", "uri": e.URL, "max_size": 8})
	})

	t.Run("should take the default stored in the configuration, and keep its values on reload", func(t *testing.T) {
		e := newEndpoint(t, `{"a": 1}`)
		s := mustCreate(t, newApp(t, container.Bag{rest.DefaultMaxSizeConfigPath: 10}, allBundles()...),
			container.Bag{rest.ConfigFieldURI: e.URL})
		e.answer(http.StatusOK, "application/json", `{"a": 1, "b": 2}`)
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, rest.ErrTooLarge,
			"unable to reload rest config source: rest config response too large",
			map[string]any{"id": "rest", "uri": e.URL, "max_size": 10})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should fail when the body cannot be read", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		boom := errors.New("boom")
		withClient(t, app, clientFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(io.MultiReader(strings.NewReader("{"), iotest.ErrReader(boom))),
			}, nil
		}), nil)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
		if !errors.Is(err, rest.ErrParse) || !errors.Is(err, boom) {
			t.Fatalf("Create() = %v, want ErrParse wrapping boom", err)
		}
	})

	t.Run("should drain the unread body before closing it", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		body := &closeRecorder{Reader: strings.NewReader("unread")}
		withClient(t, app, clientFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{}, Body: body}, nil
		}), nil)
		_, err := create(t, app, container.Bag{rest.ConfigFieldURI: "http://localhost"})
		if !errors.Is(err, rest.ErrStatus) {
			t.Fatalf("Create() = %v, want ErrStatus", err)
		}
		left, _ := io.ReadAll(body.Reader)
		testutil.Equal(t, "unread", string(left), "")
		testutil.Equal(t, "closed", body.closed.Load(), true)
	})
}

// ---------------------------------------------------------------------------
// value comparison
// ---------------------------------------------------------------------------

func Test_Source_Compare(t *testing.T) {
	scenarios := []struct {
		name     string
		from, to string
		changed  bool
	}{
		{"an equal nested document", `{"a": {"b": [1, {"c": 0.5}]}}`, `{"a": {"b": [1, {"c": 0.5}]}}`, false},
		{"a changed float", `{"a": 0.5}`, `{"a": 1.5}`, true},
		{"a float becoming a string", `{"a": 0.5}`, `{"a": "0.5"}`, true},
		{"a bag becoming a string", `{"a": {"b": 1}}`, `{"a": "b"}`, true},
		{"a bag with another key", `{"a": {"b": 1}}`, `{"a": {"c": 1}}`, true},
		{"a bag with another size", `{"a": {"b": 1}}`, `{"a": {"b": 1, "c": 1}}`, true},
		{"a bag with another value", `{"a": {"b": 1}}`, `{"a": {"b": 2}}`, true},
		{"a list becoming a string", `{"a": [1]}`, `{"a": "1"}`, true},
		{"a list with another size", `{"a": [1]}`, `{"a": [1, 2]}`, true},
		{"a list with another item", `{"a": [1]}`, `{"a": [2]}`, true},
		{"an empty document", `{}`, `null`, false},
	}
	for _, s := range scenarios {
		t.Run("should compare "+s.name, func(t *testing.T) {
			e := newEndpoint(t, s.from)
			source := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
			e.answer(http.StatusOK, "application/json", s.to)
			changed, err := source.Reload()
			testutil.NoError(t, err)
			testutil.Equal(t, "changed", changed, s.changed)
		})
	}

	t.Run("should take NaN values as unchanged", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusOK, "application/yaml", "a: .nan\nb: [.nan]\nc: {d: .nan}")
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{rest.ConfigFieldURI: e.URL})
		for range 3 {
			changed, err := s.Reload()
			testutil.NoError(t, err)
			testutil.Equal(t, "changed", changed, false)
		}
		e.answer(http.StatusOK, "application/yaml", "a: .nan\nb: [1.5]\nc: {d: .nan}")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
	})

	t.Run("should take a NaN timestamp as unchanged", func(t *testing.T) {
		e := newEndpoint(t, "")
		e.answer(http.StatusOK, "application/yaml", "at: .nan\na: 1")
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{
			rest.ConfigFieldURI: e.URL, rest.ConfigFieldTimestampPath: "at",
		})
		e.answer(http.StatusOK, "application/yaml", "at: .nan\na: 2")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})
}
