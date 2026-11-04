package middlewares

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/idempotency"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// flakyCache is a cache adaptor delegating to the one it embeds, failing
// its calls with the errors set, and counting its Del calls.
type flakyCache struct {
	flam.ICacheAdaptor

	mu      sync.Mutex
	getErr  error
	setErrs []error // of the SetEphemeral calls, in turn, nil passing
	dels    int
}

func (c *flakyCache) Get(ctx context.Context, objects ...any) ([]any, error) {
	c.mu.Lock()
	err := c.getErr
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return c.ICacheAdaptor.Get(ctx, objects...)
}

func (c *flakyCache) SetEphemeral(ctx context.Context, ttl time.Duration, objects ...any) error {
	c.mu.Lock()
	var err error
	if len(c.setErrs) > 0 {
		err, c.setErrs = c.setErrs[0], c.setErrs[1:]
	}
	c.mu.Unlock()
	if err != nil {
		return err
	}
	return c.ICacheAdaptor.SetEphemeral(ctx, ttl, objects...)
}

func (c *flakyCache) Del(ctx context.Context, objects ...any) error {
	c.mu.Lock()
	c.dels++
	c.mu.Unlock()
	return c.ICacheAdaptor.Del(ctx, objects...)
}

// flakyFactory is a cache adaptor factory giving the flaky cache, which
// embeds the first adaptor it got from the factory it embeds, so that the
// middlewares of several apps share it.
type flakyFactory struct {
	flam.ICacheAdaptorFactory

	cache *flakyCache
}

func (f flakyFactory) Get(id string) (flam.ICacheAdaptor, error) {
	a, err := f.ICacheAdaptorFactory.Get(id)
	if err != nil {
		return nil, err
	}
	f.cache.mu.Lock()
	defer f.cache.mu.Unlock()
	if f.cache.ICacheAdaptor == nil {
		f.cache.ICacheAdaptor = a
	}
	return f.cache, nil
}

// idempotencyOptions are the options of the apps creating the idempotency
// middlewares.
type idempotencyOptions struct {
	noCache bool         // registers no cache bundle
	flaky   *flakyCache  // decorates the cache adaptor factory, when set
	clock   flam.IClock  // the clock of the cache, a fake one at epoch unless set
	driver  flam.IBundle // the driver bundle, the idempotency one unless set
}

// createIdempotency returns the idempotency middleware "main" of the
// fields, created by an app with the cache bundles, configuring the
// memory cache adaptor "store", and the values, along with the clock of
// the cache.
func createIdempotency(t *testing.T, fields container.Bag, values map[string]any, opts idempotencyOptions) (flam.IRestMiddleware, error) {
	t.Helper()
	driver := opts.driver
	if driver == nil {
		driver = idempotency.NewBundle()
	}
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), driver}
	if !opts.noCache {
		bundles = append(bundles, cache.NewBundle(), json.NewBundle(), field.NewBundle(), memory.NewBundle())
	}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = idempotency.Driver
	}
	testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+".main", fields))
	testutil.NoError(t, bag.Set(cache.AdaptorConfigPath+".store", container.Bag{factory.ConfigFieldDriver: memory.Driver}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	clock := opts.clock
	if clock == nil {
		clock = testutil.NewClock(epoch)
	}
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	if opts.flaky != nil {
		testutil.NoError(t, app.Decorate(func(f flam.ICacheAdaptorFactory) flam.ICacheAdaptorFactory {
			return flakyFactory{ICacheAdaptorFactory: f, cache: opts.flaky}
		}))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	if !opts.noCache {
		t.Cleanup(func() { _ = cache.NewBundle().(flam.IClosableBundle).Close(app) })
	}

	var m flam.IRestMiddleware
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) { m, err = f.Get("main") }))
	return m, err
}

