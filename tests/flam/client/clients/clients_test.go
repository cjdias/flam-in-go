package clients

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	breakerstandard "github.com/cjdias/flam-in-go/pkg/flam/client/breakers/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/client/clients"
	"github.com/cjdias/flam-in-go/pkg/flam/client/clients/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// env is a client created by the factory of an app, with the clock of
// the app and the circuit breaker factory.
type env struct {
	client   flam.IClient
	clock    *testutil.Clock
	breakers flam.IClientCircuitBreakerFactory
}

// breaker returns the circuit breaker of the id.
func (e *env) breaker(t *testing.T, id string) flam.IClientCircuitBreaker {
	t.Helper()
	b, err := e.breakers.Get(id)
	testutil.NoError(t, err)
	return b
}

// create returns the client "main" of the fields, created by the factory
// of an app with the client bundle, the standard driver bundles and their
// defaults, the "cb" breaker, opening after 2 failures for 10s and
// closing after 1 trial, the values and a fake clock, after calling the
// setups, such as to provide a tracer provider.
func create(t *testing.T, fields container.Bag, values map[string]any, setups ...func(app *testutil.App)) (*env, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), client.NewBundle(), breakerstandard.NewBundle(), standard.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = standard.Driver
	}
	testutil.NoError(t, bag.Set(client.ClientConfigPath+".main", fields))
	testutil.NoError(t, bag.Set(client.BreakerConfigPath+".cb", container.Bag{
		factory.ConfigFieldDriver:                   breakerstandard.Driver,
		breakerstandard.ConfigFieldFailureThreshold: 2,
		breakerstandard.ConfigFieldOpenTimeout:      "10s",
		breakerstandard.ConfigFieldHalfOpenRequests: 1,
	}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	e := &env{clock: testutil.NewClock(time.Unix(0, 0))}
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return e.clock }))
	for _, setup := range setups {
		setup(app)
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IClientFactory, breakers flam.IClientCircuitBreakerFactory) {
		e.breakers = breakers
		e.client, err = f.Get("main")
	}))
	if err == nil {
		t.Cleanup(func() { _ = e.client.Close() })
	}
	return e, err
}

// newEnv returns the env of the client of the fields, failing the test
// when it cannot be created.
func newEnv(t *testing.T, fields container.Bag) *env {
	t.Helper()
	e, err := create(t, fields, nil)
	testutil.NoError(t, err)
	return e
}

// hit is a request received by a server.
type hit struct {
	method string
	path   string
	header http.Header
	body   string
	at     time.Time
}

// server is a test server recording its hits and answering each with the
// status of the statuses, in order, the last one repeating, after
// calling the handle hook, when set.
type server struct {
	*httptest.Server

	clock    *testutil.Clock
	statuses []int
	handle   func(w http.ResponseWriter, req *http.Request, n int) bool

	mu   sync.Mutex
	hits []hit
}

// newServer returns a started server answering with the statuses, read
// the time of its hits from the clock, when not nil.
func newServer(t *testing.T, clock *testutil.Clock, statuses ...int) *server {
	t.Helper()
	s := &server{clock: clock, statuses: statuses}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.Close)
	return s
}

func (s *server) serve(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	h := hit{method: req.Method, path: req.URL.Path, header: req.Header.Clone(), body: string(body)}
	if s.clock != nil {
		h.at = s.clock.Now()
	}
	s.mu.Lock()
	s.hits = append(s.hits, h)
	n := len(s.hits)
	s.mu.Unlock()

	if s.handle != nil && s.handle(w, req, n) {
		return
	}
	status := http.StatusOK
	if len(s.statuses) > 0 {
		status = s.statuses[min(n, len(s.statuses))-1]
	}
	w.WriteHeader(status)
	_, _ = io.WriteString(w, req.Method+" "+req.URL.Path)
}

// recorded returns the hits of the server.
func (s *server) recorded() []hit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]hit(nil), s.hits...)
}

// read returns the body of the response, closing it.
func read(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	testutil.NoError(t, err)
	return string(body)
}

// trackedBody is a request body recording its close.
type trackedBody struct {
	io.Reader

	closed atomic.Bool
}

func (b *trackedBody) Close() error {
	b.closed.Store(true)
	return nil
}

