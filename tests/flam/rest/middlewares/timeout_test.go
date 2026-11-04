package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/timeout"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// timeoutBody is the problem answering a request of the endpoint 7 of the
// service 3 whose handler took too long.
const timeoutBody = `{"detail":"request timed out","id":"s:3.e:7","status":503,` +
	`"title":"Service Unavailable","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503"}`

// createTimeout returns the timeout middleware "main" of the fields,
// created with the timeout bundle and the values.
func createTimeout(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, timeout.NewBundle(), timeout.Driver, fields, values, deps{})
}

// newTimeout returns the timeout middleware of the timeout, failing the
// test when it cannot be created.
func newTimeout(t *testing.T, d string) flam.IRestMiddleware {
	t.Helper()
	m, err := createTimeout(t, container.Bag{timeout.ConfigFieldTimeout: d}, nil)
	testutil.NoError(t, err)
	return m
}

// outerHeader is a middleware setting the "X-Outer" header of the
// responses before calling the handler.
type outerHeader struct{}

func (outerHeader) Id() string { return "outer" }

func (outerHeader) Wrap(_ flam.RestEndpoint, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("X-Outer", "1")
		next.ServeHTTP(w, req)
	})
}

// waitHandler returns the handler waiting for its request to be done, then
// for served to be closed, so that the request is answered, then calling
// after with its writer, if not nil, and closing finished.
func waitHandler(served <-chan struct{}, finished chan<- struct{}, after func(w http.ResponseWriter)) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer close(finished)
		w.Header().Set("X-Inner", "1")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		<-req.Context().Done()
		<-served
		if after != nil {
			after(w)
		}
	}
}

func Test_Timeout_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := timeout.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.timeout")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + timeout.Id: container.Bag{factory.ConfigFieldDriver: timeout.Driver},
			timeout.DefaultTimeoutConfigPath:             "30s",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			timeout.Driver:                   "flam.rest.middlewares.driver.timeout",
			timeout.DefaultTimeoutConfigPath: "flam.rest.defaults.middlewares.timeout.timeout",
		} {
			testutil.Equal(t, "const", got, want)
		}
		testutil.Equal(t, "Id", timeout.Id, "timeout")
		testutil.Equal(t, "field", timeout.ConfigFieldTimeout, "timeout")
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(timeout.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.timeout", "provider/1"})
	})
}