// newIdempotency returns the idempotency middleware of the fields, of the
// cache "store" unless set, failing the test when it cannot be created.
func newIdempotency(t *testing.T, fields container.Bag, opts idempotencyOptions) flam.IRestMiddleware {
	t.Helper()
	if !fields.Has(idempotency.ConfigFieldCache) {
		fields[idempotency.ConfigFieldCache] = "store"
	}
	m, err := createIdempotency(t, fields, nil, opts)
	testutil.NoError(t, err)
	return m
}

// idemRequest is a request served through an idempotency middleware.
type idemRequest struct {
	method    string // POST unless set
	path      string // "/items" unless set
	key       string // the Idempotency-Key header, none when empty
	body      string
	principal string
}

// http returns the request.
func (r idemRequest) http() *http.Request {
	method, path := r.method, r.path
	if method == "" {
		method = http.MethodPost
	}
	if path == "" {
		path = "/items"
	}
	req := httptest.NewRequest(method, path, strings.NewReader(r.body))
	if r.key != "" {
		req.Header.Set("Idempotency-Key", r.key)
	}
	if r.principal != "" {
		req = req.WithContext(rest.WithPrincipal(req.Context(), r.principal))
	}
	return req
}

// counter is a handler counting its calls, reading the body of the
// requests, and answering with a 201 response holding the count and the
// body read.
type counter struct {
	mu     sync.Mutex
	calls  int
	status int // 201 unless set
}

func (c *counter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	c.mu.Lock()
	c.calls++
	n := c.calls
	c.mu.Unlock()
	status := c.status
	if status == 0 {
		status = http.StatusCreated
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Location", "/items/1")
	w.Header().Set("X-Other", "1")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(strings.Repeat("#", n) + string(body)))
}

// count returns the number of calls.
func (c *counter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// serveIdem serves the request through the middleware with the handler,
// returning the recorded response.
func serveIdem(m flam.IRestMiddleware, handler http.Handler, r idemRequest) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	m.Wrap(logEndpoint(0), handler).ServeHTTP(rec, r.http())
	return rec
}

// idemProblem returns the problem body of the error of the status.
func idemProblem(status int, err error) string {
	return `{"detail":"` + err.Error() + `","status":` + strconv.Itoa(status) + `,"title":"` + http.StatusText(status) +
		`","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/` + strconv.Itoa(status) + `"}`
}

// checkProblem checks that the response is the problem of the status and
// the error.
func checkProblem(t *testing.T, name string, rec *httptest.ResponseRecorder, status int, err error) {
	t.Helper()
	testutil.Equal(t, name+" status", rec.Code, status)
	testutil.Equal(t, name+" content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
	testutil.Equal(t, name+" body", rec.Body.String(), idemProblem(status, err))
}

func Test_Idempotency_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := idempotency.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.idempotency")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + idempotency.Id: container.Bag{factory.ConfigFieldDriver: idempotency.Driver},
			idempotency.DefaultCacheConfigPath:               nil,
			idempotency.DefaultHeaderConfigPath:              "Idempotency-Key",
			idempotency.DefaultMethodsConfigPath:             []string{"POST", "PATCH"},
			idempotency.DefaultRequiredConfigPath:            false,
			idempotency.DefaultTTLConfigPath:                 "24h0m0s",
			idempotency.DefaultLockTTLConfigPath:             "1m0s",
			idempotency.DefaultMaxBodySizeConfigPath:         1048576,
			idempotency.DefaultHeadersConfigPath:             []string{"Content-Type", "Location"},
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			idempotency.Driver:                       "flam.rest.middlewares.driver.idempotency",
			idempotency.DefaultCacheConfigPath:       "flam.rest.defaults.middlewares.idempotency.cache",
			idempotency.DefaultHeaderConfigPath:      "flam.rest.defaults.middlewares.idempotency.header",
			idempotency.DefaultMethodsConfigPath:     "flam.rest.defaults.middlewares.idempotency.methods",
			idempotency.DefaultRequiredConfigPath:    "flam.rest.defaults.middlewares.idempotency.required",
			idempotency.DefaultTTLConfigPath:         "flam.rest.defaults.middlewares.idempotency.ttl",
			idempotency.DefaultLockTTLConfigPath:     "flam.rest.defaults.middlewares.idempotency.lock_ttl",
			idempotency.DefaultMaxBodySizeConfigPath: "flam.rest.defaults.middlewares.idempotency.max_body_size",
			idempotency.DefaultHeadersConfigPath:     "flam.rest.defaults.middlewares.idempotency.headers",
			idempotency.ReplayedHeader:               "Idempotent-Replayed",
		} {
			testutil.Equal(t, "const", got, want)
		}
		testutil.Equal(t, "Id", idempotency.Id, "idempotency")
		testutil.Equal(t, "MaxKeyLength", idempotency.MaxKeyLength, 255)
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(idempotency.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.idempotency", "provider/1"})
	})
}