// result is the outcome of a Do.
type result struct {
	resp *http.Response
	err  error
}

// doAdvancing runs the Do, advancing the clock by steps of a millisecond
// while the client waits for a retry, until Do returns.
func doAdvancing(t *testing.T, e *env, do func() (*http.Response, error)) (*http.Response, error) {
	t.Helper()
	done := make(chan result, 1)
	go func() {
		resp, err := do()
		done <- result{resp, err}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case r := <-done:
			return r.resp, r.err
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("Do() did not return")
		}
		if e.clock.Timers() > 0 {
			e.clock.Advance(time.Millisecond)
		} else {
			time.Sleep(50 * time.Microsecond)
		}
	}
}

// waitTimer waits until the client waits for a retry.
func waitTimer(t *testing.T, clock *testutil.Clock) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); clock.Timers() == 0; time.Sleep(50 * time.Microsecond) {
		if time.Now().After(deadline) {
			t.Fatal("the client did not wait")
		}
	}
}

// innermost returns the innermost *flam.Error of the error.
func innermost(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %v, want a *flam.Error", err)
	}
	for errors.As(e.Unwrap(), &e) {
	}
	return e
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                             {standard.Id, "standard"},
		"Driver":                         {standard.Driver, "flam.client.clients.driver.standard"},
		"ConfigFieldBaseURL":             {standard.ConfigFieldBaseURL, "base_url"},
		"ConfigFieldTimeout":             {standard.ConfigFieldTimeout, "timeout"},
		"ConfigFieldHeaders":             {standard.ConfigFieldHeaders, "headers"},
		"ConfigFieldBreaker":             {standard.ConfigFieldBreaker, "breaker"},
		"ConfigFieldRetries":             {standard.ConfigFieldRetries, "retries"},
		"ConfigFieldRetryDelay":          {standard.ConfigFieldRetryDelay, "retry_delay"},
		"ConfigFieldRetryMaxDelay":       {standard.ConfigFieldRetryMaxDelay, "retry_max_delay"},
		"ConfigFieldTracing":             {standard.ConfigFieldTracing, "tracing"},
		"ConfigFieldMetrics":             {standard.ConfigFieldMetrics, "metrics"},
		"ConfigFieldNamespace":           {standard.ConfigFieldNamespace, "namespace"},
		"DefaultTimeoutConfigPath":       {standard.DefaultTimeoutConfigPath, "flam.client.defaults.clients.standard.timeout"},
		"DefaultHeadersConfigPath":       {standard.DefaultHeadersConfigPath, "flam.client.defaults.clients.standard.headers"},
		"DefaultRetriesConfigPath":       {standard.DefaultRetriesConfigPath, "flam.client.defaults.clients.standard.retries"},
		"DefaultRetryDelayConfigPath":    {standard.DefaultRetryDelayConfigPath, "flam.client.defaults.clients.standard.retry_delay"},
		"DefaultRetryMaxDelayConfigPath": {standard.DefaultRetryMaxDelayConfigPath, "flam.client.defaults.clients.standard.retry_max_delay"},
		"DefaultTracingConfigPath":       {standard.DefaultTracingConfigPath, "flam.client.defaults.clients.standard.tracing"},
		"DefaultMetricsConfigPath":       {standard.DefaultMetricsConfigPath, "flam.client.defaults.clients.standard.metrics"},
		"DefaultNamespaceConfigPath":     {standard.DefaultNamespaceConfigPath, "flam.client.defaults.clients.standard.namespace"},
		"ScopeName":                      {standard.ScopeName, "github.com/cjdias/flam-in-go/pkg/flam/client/clients/standard"},
		"MetricRequests":                 {standard.MetricRequests, "requests_total"},
		"MetricDuration":                 {standard.MetricDuration, "request_duration_seconds"},
		"Subsystem":                      {standard.Subsystem, "client"},
		"LabelClient":                    {standard.LabelClient, "client"},
		"LabelMethod":                    {standard.LabelMethod, "method"},
		"LabelCode":                      {standard.LabelCode, "code"},
		"MethodOther":                    {standard.MethodOther, "OTHER"},
		"ErrorTypeOpen":                  {standard.ErrorTypeOpen, "circuit_open"},
		"ErrorTypeTimeout":               {standard.ErrorTypeTimeout, "timeout"},
		"ErrorTypeCanceled":              {standard.ErrorTypeCanceled, "canceled"},
		"ErrorFieldID":                   {standard.ErrorFieldID, "id"},
		"ErrorFieldField":                {standard.ErrorFieldField, "field"},
		"ErrorFieldBreaker":              {standard.ErrorFieldBreaker, "breaker"},
		"ErrorFieldMethod":               {standard.ErrorFieldMethod, "method"},
		"ErrorFieldURL":                  {standard.ErrorFieldURL, "url"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrInvalidField":   standard.ErrInvalidField,
		"ErrBreaker":        standard.ErrBreaker,
		"ErrInvalidRequest": standard.ErrInvalidRequest,
		"ErrOpen":           standard.ErrOpen,
		"ErrRequest":        standard.ErrRequest,
		"ErrRegister":       standard.ErrRegister,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := clients.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.client.clients")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.client.clients", "bundle:flam.client.clients.standard", "provider/1",
		})
	})

	t.Run("should configure the standard client and the defaults", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.client.clients.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			client.ClientConfigPath + "." + standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
			standard.DefaultTimeoutConfigPath:           "30s",
			standard.DefaultRetriesConfigPath:           0,
			standard.DefaultRetryDelayConfigPath:        "100ms",
			standard.DefaultRetryMaxDelayConfigPath:     "2s",
			standard.DefaultTracingConfigPath:           true,
			standard.DefaultMetricsConfigPath:           true,
			standard.DefaultNamespaceConfigPath:         "flam",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "headers", bag.Has(standard.DefaultHeadersConfigPath), false)
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should create the default client", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
		for _, b := range []flam.IBundle{config.NewBundle(), client.NewBundle(), clients.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		bag := container.Bag{}
		testutil.NoError(t, standard.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		s := newServer(t, nil)
		testutil.NoError(t, app.Invoke(func(f flam.IClientFactory) {
			c, err := f.Get(standard.Id)
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", c.Id(), standard.Id)
			resp, err := c.Request(context.Background(), http.MethodGet, s.URL+"/ping", nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "body", read(t, resp), "GET /ping")
			testutil.NoError(t, f.Close())
		}))
	})

	t.Run("should use the built-in defaults without the default paths", func(t *testing.T) {
		app := testutil.NewApp()
		clock := testutil.NewClock(time.Unix(0, 0))
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
		for _, b := range []flam.IBundle{config.NewBundle(), client.NewBundle(), clients.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(client.ClientConfigPath+".main", container.Bag{factory.ConfigFieldDriver: standard.Driver}))
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		s := newServer(t, nil, http.StatusBadGateway)
		testutil.NoError(t, app.Invoke(func(f flam.IClientFactory) {
			c, err := f.Get("main")
			testutil.NoError(t, err)
			defer func() { _ = c.Close() }()
			// no retry by default
			resp, err := c.Request(context.Background(), http.MethodGet, s.URL, nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "status", resp.StatusCode, http.StatusBadGateway)
			_ = read(t, resp)
			testutil.Equal(t, "hits", len(s.recorded()), 1)
		}))
	})

	t.Run("should not create a client of another driver", func(t *testing.T) {
		_, err := create(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want ErrNoCreator", err)
		}
	})

	t.Run("should read the default headers and breaker", func(t *testing.T) {
		e, err := create(t, container.Bag{}, map[string]any{
			standard.DefaultHeadersConfigPath: map[string]string{"x-default": "on"},
			client.DefaultBreakerConfigPath:   "cb",
		})
		testutil.NoError(t, err)
		s := newServer(t, nil, http.StatusInternalServerError)
		for range 2 {
			resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
			testutil.NoError(t, err)
			_ = read(t, resp)
		}
		testutil.Equal(t, "header", s.recorded()[0].header.Get("X-Default"), "on")
		testutil.Equal(t, "State()", e.breaker(t, "cb").State(), flam.ClientCircuitOpen)
	})

	t.Run("should read the headers of a bag", func(t *testing.T) {
		e := newEnv(t, container.Bag{standard.ConfigFieldHeaders: container.Bag{"accept": "application/json", "x-token": "t"}})
		s := newServer(t, nil)
		resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		header := s.recorded()[0].header
		testutil.Equal(t, "accept", header.Values("Accept"), []string{"application/json"})
		testutil.Equal(t, "token", header.Values("X-Token"), []string{"t"})
	})

	t.Run("should read the headers of a map", func(t *testing.T) {
		e := newEnv(t, container.Bag{standard.ConfigFieldHeaders: map[string]any{"x-map": "m"}})
		s := newServer(t, nil)
		resp, err := e.client.Request(context.Background(), http.MethodGet, s.URL, nil)
		testutil.NoError(t, err)
		_ = read(t, resp)
		testutil.Equal(t, "header", s.recorded()[0].header.Get("X-Map"), "m")
	})

	t.Run("should fail to create a client with an unknown breaker", func(t *testing.T) {
		_, err := create(t, container.Bag{standard.ConfigFieldBreaker: "unknown"}, nil)
		if !errors.Is(err, standard.ErrBreaker) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrBreaker and ErrUnknownID", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		for e.Context()[standard.ErrorFieldBreaker] == nil && errors.As(e.Unwrap(), &e) {
		}
		testutil.Equal(t, "context", e.Context(), map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldBreaker: "unknown"})
	})

	for name, tc := range map[string]struct {
		fields container.Bag
		values map[string]any
		field  string
	}{
		"a base URL of another type":        {container.Bag{standard.ConfigFieldBaseURL: 1}, nil, standard.ConfigFieldBaseURL},
		"a relative base URL":               {container.Bag{standard.ConfigFieldBaseURL: "/api"}, nil, standard.ConfigFieldBaseURL},
		"a base URL without host":           {container.Bag{standard.ConfigFieldBaseURL: "file:///api"}, nil, standard.ConfigFieldBaseURL},
		"an unparsable base URL":            {container.Bag{standard.ConfigFieldBaseURL: "http://[::1"}, nil, standard.ConfigFieldBaseURL},
		"an invalid timeout":                {container.Bag{standard.ConfigFieldTimeout: "soon"}, nil, standard.ConfigFieldTimeout},
		"an invalid default timeout":        {container.Bag{}, map[string]any{standard.DefaultTimeoutConfigPath: -time.Second}, standard.ConfigFieldTimeout},
		"headers of another type":           {container.Bag{standard.ConfigFieldHeaders: "accept"}, nil, standard.ConfigFieldHeaders},
		"a header of another type":          {container.Bag{standard.ConfigFieldHeaders: container.Bag{"x-n": 1}}, nil, standard.ConfigFieldHeaders},
		"invalid default headers":           {container.Bag{}, map[string]any{standard.DefaultHeadersConfigPath: []string{"a"}}, standard.ConfigFieldHeaders},
		"a breaker of another type":         {container.Bag{standard.ConfigFieldBreaker: 1}, nil, standard.ConfigFieldBreaker},
		"retries of another type":           {container.Bag{standard.ConfigFieldRetries: "1"}, nil, standard.ConfigFieldRetries},
		"negative retries":                  {container.Bag{standard.ConfigFieldRetries: -1}, nil, standard.ConfigFieldRetries},
		"an invalid retry delay":            {container.Bag{standard.ConfigFieldRetryDelay: "-1s"}, nil, standard.ConfigFieldRetryDelay},
		"an invalid retry max delay":        {container.Bag{standard.ConfigFieldRetryMaxDelay: 1}, nil, standard.ConfigFieldRetryMaxDelay},
		"a retry max delay below the delay": {container.Bag{standard.ConfigFieldRetryDelay: "3s"}, nil, standard.ConfigFieldRetryMaxDelay},
		"a tracing of another type":         {container.Bag{standard.ConfigFieldTracing: "yes"}, nil, standard.ConfigFieldTracing},
		"a metrics of another type":         {container.Bag{standard.ConfigFieldMetrics: 1}, nil, standard.ConfigFieldMetrics},
		"a namespace of another type":       {container.Bag{standard.ConfigFieldNamespace: 1}, nil, standard.ConfigFieldNamespace},
	} {
		t.Run("should fail to create a client with "+name, func(t *testing.T) {
			_, err := create(t, tc.fields, tc.values)
			testutil.Error(t, innermost(t, err), standard.ErrInvalidField, "unable to create standard client: invalid standard client field",
				map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldField: tc.field})
		})
	}
}