func Test_Timeout(t *testing.T) {
	t.Run("should send the response of a handler returning in time", func(t *testing.T) {
		m := newTimeout(t, "1m")
		testutil.Equal(t, "Id()", m.Id(), "main")

		var deadline time.Duration
		rec, panicked := serve(t, "/items/1", func(w http.ResponseWriter, req *http.Request) {
			d, ok := req.Context().Deadline()
			testutil.Equal(t, "deadline", ok, true)
			deadline = time.Until(d)
			testutil.Equal(t, "outer header seen", w.Header().Get("X-Outer"), "1")
			w.Header().Del("X-Outer")
			w.Header().Set("X-Inner", "1")
			w.WriteHeader(http.StatusEarlyHints)
			w.WriteHeader(http.StatusCreated)
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte("cre"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
			_, _ = w.Write([]byte("ated"))
		}, outerHeader{}, m)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "deadline within", deadline > 0 && deadline <= time.Minute, true)
		testutil.Equal(t, "status", rec.Code, http.StatusCreated)
		testutil.Equal(t, "body", rec.Body.String(), "created")
		testutil.Equal(t, "header", rec.Header(), http.Header{"X-Inner": {"1"}})
	})

	t.Run("should send a 200 when the handler writes nothing", func(t *testing.T) {
		rec, _ := serve(t, "/items/1", func(http.ResponseWriter, *http.Request) {}, newTimeout(t, "1m"))
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "body", rec.Body.String(), "")
		testutil.Equal(t, "flushed", rec.Flushed, false)
	})

	t.Run("should send a 200 when the handler writes the body only", func(t *testing.T) {
		rec, _ := serve(t, "/items/1", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		}, newTimeout(t, "1m"))
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "body", rec.Body.String(), "ok")
	})

	t.Run("should answer with a 503 envelope once the timeout passes", func(t *testing.T) {
		served, finished := make(chan struct{}), make(chan struct{})
		var writeErr error
		var n int
		rec, panicked := serve(t, "/items/1", waitHandler(served, finished, func(w http.ResponseWriter) {
			w.WriteHeader(http.StatusOK)
			n, writeErr = w.Write([]byte("late"))
		}), outerHeader{}, newTimeout(t, "20ms"))
		close(served)
		<-finished

		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusServiceUnavailable)
		testutil.Equal(t, "body", rec.Body.String(), timeoutBody)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"X-Outer":      {"1"},
			"Content-Type": {flamenvelope.ContentTypeProblem},
		})
		testutil.Equal(t, "late write", n, 0)
		testutil.Equal(t, "late write error", writeErr, http.ErrHandlerTimeout)
	})

	t.Run("should answer with a 503 envelope once the request is canceled", func(t *testing.T) {
		m := newTimeout(t, "1m")
		ctx, cancel := context.WithCancel(context.Background())
		served, finished := make(chan struct{}), make(chan struct{})
		req := httptest.NewRequestWithContext(ctx, http.MethodGet, "/items/1", nil)
		rec := httptest.NewRecorder()
		go cancel()
		m.Wrap(logEndpoint(0), waitHandler(served, finished, nil)).ServeHTTP(rec, req)
		close(served)
		<-finished

		testutil.Equal(t, "status", rec.Code, http.StatusServiceUnavailable)
		testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
		testutil.Equal(t, "body", rec.Body.String(), `{"detail":"request timed out","status":503,`+
			`"title":"Service Unavailable","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503"}`)
	})

	t.Run("should let the middlewares listed before and after record the response", func(t *testing.T) {
		for name, tc := range map[string]struct {
			duration string
			handler  func(served <-chan struct{}, finished chan<- struct{}) http.HandlerFunc
			status   int
			inner    any
		}{
			"in time": {
				duration: "1m",
				handler: func(_ <-chan struct{}, finished chan<- struct{}) http.HandlerFunc {
					return func(w http.ResponseWriter, _ *http.Request) {
						defer close(finished)
						w.WriteHeader(http.StatusAccepted)
						_, _ = w.Write([]byte("ok"))
					}
				},
				status: http.StatusAccepted,
				inner:  http.StatusAccepted,
			},
			"timed out": {
				duration: "20ms",
				handler: func(served <-chan struct{}, finished chan<- struct{}) http.HandlerFunc {
					return waitHandler(served, finished, nil)
				},
				status: http.StatusServiceUnavailable,
				inner:  http.StatusAccepted,
			},
		} {
			t.Run(name, func(t *testing.T) {
				outer, outerLogger, _ := newLog(t, container.Bag{})
				inner, innerLogger, _ := newLog(t, container.Bag{})
				served, finished := make(chan struct{}), make(chan struct{})
				rec, panicked := serve(t, "/items/1", tc.handler(served, finished), outer, newTimeout(t, tc.duration), inner)
				close(served)
				<-finished

				testutil.Equal(t, "panicked", panicked, nil)
				testutil.Equal(t, "status", rec.Code, tc.status)
				outerEntries := outerLogger.all()
				testutil.Equal(t, "outer count", len(outerEntries), 2)
				testutil.Equal(t, "outer status", outerEntries[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldStatus), any(tc.status))
				for deadline := time.Now().Add(5 * time.Second); len(innerLogger.all()) < 2 && time.Now().Before(deadline); {
					time.Sleep(time.Millisecond)
				}
				innerEntries := innerLogger.all()
				testutil.Equal(t, "inner count", len(innerEntries), 2)
				testutil.Equal(t, "inner status", innerEntries[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldStatus), tc.inner)
			})
		}
	})

	t.Run("should panic with the value the handler panicked with in time", func(t *testing.T) {
		rec, panicked := serve(t, "/items/1", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("partial"))
			panic(errBoom)
		}, newTimeout(t, "1m"))
		testutil.Equal(t, "panicked", panicked, any(errBoom))
		testutil.Equal(t, "body", rec.Body.String(), "")

		r, err := create(t, envelope.NewBundle(), envelope.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		rec, panicked = serve(t, "/items/1", func(http.ResponseWriter, *http.Request) {
			panic(flamenvelope.NewEnvelope().WithNewError(http.StatusConflict, errBoom))
		}, r, newTimeout(t, "1m"))
		testutil.Equal(t, "envelope panicked", panicked, nil)
		testutil.Equal(t, "envelope status", rec.Code, http.StatusConflict)
	})

	t.Run("should drop the panic of the handler once the request is answered", func(t *testing.T) {
		served, finished := make(chan struct{}), make(chan struct{})
		rec, panicked := serve(t, "/items/1", waitHandler(served, finished, func(http.ResponseWriter) {
			panic(errBoom)
		}), newTimeout(t, "20ms"))
		close(served)
		<-finished

		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusServiceUnavailable)
	})

	t.Run("should take the field, then its default", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields container.Bag
			values map[string]any
			want   time.Duration
		}{
			"field duration": {container.Bag{timeout.ConfigFieldTimeout: 2 * time.Minute}, map[string]any{timeout.DefaultTimeoutConfigPath: "1h"}, 2 * time.Minute},
			"field text":     {container.Bag{timeout.ConfigFieldTimeout: "3m"}, map[string]any{timeout.DefaultTimeoutConfigPath: "1h"}, 3 * time.Minute},
			"default":        {container.Bag{}, map[string]any{timeout.DefaultTimeoutConfigPath: "1h"}, time.Hour},
			"bundle default": {container.Bag{}, nil, 30 * time.Second},
		} {
			m, err := createTimeout(t, tc.fields, tc.values)
			testutil.NoError(t, err)
			var deadline time.Duration
			serve(t, "/items/1", func(_ http.ResponseWriter, req *http.Request) {
				d, _ := req.Context().Deadline()
				deadline = time.Until(d)
			}, m)
			testutil.Equal(t, name+" deadline", deadline > tc.want-time.Second && deadline <= tc.want, true)
		}
	})

	t.Run("should use the built-in default when the default is missing", func(t *testing.T) {
		m, err := create(t, bare{timeout.NewBundle()}, timeout.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		var deadline time.Duration
		serve(t, "/items/1", func(_ http.ResponseWriter, req *http.Request) {
			d, _ := req.Context().Deadline()
			deadline = time.Until(d)
		}, m)
		testutil.Equal(t, "deadline", deadline > 29*time.Second && deadline <= 30*time.Second, true)
	})

	t.Run("should fail with an invalid field or default", func(t *testing.T) {
		for _, value := range []any{"0s", time.Duration(0), -time.Second, "soon", 30, nil} {
			_, err := createTimeout(t, container.Bag{timeout.ConfigFieldTimeout: value}, nil)
			testutil.Error(t, errors.Unwrap(err), timeout.ErrInvalidField,
				"unable to create timeout rest middleware: invalid timeout rest middleware field",
				map[string]any{timeout.ErrorFieldID: "main", timeout.ErrorFieldField: timeout.ConfigFieldTimeout})
		}
		_, err := createTimeout(t, container.Bag{}, map[string]any{timeout.DefaultTimeoutConfigPath: 30})
		testutil.Error(t, errors.Unwrap(err), timeout.ErrInvalidField,
			"unable to create timeout rest middleware: invalid timeout rest middleware field",
			map[string]any{timeout.ErrorFieldID: "main", timeout.ErrorFieldField: timeout.ConfigFieldTimeout})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createTimeout(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