func Test_Idempotency(t *testing.T) {
	t.Run("should replay the response stored for a repeated key", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		testutil.Equal(t, "Id()", m.Id(), "main")
		h := &counter{}

		first := serveIdem(m, h, idemRequest{key: "k1", body: "a"})
		testutil.Equal(t, "first status", first.Code, http.StatusCreated)
		testutil.Equal(t, "first body", first.Body.String(), "#a")
		testutil.Equal(t, "first header", first.Header(), http.Header{
			"Content-Type": {"text/plain"}, "Location": {"/items/1"}, "X-Other": {"1"},
		})

		for range 2 {
			again := serveIdem(m, h, idemRequest{key: "k1", body: "a"})
			testutil.Equal(t, "replayed status", again.Code, http.StatusCreated)
			testutil.Equal(t, "replayed body", again.Body.String(), "#a")
			testutil.Equal(t, "replayed header", again.Header(), http.Header{
				"Content-Type": {"text/plain"}, "Location": {"/items/1"}, "Idempotent-Replayed": {"true"},
			})
		}
		testutil.Equal(t, "calls", h.count(), 1)
	})

	t.Run("should scope the keys by principal, method and path", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{}
		for i, r := range []idemRequest{
			{key: "k"},
			{key: "k", principal: "alice"},
			{key: "k", principal: "bob"},
			{key: "k", method: http.MethodPatch},
			{key: "k", path: "/other"},
			{key: "other"},
		} {
			rec := serveIdem(m, h, r)
			testutil.Equal(t, "header", rec.Header().Get(idempotency.ReplayedHeader), "")
			testutil.Equal(t, "calls", h.count(), i+1)
		}
		rec := serveIdem(m, h, idemRequest{key: "k", principal: "alice"})
		testutil.Equal(t, "replayed", rec.Header().Get(idempotency.ReplayedHeader), "true")
		testutil.Equal(t, "replayed body", rec.Body.String(), "##")
	})

	t.Run("should answer a reused key with another body with a 422", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{}
		serveIdem(m, h, idemRequest{key: "k", body: "a"})
		checkProblem(t, "reused", serveIdem(m, h, idemRequest{key: "k", body: "b"}), http.StatusUnprocessableEntity, idempotency.ErrKeyReused)
		testutil.Equal(t, "calls", h.count(), 1)
	})

	t.Run("should serve as they are the requests of other methods or without key", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{}
		for i, r := range []idemRequest{
			{method: http.MethodGet, key: "k"},
			{method: http.MethodGet, key: "k"},
			{},
			{},
		} {
			rec := serveIdem(m, h, r)
			testutil.Equal(t, "body", rec.Body.String(), strings.Repeat("#", i+1))
		}
	})

	t.Run("should answer the requests of invalid keys with a 400", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{idempotency.ConfigFieldRequired: true}, idempotencyOptions{})
		h := &counter{}
		checkProblem(t, "missing", serveIdem(m, h, idemRequest{}), http.StatusBadRequest, idempotency.ErrMissingKey)
		checkProblem(t, "too long", serveIdem(m, h, idemRequest{key: strings.Repeat("k", 256)}), http.StatusBadRequest, idempotency.ErrInvalidKey)
		testutil.Equal(t, "calls", h.count(), 0)
		rec := serveIdem(m, h, idemRequest{key: strings.Repeat("k", 255)})
		testutil.Equal(t, "longest", rec.Code, http.StatusCreated)
		rec = serveIdem(m, h, idemRequest{method: http.MethodGet})
		testutil.Equal(t, "other method", rec.Code, http.StatusCreated)
	})

	t.Run("should answer the requests whose body cannot be read", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{}

		req := idemRequest{key: "k"}.http()
		req.Body = io.NopCloser(io.MultiReader(strings.NewReader("a"), iotestErrReader{}))
		rec := httptest.NewRecorder()
		m.Wrap(logEndpoint(0), h).ServeHTTP(rec, req)
		checkProblem(t, "unreadable", rec, http.StatusBadRequest, idempotency.ErrReadBody)

		req = idemRequest{key: "k", body: "abc"}.http()
		rec = httptest.NewRecorder()
		req.Body = http.MaxBytesReader(rec, req.Body, 2)
		m.Wrap(logEndpoint(0), h).ServeHTTP(rec, req)
		checkProblem(t, "too large", rec, http.StatusRequestEntityTooLarge, idempotency.ErrReadBody)
		testutil.Equal(t, "calls", h.count(), 0)
	})

	t.Run("should take a request without body as an empty one", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{}
		req := idemRequest{key: "k"}.http()
		req.Body = nil
		rec := httptest.NewRecorder()
		m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})).ServeHTTP(rec, req)
		testutil.Equal(t, "body", rec.Body.String(), "ok")
		rec = serveIdem(m, h, idemRequest{key: "k"})
		testutil.Equal(t, "replayed", rec.Header().Get(idempotency.ReplayedHeader), "true")
		testutil.Equal(t, "replayed body", rec.Body.String(), "ok")
		testutil.Equal(t, "replayed status", rec.Code, http.StatusOK)
	})

	t.Run("should store the 4xx responses and the empty ones", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		h := &counter{status: http.StatusConflict}
		serveIdem(m, h, idemRequest{key: "k"})
		rec := serveIdem(m, h, idemRequest{key: "k"})
		testutil.Equal(t, "status", rec.Code, http.StatusConflict)
		testutil.Equal(t, "replayed", rec.Header().Get(idempotency.ReplayedHeader), "true")

		empty := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
		rec = serveIdem(m, empty, idemRequest{key: "e"})
		testutil.Equal(t, "empty status", rec.Code, http.StatusOK)
		rec = serveIdem(m, empty, idemRequest{key: "e"})
		testutil.Equal(t, "empty replayed status", rec.Code, http.StatusOK)
		testutil.Equal(t, "empty replayed header", rec.Header(), http.Header{"Idempotent-Replayed": {"true"}})
		testutil.Equal(t, "empty replayed body", rec.Body.Len(), 0)
		testutil.Equal(t, "calls", h.count(), 1)
	})

	t.Run("should not store the 5xx responses, the long ones and the panics", func(t *testing.T) {
		flaky := &flakyCache{}
		m := newIdempotency(t, container.Bag{idempotency.ConfigFieldMaxBodySize: 3}, idempotencyOptions{flaky: flaky})

		h := &counter{status: http.StatusBadGateway}
		serveIdem(m, h, idemRequest{key: "5xx"})
		serveIdem(m, h, idemRequest{key: "5xx"})
		testutil.Equal(t, "5xx calls", h.count(), 2)

		long := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			for _, part := range []string{"ab", "cd", "ef"} {
				_, _ = w.Write([]byte(part))
			}
		})
		rec := serveIdem(m, long, idemRequest{key: "long"})
		testutil.Equal(t, "long body", rec.Body.String(), "abcdef")
		rec = serveIdem(m, long, idemRequest{key: "long"})
		testutil.Equal(t, "long replayed", rec.Header().Get(idempotency.ReplayedHeader), "")

		calls := 0
		panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
			calls++
			panic(errBoom)
		})
		for range 2 {
			testutil.Panics(t, "ServeHTTP", func() { serveIdem(m, panicking, idemRequest{key: "panic"}) })
		}
		testutil.Equal(t, "panic calls", calls, 2)
		testutil.Equal(t, "dels", flaky.dels, 6)

		rec = serveIdem(m, long, idemRequest{key: "fits", body: "abc"})
		testutil.Equal(t, "fitting body", rec.Body.String(), "abcdef")
		testutil.Equal(t, "dels after", flaky.dels, 7)
		fits := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abc")) })
		serveIdem(m, fits, idemRequest{key: "three"})
		rec = serveIdem(m, fits, idemRequest{key: "three"})
		testutil.Equal(t, "three replayed", rec.Header().Get(idempotency.ReplayedHeader), "true")
		testutil.Equal(t, "three body", rec.Body.String(), "abc")
	})

	t.Run("should expire the responses stored after the ttl", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		m := newIdempotency(t, container.Bag{idempotency.ConfigFieldTTL: "1h"}, idempotencyOptions{clock: clock})
		h := &counter{}
		serveIdem(m, h, idemRequest{key: "k"})
		clock.Advance(time.Hour - time.Second)
		testutil.Equal(t, "replayed", serveIdem(m, h, idemRequest{key: "k"}).Header().Get(idempotency.ReplayedHeader), "true")
		clock.Advance(time.Second)
		testutil.Equal(t, "expired", serveIdem(m, h, idemRequest{key: "k"}).Header().Get(idempotency.ReplayedHeader), "")
		testutil.Equal(t, "calls", h.count(), 2)
	})

	t.Run("should answer a repeated key in flight with a 409", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		flaky := &flakyCache{}
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{flaky: flaky, clock: clock})
		other := newIdempotency(t, container.Bag{}, idempotencyOptions{flaky: flaky, clock: clock})

		entered, release := make(chan struct{}), make(chan struct{})
		blocking := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(entered)
			<-release
			_, _ = w.Write([]byte("done"))
		})
		done := make(chan *httptest.ResponseRecorder)
		go func() { done <- serveIdem(m, blocking, idemRequest{key: "k", body: "a"}) }()
		<-entered

		h := &counter{}
		checkProblem(t, "same process", serveIdem(m, h, idemRequest{key: "k", body: "a"}), http.StatusConflict, idempotency.ErrInFlight)
		checkProblem(t, "same process, other body", serveIdem(m, h, idemRequest{key: "k", body: "b"}), http.StatusConflict, idempotency.ErrInFlight)
		checkProblem(t, "other process", serveIdem(other, h, idemRequest{key: "k", body: "a"}), http.StatusConflict, idempotency.ErrInFlight)
		checkProblem(t, "other process, other body", serveIdem(other, h, idemRequest{key: "k", body: "b"}), http.StatusUnprocessableEntity, idempotency.ErrKeyReused)
		testutil.Equal(t, "calls", h.count(), 0)

		clock.Advance(time.Minute)
		testutil.Equal(t, "lock expired", serveIdem(other, h, idemRequest{key: "k", body: "a"}).Body.String(), "#a")

		close(release)
		testutil.Equal(t, "first body", (<-done).Body.String(), "done")
		rec := serveIdem(m, h, idemRequest{key: "k", body: "a"})
		testutil.Equal(t, "replayed", rec.Header().Get(idempotency.ReplayedHeader), "true")
		testutil.Equal(t, "replayed body", rec.Body.String(), "done")
	})

	t.Run("should store the response once the client went away", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		ctx, cancel := context.WithCancel(context.Background())
		req := idemRequest{key: "k"}.http().WithContext(ctx)
		m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			cancel()
			_, _ = w.Write([]byte("ok"))
		})).ServeHTTP(httptest.NewRecorder(), req)
		rec := serveIdem(m, &counter{}, idemRequest{key: "k"})
		testutil.Equal(t, "replayed body", rec.Body.String(), "ok")
	})

	t.Run("should answer with a 503 when the cache fails", func(t *testing.T) {
		flaky := &flakyCache{}
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{flaky: flaky})
		h := &counter{}

		flaky.getErr = errBoom
		checkProblem(t, "get", serveIdem(m, h, idemRequest{key: "k"}), http.StatusServiceUnavailable, idempotency.ErrCache)
		flaky.getErr = nil

		flaky.setErrs = []error{errBoom}
		checkProblem(t, "marker", serveIdem(m, h, idemRequest{key: "k"}), http.StatusServiceUnavailable, idempotency.ErrCache)
		testutil.Equal(t, "calls", h.count(), 0)

		flaky.setErrs = []error{nil, errBoom}
		testutil.Equal(t, "unstored body", serveIdem(m, h, idemRequest{key: "k"}).Body.String(), "#")
		testutil.Equal(t, "unstored dels", flaky.dels, 1)
		testutil.Equal(t, "again body", serveIdem(m, h, idemRequest{key: "k"}).Body.String(), "##")
		testutil.Equal(t, "replayed body", serveIdem(m, h, idemRequest{key: "k"}).Body.String(), "##")
		testutil.Equal(t, "dels", flaky.dels, 1)
	})

	t.Run("should let the handler flush and unwrap the writer", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{}, idempotencyOptions{})
		rec := httptest.NewRecorder()
		var unwrapped http.ResponseWriter
		m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("a"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
			unwrapped = w.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
		})).ServeHTTP(rec, idemRequest{key: "k"}.http())
		testutil.Equal(t, "flushed", rec.Flushed, true)
		_, ok := unwrapped.(flam.IRestResponseWriter)
		testutil.Equal(t, "unwrapped", ok, true)
	})

	t.Run("should pass the requests of a streaming endpoint through", func(t *testing.T) {
		m := newIdempotency(t, container.Bag{idempotency.ConfigFieldRequired: true}, idempotencyOptions{})
		h := &counter{}
		for i := range 2 {
			rec := httptest.NewRecorder()
			m.Wrap(flam.RestEndpoint{Pattern: "POST /items", Streaming: true}, h).ServeHTTP(rec, idemRequest{key: "k"}.http())
			testutil.Equal(t, "body", rec.Body.String(), strings.Repeat("#", i+1))
			testutil.Equal(t, "replayed", rec.Header().Get(idempotency.ReplayedHeader), "")
		}
		rec := httptest.NewRecorder()
		m.Wrap(flam.RestEndpoint{Pattern: "POST /items", Streaming: true}, h).ServeHTTP(rec, idemRequest{}.http())
		testutil.Equal(t, "without key", rec.Code, http.StatusCreated)
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		m, err := createIdempotency(t, container.Bag{
			idempotency.ConfigFieldHeader:  "x-key",
			idempotency.ConfigFieldMethods: []string{" PUT "},
			idempotency.ConfigFieldHeaders: []string{" x-other "},
		}, map[string]any{
			idempotency.DefaultCacheConfigPath:       "store",
			idempotency.DefaultRequiredConfigPath:    true,
			idempotency.DefaultTTLConfigPath:         "2h",
			idempotency.DefaultLockTTLConfigPath:     time.Second,
			idempotency.DefaultMaxBodySizeConfigPath: 2,
		}, idempotencyOptions{clock: clock})
		testutil.NoError(t, err)
		h := &counter{}
		req := func(method string, key string) *httptest.ResponseRecorder {
			r := httptest.NewRequest(method, "/items", nil)
			if key != "" {
				r.Header.Set("X-Key", key)
			}
			rec := httptest.NewRecorder()
			m.Wrap(logEndpoint(0), h).ServeHTTP(rec, r)
			return rec
		}
		testutil.Equal(t, "post", req(http.MethodPost, "").Code, http.StatusCreated)
		checkProblem(t, "required", req(http.MethodPut, ""), http.StatusBadRequest, idempotency.ErrMissingKey)
		req(http.MethodPut, "k")
		rec := req(http.MethodPut, "k")
		testutil.Equal(t, "replayed header", rec.Header(), http.Header{"X-Other": {"1"}, "Idempotent-Replayed": {"true"}})
		clock.Advance(2 * time.Hour)
		testutil.Equal(t, "expired", req(http.MethodPut, "k").Header().Get(idempotency.ReplayedHeader), "")

		m, err = createIdempotency(t, container.Bag{
			idempotency.ConfigFieldCache:       "store",
			idempotency.ConfigFieldRequired:    false,
			idempotency.ConfigFieldTTL:         time.Hour,
			idempotency.ConfigFieldLockTTL:     "1s",
			idempotency.ConfigFieldMaxBodySize: 0,
		}, map[string]any{
			idempotency.DefaultCacheConfigPath:   "unknown",
			idempotency.DefaultHeaderConfigPath:  "X-Key",
			idempotency.DefaultMethodsConfigPath: []any{"DELETE"},
			idempotency.DefaultHeadersConfigPath: []any{},
		}, idempotencyOptions{})
		testutil.NoError(t, err)
		empty := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Header().Set("Location", "/a") })
		for range 2 {
			r := httptest.NewRequest(http.MethodDelete, "/items", nil)
			r.Header.Set("X-Key", "k")
			rec = httptest.NewRecorder()
			m.Wrap(logEndpoint(0), empty).ServeHTTP(rec, r)
		}
		testutil.Equal(t, "second replayed header", rec.Header(), http.Header{"Idempotent-Replayed": {"true"}})
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		m, err := createIdempotency(t, container.Bag{idempotency.ConfigFieldCache: "store"}, nil,
			idempotencyOptions{driver: bare{idempotency.NewBundle()}})
		testutil.NoError(t, err)
		h := &counter{}
		serveIdem(m, h, idemRequest{key: "k", method: http.MethodPatch})
		rec := serveIdem(m, h, idemRequest{key: "k", method: http.MethodPatch})
		testutil.Equal(t, "replayed header", rec.Header(), http.Header{
			"Content-Type": {"text/plain"}, "Location": {"/items/1"}, "Idempotent-Replayed": {"true"},
		})
		serveIdem(m, h, idemRequest{method: http.MethodPost})
		testutil.Equal(t, "calls", h.count(), 2)

		_, err = createIdempotency(t, container.Bag{}, nil, idempotencyOptions{driver: bare{idempotency.NewBundle()}})
		testutil.Error(t, errors.Unwrap(err), idempotency.ErrInvalidField,
			"unable to create idempotency rest middleware: invalid idempotency rest middleware field",
			map[string]any{idempotency.ErrorFieldID: "main", idempotency.ErrorFieldField: idempotency.ConfigFieldCache})
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{idempotency.ConfigFieldCache, ""},
			{idempotency.ConfigFieldCache, 1},
			{idempotency.ConfigFieldHeader, ""},
			{idempotency.ConfigFieldHeader, true},
			{idempotency.ConfigFieldMethods, []any{}},
			{idempotency.ConfigFieldMethods, []any{"POST", " "}},
			{idempotency.ConfigFieldMethods, "POST"},
			{idempotency.ConfigFieldRequired, "true"},
			{idempotency.ConfigFieldTTL, "0s"},
			{idempotency.ConfigFieldTTL, -time.Second},
			{idempotency.ConfigFieldTTL, 60},
			{idempotency.ConfigFieldLockTTL, time.Duration(0)},
			{idempotency.ConfigFieldLockTTL, "soon"},
			{idempotency.ConfigFieldMaxBodySize, -1},
			{idempotency.ConfigFieldMaxBodySize, "1"},
			{idempotency.ConfigFieldHeaders, []any{1}},
			{idempotency.ConfigFieldHeaders, "Location"},
		} {
			fields := container.Bag{idempotency.ConfigFieldCache: "store"}
			fields[tc.field] = tc.value
			_, err := createIdempotency(t, fields, nil, idempotencyOptions{})
			testutil.Error(t, errors.Unwrap(err), idempotency.ErrInvalidField,
				"unable to create idempotency rest middleware: invalid idempotency rest middleware field",
				map[string]any{idempotency.ErrorFieldID: "main", idempotency.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		for path, field := range map[string]string{
			idempotency.DefaultCacheConfigPath:       idempotency.ConfigFieldCache,
			idempotency.DefaultHeaderConfigPath:      idempotency.ConfigFieldHeader,
			idempotency.DefaultMethodsConfigPath:     idempotency.ConfigFieldMethods,
			idempotency.DefaultRequiredConfigPath:    idempotency.ConfigFieldRequired,
			idempotency.DefaultTTLConfigPath:         idempotency.ConfigFieldTTL,
			idempotency.DefaultLockTTLConfigPath:     idempotency.ConfigFieldLockTTL,
			idempotency.DefaultMaxBodySizeConfigPath: idempotency.ConfigFieldMaxBodySize,
			idempotency.DefaultHeadersConfigPath:     idempotency.ConfigFieldHeaders,
		} {
			fields := container.Bag{}
			if field != idempotency.ConfigFieldCache {
				fields[idempotency.ConfigFieldCache] = "store"
			}
			_, err := createIdempotency(t, fields, map[string]any{path: 42.5}, idempotencyOptions{})
			testutil.Error(t, errors.Unwrap(err), idempotency.ErrInvalidField,
				"unable to create idempotency rest middleware: invalid idempotency rest middleware field",
				map[string]any{idempotency.ErrorFieldID: "main", idempotency.ErrorFieldField: field})
		}
	})

	t.Run("should fail without cache adaptor factory", func(t *testing.T) {
		_, err := createIdempotency(t, container.Bag{idempotency.ConfigFieldCache: "store"}, nil, idempotencyOptions{noCache: true})
		testutil.Error(t, errors.Unwrap(err), idempotency.ErrNoFactory,
			"unable to create idempotency rest middleware: no cache adaptor factory",
			map[string]any{idempotency.ErrorFieldID: "main", idempotency.ErrorFieldCache: "store"})
	})

	t.Run("should fail with an unknown cache adaptor", func(t *testing.T) {
		_, err := createIdempotency(t, container.Bag{idempotency.ConfigFieldCache: "unknown"}, nil, idempotencyOptions{})
		cause := errors.Unwrap(err)
		if !errors.Is(cause, idempotency.ErrCacheAdaptor) || !errors.Is(cause, factory.ErrUnknownID) {
			t.Fatalf("error = %v, want it to wrap ErrCacheAdaptor and factory.ErrUnknownID", cause)
		}
		var e *flam.Error
		if !errors.As(cause, &e) {
			t.Fatalf("error = %T, want a *flam.Error", cause)
		}
		testutil.Equal(t, "prefix", strings.HasPrefix(e.Error(),
			"unable to create idempotency rest middleware: unable to get idempotency cache adaptor: "), true)
		testutil.Context(t, e, map[string]any{idempotency.ErrorFieldID: "main", idempotency.ErrorFieldCache: "unknown"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createIdempotency(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, idempotencyOptions{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

// iotestErrReader is a reader failing with errBoom.
type iotestErrReader struct{}

func (iotestErrReader) Read([]byte) (int, error) { return 0, errBoom }