func Test_Client(t *testing.T) {
	t.Run("should resolve the URL against the base URL and add the default headers", func(t *testing.T) {
		s := newServer(t, nil)
		e := newEnv(t, container.Bag{
			standard.ConfigFieldBaseURL: s.URL + "/v1/",
			standard.ConfigFieldHeaders: container.Bag{"x-default": "default", "x-kept": "default"},
		})

		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "users/7?full=1", nil)
		testutil.NoError(t, err)
		req.Header.Set("X-Kept", "request")
		resp, err := e.client.Do(req)
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusOK)
		testutil.Equal(t, "body", read(t, resp), "GET /v1/users/7")

		resp, err = e.client.Request(context.Background(), http.MethodDelete, "/users/7", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "rooted body", read(t, resp), "DELETE /users/7")

		other := newServer(t, nil)
		resp, err = e.client.Request(context.Background(), http.MethodGet, other.URL+"/abs", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "absolute body", read(t, resp), "GET /abs")

		hits := s.recorded()
		testutil.Equal(t, "hits", len(hits), 2)
		testutil.Equal(t, "default", hits[0].header.Values("X-Default"), []string{"default"})
		testutil.Equal(t, "kept", hits[0].header.Values("X-Kept"), []string{"request"})
		testutil.Equal(t, "absolute default", other.recorded()[0].header.Get("X-Default"), "default")

		// the request is not changed
		testutil.Equal(t, "request URL", req.URL.String(), "users/7?full=1")
		testutil.Equal(t, "request header", req.Header.Get("X-Default"), "")
	})

	t.Run("should send a request without header", func(t *testing.T) {
		s := newServer(t, nil)
		e := newEnv(t, container.Bag{standard.ConfigFieldHeaders: container.Bag{"x-default": "on"}})
		req, err := http.NewRequestWithContext(context.Background(), "", s.URL+"/bare", nil)
		testutil.NoError(t, err)
		req.Header = nil
		resp, err := e.client.Do(req)
		testutil.NoError(t, err)
		testutil.Equal(t, "body", read(t, resp), "GET /bare")
		testutil.Equal(t, "header", s.recorded()[0].header.Get("X-Default"), "on")
	})

	t.Run("should fail to do a nil request", func(t *testing.T) {
		e := newEnv(t, container.Bag{})
		resp, err := e.client.Do(nil)
		testutil.Equal(t, "response", resp, (*http.Response)(nil))
		testutil.Error(t, err, standard.ErrInvalidRequest, "unable to do standard client request: invalid standard client request",
			map[string]any{standard.ErrorFieldID: "main"})
		_, err = e.client.Do(&http.Request{})
		testutil.Error(t, err, standard.ErrInvalidRequest, "unable to do standard client request: invalid standard client request",
			map[string]any{standard.ErrorFieldID: "main"})
	})

	t.Run("should fail to do a relative request without base URL", func(t *testing.T) {
		e := newEnv(t, container.Bag{})
		body := &trackedBody{Reader: strings.NewReader("x")}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/users", body)
		testutil.NoError(t, err)
		resp, err := e.client.Do(req)
		testutil.Equal(t, "response", resp, (*http.Response)(nil))
		testutil.Error(t, err, standard.ErrInvalidRequest, "unable to do standard client request: invalid standard client request",
			map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "POST", standard.ErrorFieldURL: "/users"})
		testutil.Equal(t, "body closed", body.closed.Load(), true)

		_, err = e.client.Do(&http.Request{URL: req.URL})
		testutil.Error(t, err, standard.ErrInvalidRequest, "unable to do standard client request: invalid standard client request",
			map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "GET", standard.ErrorFieldURL: "/users"})
	})

	t.Run("should fail to create an invalid request", func(t *testing.T) {
		e := newEnv(t, container.Bag{})
		_, err := e.client.Request(context.Background(), "bad method", "http://localhost", nil)
		var fe *flam.Error
		if !errors.Is(err, standard.ErrInvalidRequest) || !errors.As(err, &fe) {
			t.Fatalf("Request() = %v, want ErrInvalidRequest", err)
		}
		testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "bad method", standard.ErrorFieldURL: "http://localhost"})

		var nilCtx context.Context
		_, err = e.client.Request(nilCtx, http.MethodGet, "http://localhost", nil)
		if !errors.Is(err, standard.ErrInvalidRequest) {
			t.Fatalf("Request() = %v, want ErrInvalidRequest", err)
		}
	})

	t.Run("should fail to reach a closed server", func(t *testing.T) {
		s := newServer(t, nil)
		s.Close()
		e := newEnv(t, container.Bag{standard.ConfigFieldBreaker: "cb", standard.ConfigFieldBaseURL: s.URL})
		_, err := e.client.Request(context.Background(), http.MethodGet, "/down", nil)
		var fe *flam.Error
		if !errors.Is(err, standard.ErrRequest) || !errors.As(err, &fe) {
			t.Fatalf("Request() = %v, want ErrRequest", err)
		}
		testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "GET", standard.ErrorFieldURL: s.URL + "/down"})
		_, _ = e.client.Request(context.Background(), http.MethodGet, "/down", nil)
		testutil.Equal(t, "State()", e.breaker(t, "cb").State(), flam.ClientCircuitOpen)
	})

	t.Run("should fail fast while the circuit is open", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError, http.StatusServiceUnavailable, http.StatusOK)
		e := newEnv(t, container.Bag{standard.ConfigFieldBreaker: "cb", standard.ConfigFieldBaseURL: s.URL})
		for _, status := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable} {
			resp, err := e.client.Request(context.Background(), http.MethodGet, "/x", nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "status", resp.StatusCode, status)
			_ = read(t, resp)
		}
		b := e.breaker(t, "cb")
		testutil.Equal(t, "State()", b.State(), flam.ClientCircuitOpen)

		body := &trackedBody{Reader: strings.NewReader("x")}
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/x", body)
		testutil.NoError(t, err)
		resp, err := e.client.Do(req)
		testutil.Equal(t, "response", resp, (*http.Response)(nil))
		testutil.Error(t, err, standard.ErrOpen, "unable to do standard client request: standard client circuit open",
			map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "POST", standard.ErrorFieldURL: s.URL + "/x", standard.ErrorFieldBreaker: "cb"})
		testutil.Equal(t, "body closed", body.closed.Load(), true)
		testutil.Equal(t, "hits", len(s.recorded()), 2)

		// the trial request closes the circuit
		e.clock.Advance(10 * time.Second)
		resp, err = e.client.Request(context.Background(), http.MethodGet, "/x", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "trial status", resp.StatusCode, http.StatusOK)
		_ = read(t, resp)
		testutil.Equal(t, "State() after trial", b.State(), flam.ClientCircuitClosed)
	})

	t.Run("should not count the 4xx responses as failures", func(t *testing.T) {
		s := newServer(t, nil, http.StatusNotFound)
		e := newEnv(t, container.Bag{standard.ConfigFieldBreaker: "cb", standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 2})
		for range 3 {
			resp, err := e.client.Request(context.Background(), http.MethodGet, "/x", nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "status", resp.StatusCode, http.StatusNotFound)
			_ = read(t, resp)
		}
		testutil.Equal(t, "hits", len(s.recorded()), 3)
		testutil.Equal(t, "State()", e.breaker(t, "cb").State(), flam.ClientCircuitClosed)
	})

	t.Run("should not count the requests cancelled by their caller", func(t *testing.T) {
		release := make(chan struct{})
		s := newServer(t, nil)
		s.handle = func(w http.ResponseWriter, req *http.Request, _ int) bool {
			<-req.Context().Done()
			<-release
			return true
		}
		defer close(release)
		e := newEnv(t, container.Bag{standard.ConfigFieldBreaker: "cb", standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 3})
		for i := range 3 {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				for len(s.recorded()) == i {
					time.Sleep(50 * time.Microsecond)
				}
				cancel()
			}()
			_, err := e.client.Request(ctx, http.MethodGet, "/slow", nil)
			cancel()
			if !errors.Is(err, standard.ErrRequest) || !errors.Is(err, context.Canceled) {
				t.Fatalf("Request() = %v, want ErrRequest and context.Canceled", err)
			}
		}
		testutil.Equal(t, "hits", len(s.recorded()), 3)
		testutil.Equal(t, "State()", e.breaker(t, "cb").State(), flam.ClientCircuitClosed)
	})

	t.Run("should fail a request exceeding the timeout", func(t *testing.T) {
		release := make(chan struct{})
		s := newServer(t, nil)
		s.handle = func(http.ResponseWriter, *http.Request, int) bool {
			<-release
			return false
		}
		defer close(release)
		e := newEnv(t, container.Bag{standard.ConfigFieldBreaker: "cb", standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldTimeout: "20ms"})
		for range 2 {
			_, err := e.client.Request(context.Background(), http.MethodGet, "/slow", nil)
			if !errors.Is(err, standard.ErrRequest) {
				t.Fatalf("Request() = %v, want ErrRequest", err)
			}
		}
		testutil.Equal(t, "State()", e.breaker(t, "cb").State(), flam.ClientCircuitOpen)
	})

	t.Run("should retry the failed attempts after a growing delay", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusOK)
		e := newEnv(t, container.Bag{
			standard.ConfigFieldBaseURL:       s.URL,
			standard.ConfigFieldRetries:       3,
			standard.ConfigFieldRetryDelay:    "100ms",
			standard.ConfigFieldRetryMaxDelay: "300ms",
		})
		s.clock = e.clock
		resp, err := doAdvancing(t, e, func() (*http.Response, error) {
			return e.client.Request(context.Background(), http.MethodGet, "/retry", nil)
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusOK)
		testutil.Equal(t, "body", read(t, resp), "GET /retry")

		var at []time.Duration
		for _, h := range s.recorded() {
			at = append(at, h.at.Sub(time.Unix(0, 0)))
		}
		testutil.Equal(t, "attempts", at, []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond, 600 * time.Millisecond})
		testutil.Equal(t, "timers", e.clock.Timers(), 0)
	})

	t.Run("should keep doubling the delay below the maximum", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError)
		e := newEnv(t, container.Bag{
			standard.ConfigFieldBaseURL:       s.URL,
			standard.ConfigFieldRetries:       3,
			standard.ConfigFieldRetryDelay:    "10ms",
			standard.ConfigFieldRetryMaxDelay: "1s",
		})
		s.clock = e.clock
		resp, err := doAdvancing(t, e, func() (*http.Response, error) {
			return e.client.Request(context.Background(), http.MethodHead, "/retry", nil)
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusInternalServerError)
		_ = read(t, resp)

		var at []time.Duration
		for _, h := range s.recorded() {
			at = append(at, h.at.Sub(time.Unix(0, 0)))
		}
		testutil.Equal(t, "attempts", at, []time.Duration{0, 10 * time.Millisecond, 30 * time.Millisecond, 70 * time.Millisecond})
	})

	t.Run("should retry without delay", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError, http.StatusOK)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "0s"})
		resp, err := e.client.Request(context.Background(), http.MethodOptions, "/now", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusOK)
		_ = read(t, resp)
		testutil.Equal(t, "hits", len(s.recorded()), 2)
	})

	t.Run("should retry a transport failure", func(t *testing.T) {
		s := newServer(t, nil)
		s.handle = func(w http.ResponseWriter, _ *http.Request, n int) bool {
			if n > 1 {
				return false
			}
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return true
		}
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "0s"})
		resp, err := e.client.Request(context.Background(), http.MethodGet, "/flaky", nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "body", read(t, resp), "GET /flaky")
		testutil.Equal(t, "hits", len(s.recorded()), 2)
	})

	t.Run("should send the body again on a retry", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError, http.StatusOK)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "0s"})
		resp, err := e.client.Request(context.Background(), http.MethodPut, "/doc", strings.NewReader("payload"))
		testutil.NoError(t, err)
		testutil.Equal(t, "status", resp.StatusCode, http.StatusOK)
		_ = read(t, resp)
		hits := s.recorded()
		testutil.Equal(t, "hits", len(hits), 2)
		testutil.Equal(t, "first body", hits[0].body, "payload")
		testutil.Equal(t, "second body", hits[1].body, "payload")
	})

	t.Run("should not retry a request that cannot be sent again", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 2, standard.ConfigFieldRetryDelay: "0s"})
		for name, do := range map[string]func() (*http.Response, error){
			"a POST": func() (*http.Response, error) {
				return e.client.Request(context.Background(), http.MethodPost, "/x", strings.NewReader("a"))
			},
			"a body without GetBody": func() (*http.Response, error) {
				return e.client.Request(context.Background(), http.MethodPut, "/x", io.NopCloser(strings.NewReader("b")))
			},
		} {
			before := len(s.recorded())
			resp, err := do()
			testutil.NoError(t, err)
			testutil.Equal(t, name+" status", resp.StatusCode, http.StatusInternalServerError)
			_ = read(t, resp)
			testutil.Equal(t, name+" hits", len(s.recorded())-before, 1)
		}
	})

	t.Run("should fail when the body cannot be sent again", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 2, standard.ConfigFieldRetryDelay: "0s"})
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "/x", strings.NewReader("a"))
		testutil.NoError(t, err)
		req.GetBody = func() (io.ReadCloser, error) { return nil, errBoom }
		_, err = e.client.Do(req)
		if !errors.Is(err, standard.ErrRequest) || !errors.Is(err, errBoom) {
			t.Fatalf("Do() = %v, want ErrRequest and errBoom", err)
		}
		testutil.Equal(t, "hits", len(s.recorded()), 1)
	})

	t.Run("should stop retrying once the circuit opens", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError)
		e := newEnv(t, container.Bag{
			standard.ConfigFieldBaseURL:    s.URL,
			standard.ConfigFieldBreaker:    "cb",
			standard.ConfigFieldRetries:    5,
			standard.ConfigFieldRetryDelay: "0s",
		})
		_, err := e.client.Request(context.Background(), http.MethodGet, "/x", nil)
		if !errors.Is(err, standard.ErrOpen) {
			t.Fatalf("Request() = %v, want ErrOpen", err)
		}
		testutil.Equal(t, "hits", len(s.recorded()), 2)
	})

	t.Run("should stop waiting for a retry when the context is done", func(t *testing.T) {
		s := newServer(t, nil, http.StatusInternalServerError)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldRetries: 1, standard.ConfigFieldRetryDelay: "1s"})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			_, err := e.client.Request(ctx, http.MethodGet, "/x", nil)
			done <- err
		}()
		waitTimer(t, e.clock)
		cancel()
		err := <-done
		var fe *flam.Error
		if !errors.Is(err, standard.ErrRequest) || !errors.Is(err, context.Canceled) || !errors.As(err, &fe) {
			t.Fatalf("Request() = %v, want ErrRequest and context.Canceled", err)
		}
		testutil.Context(t, fe, map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldMethod: "GET", standard.ErrorFieldURL: s.URL + "/x"})
		testutil.Equal(t, "hits", len(s.recorded()), 1)
		testutil.Equal(t, "timers", e.clock.Timers(), 0)
	})

	t.Run("should redact the password of the URL of the errors", func(t *testing.T) {
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: "http://user:secret@127.0.0.1:1/"})
		_, err := e.client.Request(context.Background(), http.MethodGet, "x", nil)
		var fe *flam.Error
		if !errors.As(err, &fe) {
			t.Fatalf("Request() = %v, want a *flam.Error", err)
		}
		testutil.Equal(t, "url", fe.Context()[standard.ErrorFieldURL], "http://user:xxxxx@127.0.0.1:1/x")
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		s := newServer(t, nil, http.StatusOK)
		e := newEnv(t, container.Bag{standard.ConfigFieldBaseURL: s.URL, standard.ConfigFieldBreaker: "cb"})
		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() {
				resp, err := e.client.Request(context.Background(), http.MethodGet, "/c", nil)
				if err == nil {
					_ = read(t, resp)
				}
			})
		}
		wg.Wait()
		testutil.Equal(t, "hits", len(s.recorded()), 10)
		testutil.NoError(t, e.client.Close())
	})
}
