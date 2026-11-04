package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/bodylimit"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/cors"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/requestid"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/timeout"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// epoch is the time of the fake clocks.
var epoch = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

// entry is a signaled log entry.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     flam.IBag
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
	e.signaled = append(e.signaled, entry{level, channel, message, ctx[0]})
}

// all returns a copy of the signaled entries.
func (e *entries) all() []entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]entry(nil), e.signaled...)
}

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// deps are the values provided to the container of an app, when not nil.
type deps struct {
	logger flam.ILogger
	clock  flam.IClock
}

// create returns the middleware "main" of the fields, the driver of the
// bundle unless set, created by the middleware factory of an app with the
// driver bundle, the defaults overridden by the values and the deps.
func create(t *testing.T, driver flam.IBundle, name string, fields container.Bag, values map[string]any, d deps) (flam.IRestMiddleware, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), driver}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = name
	}
	testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	if d.logger != nil {
		testutil.NoError(t, app.Provide(func() flam.ILogger { return d.logger }))
	}
	if d.clock != nil {
		testutil.NoError(t, app.Provide(func() flam.IClock { return d.clock }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var m flam.IRestMiddleware
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) { m, err = f.Get("main") }))
	return m, err
}

// serve serves a GET request of the path with a router of the service 3
// holding the endpoint 7 of the pattern "GET /items/{id}", wrapped in the
// middlewares, returning the recorded response and the value its serving
// panicked with, if any.
func serve(t *testing.T, path string, handler http.HandlerFunc, mws ...flam.IRestMiddleware) (rec *httptest.ResponseRecorder, panicked any) {
	t.Helper()
	router, err := rest.NewRouter(3, mws...)
	testutil.NoError(t, err)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: "GET /items/{id}"}, handler))

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.RemoteAddr = "10.0.0.1:4321"
	defer func() { panicked = recover() }()
	router.ServeHTTP(rec, req)
	return rec, nil
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := middlewares.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.rest.middlewares",
			"bundle:flam.rest.middlewares.envelope", "provider/1",
			"bundle:flam.rest.middlewares.log", "provider/1",
			"bundle:flam.rest.middlewares.jwt", "provider/1",
			"bundle:flam.rest.middlewares.apikey", "provider/1",
			"bundle:flam.rest.middlewares.ratelimit", "provider/1",
			"bundle:flam.rest.middlewares.requestid", "provider/1",
			"bundle:flam.rest.middlewares.cors", "provider/1",
			"bundle:flam.rest.middlewares.bodylimit", "provider/1",
			"bundle:flam.rest.middlewares.timeout", "provider/1",
		})
	})

	t.Run("should configure the envelope middleware", func(t *testing.T) {
		b := envelope.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.envelope")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + envelope.Id: container.Bag{factory.ConfigFieldDriver: envelope.Driver},
			envelope.DefaultChannelConfigPath:             "flam.rest",
			envelope.DefaultLevelConfigPath:               "error",
			envelope.DefaultExposeErrorsConfigPath:        false,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", envelope.Id, "envelope")
		testutil.Equal(t, "Driver", envelope.Driver, "flam.rest.middlewares.driver.envelope")
		testutil.Equal(t, "default channel path", envelope.DefaultChannelConfigPath, "flam.rest.defaults.middlewares.envelope.channel")
		testutil.Equal(t, "default level path", envelope.DefaultLevelConfigPath, "flam.rest.defaults.middlewares.envelope.level")
		testutil.Equal(t, "default expose errors path", envelope.DefaultExposeErrorsConfigPath, "flam.rest.defaults.middlewares.envelope.expose_errors")
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should configure the log middleware", func(t *testing.T) {
		b := log.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.log")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + log.Id: container.Bag{factory.ConfigFieldDriver: log.Driver},
			log.DefaultChannelConfigPath:             "flam.rest",
			log.DefaultLevelRequestConfigPath:        "info",
			log.DefaultLevelResponseConfigPath:       "info",
			log.DefaultLevelErrorConfigPath:          "error",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", log.Id, "log")
		testutil.Equal(t, "Driver", log.Driver, "flam.rest.middlewares.driver.log")
		testutil.Equal(t, "default channel path", log.DefaultChannelConfigPath, "flam.rest.defaults.middlewares.log.channel")
		testutil.Equal(t, "default level error path", log.DefaultLevelErrorConfigPath, "flam.rest.defaults.middlewares.log.level_error")
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should create the configured middlewares through the aggregate bundle", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.ILogger { return &entries{} }))
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(epoch) }))
		bag := container.Bag{}
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), envelope.NewBundle(), log.NewBundle(),
			requestid.NewBundle(), cors.NewBundle(), bodylimit.NewBundle(), timeout.NewBundle()} {
			if configurable, ok := b.(flam.IConfigurableBundle); ok {
				testutil.NoError(t, configurable.Config(bag))
			}
		}
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), middlewares.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) {
			for _, id := range []string{envelope.Id, log.Id, requestid.Id, cors.Id, bodylimit.Id, timeout.Id} {
				m, err := f.Get(id)
				testutil.NoError(t, err)
				testutil.Equal(t, id+" Id()", m.Id(), id)
			}
		}))
	})
}

func Test_Middlewares_Order(t *testing.T) {
	t.Run("should log the answered panic as a 500 when the envelope is inner", func(t *testing.T) {
		logger := &entries{}
		r, err := create(t, envelope.NewBundle(), envelope.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		l, err := create(t, log.NewBundle(), log.Driver, container.Bag{}, nil, deps{logger: logger, clock: testutil.NewClock(epoch)})
		testutil.NoError(t, err)

		rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, l, r)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusInternalServerError)
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "level", got[1].level, flam.LogError)
		testutil.Equal(t, "logged status", got[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldStatus), any(http.StatusInternalServerError))
		testutil.Equal(t, "logged panicked", got[1].ctx.Has(log.ContextFieldPanicked), false)
	})

	t.Run("should log the panic when the envelope is outer", func(t *testing.T) {
		logger := &entries{}
		r, err := create(t, envelope.NewBundle(), envelope.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		l, err := create(t, log.NewBundle(), log.Driver, container.Bag{}, nil, deps{logger: logger, clock: testutil.NewClock(epoch)})
		testutil.NoError(t, err)

		rec, panicked := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) { panic(errBoom) }, r, l)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusInternalServerError)
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "level", got[1].level, flam.LogError)
		testutil.Equal(t, "logged status", got[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldStatus), any(0))
		testutil.Equal(t, "logged panicked", got[1].ctx.Get(log.ContextFieldPanicked), any(true))
	})
}
