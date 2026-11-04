package middlewares

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createLog returns the log middleware "main" of the fields, created with
// the log bundle, the values, the clock and the logger when not nil.
func createLog(t *testing.T, fields container.Bag, values map[string]any, logger flam.ILogger, clock flam.IClock) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, log.NewBundle(), log.Driver, fields, values, deps{logger: logger, clock: clock})
}

// logEndpoint is the endpoint 7 of the service 3 of the log tests, of the
// expected status, 0 meaning 200.
func logEndpoint(status int) flam.RestEndpoint {
	return flam.RestEndpoint{ServiceId: 3, Id: 7, Pattern: "GET /items/{id}", Status: status}
}

// logRequest returns a request of the method and the target, from the
// remote address "10.0.0.1:4321", with the body, nil leaving it empty.
func logRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.RemoteAddr = "10.0.0.1:4321"
	return req
}

// serveLog serves the request with the handler wrapped by the middleware
// for the endpoint, returning the recorded response and the value its
// serving panicked with, if any.
func serveLog(m flam.IRestMiddleware, endpoint flam.RestEndpoint, req *http.Request, handler http.HandlerFunc) (rec *httptest.ResponseRecorder, panicked any) {
	rec = httptest.NewRecorder()
	defer func() { panicked = recover() }()
	m.Wrap(endpoint, handler).ServeHTTP(rec, req)
	return rec, nil
}

// requested returns the request bag of a GET of the path "/items/1"
// served by the endpoint 7 of the service 3, with no header, parameter or
// body, extended by the fields.
func requested(fields container.Bag) container.Bag {
	bag := container.Bag{
		log.ContextFieldMethod:   http.MethodGet,
		log.ContextFieldPath:     "/items/1",
		log.ContextFieldPattern:  "GET /items/{id}",
		log.ContextFieldEndpoint: 7,
		log.ContextFieldService:  3,
		log.ContextFieldRemote:   "10.0.0.1:4321",
		log.ContextFieldHeaders:  container.Bag{},
		log.ContextFieldParams:   container.Bag{},
		log.ContextFieldBody:     "",
	}
	for k, v := range fields {
		bag[k] = v
	}
	return bag
}

// responded returns the response bag of the status and the size, with no
// header, extended by the fields.
func responded(status, size int, fields container.Bag) container.Bag {
	bag := container.Bag{
		log.ContextFieldStatus:  status,
		log.ContextFieldSize:    size,
		log.ContextFieldHeaders: container.Bag{},
	}
	for k, v := range fields {
		bag[k] = v
	}
	return bag
}

// sent returns the context of the entry of the response to the request,
// taking a second and a half.
func sent(request, response container.Bag) flam.IBag {
	return container.Bag{
		log.ContextFieldRequest:  request,
		log.ContextFieldResponse: response,
		log.ContextFieldDuration: 1500 * time.Millisecond,
	}
}

// trackedBody is a request body recording its reads and closing, failing
// with its error, when not nil, once its content is read.
type trackedBody struct {
	r      io.Reader
	err    error
	reads  int
	closed bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	b.reads++
	n, err := b.r.Read(p)
	if err == io.EOF && b.err != nil {
		return n, b.err
	}
	return n, err
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

// discard is a response writer of the header, discarding what is written.
type discard struct{ header http.Header }

func (d discard) Header() http.Header { return d.header }

func (d discard) Write(p []byte) (int, error) { return len(p), nil }

func (d discard) WriteHeader(int) {}

// newLog returns a log middleware of the fields, along with its logger and
// clock, failing the test when it cannot be created.
func newLog(t *testing.T, fields container.Bag) (flam.IRestMiddleware, *entries, *testutil.Clock) {
	t.Helper()
	logger := &entries{}
	clock := testutil.NewClock(epoch)
	m, err := createLog(t, fields, nil, logger, clock)
	testutil.NoError(t, err)
	return m, logger, clock
}

func Test_Log_Bundle(t *testing.T) {
	t.Run("should configure the defaults of the fields", func(t *testing.T) {
		b := log.NewBundle()
		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			log.DefaultChannelConfigPath:       "flam.rest",
			log.DefaultLevelRequestConfigPath:  "info",
			log.DefaultLevelResponseConfigPath: "info",
			log.DefaultLevelErrorConfigPath:    "error",
			log.DefaultParseJSONConfigPath:     false,
			log.DefaultMaxBodyConfigPath:       65536,
			log.DefaultRedactHeadersConfigPath: []string{"Authorization", "Cookie", "Set-Cookie", "Proxy-Authorization"},
			log.DefaultRedactParamsConfigPath:  []string{"access_token", "api_key", "apikey", "password", "secret", "token"},
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			log.DefaultLevelRequestConfigPath:  "flam.rest.defaults.middlewares.log.level_request",
			log.DefaultLevelResponseConfigPath: "flam.rest.defaults.middlewares.log.level_response",
			log.DefaultParseJSONConfigPath:     "flam.rest.defaults.middlewares.log.parse_json",
			log.DefaultMaxBodyConfigPath:       "flam.rest.defaults.middlewares.log.max_body",
			log.DefaultRedactHeadersConfigPath: "flam.rest.defaults.middlewares.log.redact_headers",
			log.DefaultRedactParamsConfigPath:  "flam.rest.defaults.middlewares.log.redact_params",
			log.ConfigFieldRedactParams:        "redact_params",
		} {
			testutil.Equal(t, "path", got, want)
		}
	})
}

func Test_Log(t *testing.T) {
	t.Run("should signal the request and its response at the level of its status", func(t *testing.T) {
		for _, tc := range []struct {
			status int
			level  flam.LogLevel
			body   bool
		}{
			{http.StatusOK, flam.LogInfo, false},
			{http.StatusNotFound, flam.LogInfo, true},
			{http.StatusInternalServerError - 1, flam.LogInfo, true},
			{http.StatusInternalServerError, flam.LogError, true},
			{http.StatusServiceUnavailable, flam.LogError, true},
		} {
			m, logger, clock := newLog(t, container.Bag{})
			testutil.Equal(t, "Id()", m.Id(), "main")

			rec, panicked := serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
				clock.Advance(1500 * time.Millisecond)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("hello"))
			})
			testutil.Equal(t, "panicked", panicked, nil)
			testutil.Equal(t, "status", rec.Code, tc.status)
			testutil.Equal(t, "body", rec.Body.String(), "hello")

			got := logger.all()
			testutil.Equal(t, "count", len(got), 2)
			testutil.Equal(t, "request level", got[0].level, flam.LogInfo)
			testutil.Equal(t, "request channel", got[0].channel, "flam.rest")
			testutil.Equal(t, "request message", got[0].message, "rest request received")
			testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: requested(nil)}))
			testutil.Equal(t, "response level", got[1].level, tc.level)
			testutil.Equal(t, "response channel", got[1].channel, "flam.rest")
			testutil.Equal(t, "response message", got[1].message, "rest response sent")
			response := responded(tc.status, 5, nil)
			if tc.body {
				response[log.ContextFieldBody] = "hello"
			}
			testutil.Equal(t, "response ctx", got[1].ctx, sent(requested(nil), response))
		}
	})

	t.Run("should log the headers and the parameters, redacting the listed ones", func(t *testing.T) {
		m, logger, clock := newLog(t, container.Bag{})
		req := logRequest(http.MethodGet, "/items/1?a=1&b=2&b=3&Token=t&api_key=k1&api_key=k2&apikey=k&access_token=a&password=p&SECRET=s", nil)
		req.Header.Set("Accept", "text/plain")
		req.Header.Set("Authorization", "Bearer secret")
		req.Header["cookie"] = []string{"a=1", "b=2"}
		req.Header.Add("X-Multi", "x")
		req.Header.Add("X-Multi", "y")

		serveLog(m, logEndpoint(0), req, func(w http.ResponseWriter, _ *http.Request) {
			clock.Advance(1500 * time.Millisecond)
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Set-Cookie", "s=1")
			w.Header().Add("Vary", "Accept")
			w.Header().Add("Vary", "Origin")
		})

		request := requested(container.Bag{
			log.ContextFieldHeaders: container.Bag{
				"Accept":        "text/plain",
				"Authorization": "[redacted]",
				"cookie":        "[redacted]",
				"X-Multi":       []string{"x", "y"},
			},
			log.ContextFieldParams: container.Bag{
				"a":            "1",
				"b":            []string{"2", "3"},
				"Token":        "[redacted]",
				"api_key":      "[redacted]",
				"apikey":       "[redacted]",
				"access_token": "[redacted]",
				"password":     "[redacted]",
				"SECRET":       "[redacted]",
			},
		})
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
		testutil.Equal(t, "response ctx", got[1].ctx, sent(request, responded(http.StatusOK, 0, container.Bag{
			log.ContextFieldHeaders: container.Bag{
				"Content-Type": "text/plain",
				"Set-Cookie":   "[redacted]",
				"Vary":         []string{"Accept", "Origin"},
			},
		})))
	})

	t.Run("should log the response body when the status is not the one of the endpoint", func(t *testing.T) {
		for _, tc := range []struct {
			expected int
			status   int
			body     bool
		}{
			{0, http.StatusOK, false},
			{0, http.StatusCreated, true},
			{http.StatusCreated, http.StatusCreated, false},
			{http.StatusCreated, http.StatusOK, true},
		} {
			m, logger, clock := newLog(t, container.Bag{})
			serveLog(m, logEndpoint(tc.expected), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
				clock.Advance(1500 * time.Millisecond)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte("abc"))
			})
			response := responded(tc.status, 3, nil)
			if tc.body {
				response[log.ContextFieldBody] = "abc"
			}
			got := logger.all()
			testutil.Equal(t, "count", len(got), 2)
			testutil.Equal(t, "response ctx", got[1].ctx, sent(requested(nil), response))
		}
	})

	t.Run("should log the request body and leave it whole for the handler", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			content   string
			logged    string
			truncated bool
		}{
			{"shorter", "abc", "abc", false},
			{"as long", "abcd", "abcd", false},
			{"longer", "abcdefghij", "abcd", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, logger, clock := newLog(t, container.Bag{log.ConfigFieldMaxBody: 4})
				body := &trackedBody{r: strings.NewReader(tc.content)}
				var read string
				serveLog(m, logEndpoint(0), logRequest(http.MethodPost, "/items/1", body), func(_ http.ResponseWriter, req *http.Request) {
					clock.Advance(1500 * time.Millisecond)
					b, err := io.ReadAll(req.Body)
					testutil.NoError(t, err)
					read = string(b)
					testutil.NoError(t, req.Body.Close())
				})
				testutil.Equal(t, "read", read, tc.content)
				testutil.Equal(t, "closed", body.closed, true)

				request := requested(container.Bag{log.ContextFieldMethod: http.MethodPost, log.ContextFieldBody: tc.logged})
				if tc.truncated {
					request[log.ContextFieldBodyTruncated] = true
				}
				got := logger.all()
				testutil.Equal(t, "count", len(got), 2)
				testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
			})
		}
	})

	t.Run("should not read the body of a request expecting a 100-continue", func(t *testing.T) {
		for _, expect := range []string{"100-continue", "100-Continue"} {
			m, logger, clock := newLog(t, container.Bag{log.ConfigFieldParseJSON: true})
			body := &trackedBody{r: strings.NewReader(`{"a":1}`)}
			req := logRequest(http.MethodPost, "/items/1", body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Expect", expect)
			var reads int
			var handled io.ReadCloser
			var read string
			serveLog(m, logEndpoint(0), req, func(_ http.ResponseWriter, req *http.Request) {
				clock.Advance(1500 * time.Millisecond)
				reads = body.reads
				handled = req.Body
				b, err := io.ReadAll(req.Body)
				testutil.NoError(t, err)
				read = string(b)
			})
			testutil.Equal(t, "reads before the handler", reads, 0)
			testutil.Equal(t, "handler body", handled, io.ReadCloser(body))
			testutil.Equal(t, "read", read, `{"a":1}`)

			request := requested(container.Bag{
				log.ContextFieldMethod:  http.MethodPost,
				log.ContextFieldHeaders: container.Bag{"Content-Type": "application/json", "Expect": expect},
			})
			delete(request, log.ContextFieldBody)
			got := logger.all()
			testutil.Equal(t, "count", len(got), 2)
			testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
			testutil.Equal(t, "response ctx", got[1].ctx, sent(request, responded(http.StatusOK, 0, nil)))
		}
	})

	t.Run("should read the body of a request expecting something else", func(t *testing.T) {
		m, logger, _ := newLog(t, container.Bag{})
		req := logRequest(http.MethodPost, "/items/1", strings.NewReader("abc"))
		req.Header.Set("Expect", "100-continue-later")
		serveLog(m, logEndpoint(0), req, func(http.ResponseWriter, *http.Request) {})
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "body", got[0].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldBody), any("abc"))
	})

	t.Run("should keep the response body only when the status is not the one of the endpoint", func(t *testing.T) {
		const size = 4 << 20
		data := []byte(strings.Repeat("x", size))
		for _, tc := range []struct {
			name   string
			status int
			kept   bool
		}{
			{"expected", http.StatusOK, false},
			{"not expected", http.StatusBadRequest, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, logger, _ := newLog(t, container.Bag{log.ConfigFieldMaxBody: size})
				handler := m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(tc.status)
					_, _ = w.Write(data[:1])
					_, _ = w.Write(data[1:])
				}))
				req := logRequest(http.MethodGet, "/items/1", nil)
				w := discard{header: http.Header{}}

				var before, after runtime.MemStats
				runtime.GC()
				runtime.ReadMemStats(&before)
				handler.ServeHTTP(w, req)
				runtime.ReadMemStats(&after)

				allocated := after.TotalAlloc - before.TotalAlloc
				testutil.Equal(t, "kept", allocated >= size, tc.kept)
				got := logger.all()
				testutil.Equal(t, "count", len(got), 2)
				testutil.Equal(t, "logged size", got[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldSize), any(size))
				testutil.Equal(t, "logged body", got[1].ctx.Has(log.ContextFieldResponse+"."+log.ContextFieldBody), tc.kept)
			})
		}
	})

	t.Run("should log the bytes read of a failing request body", func(t *testing.T) {
		m, logger, _ := newLog(t, container.Bag{})
		errRead := errors.New("read")
		body := &trackedBody{r: strings.NewReader("abc"), err: errRead}
		var err error
		serveLog(m, logEndpoint(0), logRequest(http.MethodPost, "/items/1", body), func(_ http.ResponseWriter, req *http.Request) {
			_, err = io.ReadAll(req.Body)
		})
		testutil.Equal(t, "handler error", err, errRead)
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "body", got[0].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldBody), any("abc"))
	})

	t.Run("should log an empty body for a request without body", func(t *testing.T) {
		m, logger, _ := newLog(t, container.Bag{})
		req := logRequest(http.MethodGet, "/items/1", nil)
		req.Body = nil
		var body io.ReadCloser = http.NoBody
		serveLog(m, logEndpoint(0), req, func(_ http.ResponseWriter, req *http.Request) { body = req.Body })
		testutil.Equal(t, "handler body", body, io.ReadCloser(nil))

		req = logRequest(http.MethodGet, "/items/1", nil)
		serveLog(m, logEndpoint(0), req, func(_ http.ResponseWriter, req *http.Request) { body = req.Body })
		testutil.Equal(t, "handler body", body, io.ReadCloser(http.NoBody))

		got := logger.all()
		testutil.Equal(t, "count", len(got), 4)
		testutil.Equal(t, "nil body", got[0].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldBody), any(""))
		testutil.Equal(t, "no body", got[2].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldBody), any(""))
	})

	t.Run("should leave the bodies out with a maximum body size of 0", func(t *testing.T) {
		m, logger, clock := newLog(t, container.Bag{log.ConfigFieldMaxBody: 0, log.ConfigFieldParseJSON: true})
		body := &trackedBody{r: strings.NewReader(`{"a":1}`)}
		req := logRequest(http.MethodPost, "/items/1", body)
		req.Header.Set("Content-Type", "application/json")
		var w http.ResponseWriter
		var handled io.ReadCloser
		serveLog(m, logEndpoint(0), req, func(rw http.ResponseWriter, req *http.Request) {
			clock.Advance(1500 * time.Millisecond)
			w = rw
			handled = req.Body
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusBadRequest)
			_, _ = rw.Write([]byte(`{"b":2}`))
		})
		testutil.Equal(t, "reads", body.reads, 0)
		testutil.Equal(t, "handler body", handled, io.ReadCloser(body))
		if _, ok := w.(interface{ Unwrap() http.ResponseWriter }); !ok {
			t.Fatalf("writer = %T, want the response writer of the rest package", w)
		}

		request := requested(container.Bag{
			log.ContextFieldMethod:  http.MethodPost,
			log.ContextFieldHeaders: container.Bag{"Content-Type": "application/json"},
		})
		delete(request, log.ContextFieldBody)
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
		testutil.Equal(t, "response ctx", got[1].ctx, sent(request, responded(http.StatusBadRequest, 7, container.Bag{
			log.ContextFieldHeaders: container.Bag{"Content-Type": "application/json"},
		})))
	})

	t.Run("should truncate the response body logged", func(t *testing.T) {
		for _, tc := range []struct {
			name      string
			parts     []string
			logged    string
			truncated bool
		}{
			{"shorter", []string{"ab", "c"}, "abc", false},
			{"as long", []string{"ab", "cd"}, "abcd", false},
			{"longer in a part", []string{"ab", "cdef"}, "abcd", true},
			{"longer in parts", []string{"abcd", "e", "f"}, "abcd", true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, logger, clock := newLog(t, container.Bag{log.ConfigFieldMaxBody: 4})
				rec, _ := serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
					clock.Advance(1500 * time.Millisecond)
					w.WriteHeader(http.StatusBadRequest)
					for _, part := range tc.parts {
						n, err := w.Write([]byte(part))
						testutil.NoError(t, err)
						testutil.Equal(t, "written", n, len(part))
					}
				})
				written := strings.Join(tc.parts, "")
				testutil.Equal(t, "body", rec.Body.String(), written)

				response := responded(http.StatusBadRequest, len(written), container.Bag{log.ContextFieldBody: tc.logged})
				if tc.truncated {
					response[log.ContextFieldBodyTruncated] = true
				}
				got := logger.all()
				testutil.Equal(t, "count", len(got), 2)
				testutil.Equal(t, "response ctx", got[1].ctx, sent(requested(nil), response))
			})
		}
	})

	t.Run("should decode the JSON bodies when enabled", func(t *testing.T) {
		for _, tc := range []struct {
			name         string
			parse        bool
			maxBody      int
			reqType      string
			reqBody      string
			resType      string
			resBody      string
			reqJSON      any
			resJSON      any
			reqTruncated bool
			resTruncated bool
		}{
			{"disabled", false, 100, "application/json", `{"a":1}`, "application/json", `{"b":2}`, nil, nil, false, false},
			{"json", true, 100, "Application/JSON; charset=utf-8", `{"a":1}`, "application/json", `[1,"x"]`,
				map[string]any{"a": float64(1)}, []any{float64(1), "x"}, false, false},
			{"problem", true, 100, "application/json", `"s"`, "application/problem+json", `{"title":"t"}`,
				"s", map[string]any{"title": "t"}, false, false},
			{"not json types", true, 100, "text/plain", `{"a":1}`, "text/json", `{"b":2}`, nil, nil, false, false},
			{"problem request", true, 100, "application/problem+json", `{"a":1}`, "text/plain", `{"b":2}`, nil, nil, false, false},
			{"invalid", true, 100, "application/json", `{"a":`, "application/json", `{`, nil, nil, false, false},
			{"truncated", true, 4, "application/json", `{"a":1}`, "application/json", `{"b":2}`, nil, nil, true, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, logger, clock := newLog(t, container.Bag{log.ConfigFieldParseJSON: tc.parse, log.ConfigFieldMaxBody: tc.maxBody})
				req := logRequest(http.MethodPost, "/items/1", strings.NewReader(tc.reqBody))
				req.Header.Set("Content-Type", tc.reqType)
				serveLog(m, logEndpoint(0), req, func(w http.ResponseWriter, _ *http.Request) {
					clock.Advance(1500 * time.Millisecond)
					w.Header().Set("Content-Type", tc.resType)
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(tc.resBody))
				})

				request := requested(container.Bag{
					log.ContextFieldMethod:  http.MethodPost,
					log.ContextFieldHeaders: container.Bag{"Content-Type": tc.reqType},
					log.ContextFieldBody:    tc.reqBody[:min(len(tc.reqBody), tc.maxBody)],
				})
				if tc.reqJSON != nil {
					request[log.ContextFieldBodyJSON] = tc.reqJSON
				}
				if tc.reqTruncated {
					request[log.ContextFieldBodyTruncated] = true
				}
				response := responded(http.StatusUnprocessableEntity, len(tc.resBody), container.Bag{
					log.ContextFieldHeaders: container.Bag{"Content-Type": tc.resType},
					log.ContextFieldBody:    tc.resBody[:min(len(tc.resBody), tc.maxBody)],
				})
				if tc.resJSON != nil {
					response[log.ContextFieldBodyJSON] = tc.resJSON
				}
				if tc.resTruncated {
					response[log.ContextFieldBodyTruncated] = true
				}
				got := logger.all()
				testutil.Equal(t, "count", len(got), 2)
				testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
				testutil.Equal(t, "response ctx", got[1].ctx, sent(request, response))
			})
		}
	})

	t.Run("should keep the writer usable by the inner handlers", func(t *testing.T) {
		m, logger, _ := newLog(t, container.Bag{})
		var shared bool
		rec, _ := serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
			rw := rest.NewResponseWriter(w)
			shared = rw == w
			rw.WriteHeader(http.StatusAccepted)
			_, _ = rw.Write([]byte("ab"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
			testutil.Equal(t, "Status()", rw.Status(), http.StatusAccepted)
			testutil.Equal(t, "Size()", rw.Size(), 2)
			testutil.Equal(t, "Written()", rw.Written(), true)
			w.(http.Flusher).Flush()
			unwrapped := w.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
			if _, ok := unwrapped.(flam.IRestResponseWriter); !ok {
				t.Errorf("Unwrap() = %T, want a flam.IRestResponseWriter", unwrapped)
			}
		})
		testutil.Equal(t, "shared", shared, true)
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "body", rec.Body.String(), "ab")
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "logged body", got[1].ctx.Get(log.ContextFieldResponse+"."+log.ContextFieldBody), any("ab"))
	})

	t.Run("should signal a 200 when the handler writes nothing", func(t *testing.T) {
		m, logger, clock := newLog(t, container.Bag{})
		serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(http.ResponseWriter, *http.Request) {
			clock.Advance(1500 * time.Millisecond)
		})
		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		testutil.Equal(t, "level", got[1].level, flam.LogInfo)
		testutil.Equal(t, "ctx", got[1].ctx, sent(requested(nil), responded(http.StatusOK, 0, nil)))
	})

	t.Run("should signal a panicked request and let the panic go on", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			status int
			size   int
			write  bool
		}{
			{"before writing", 0, 0, false},
			{"after writing", http.StatusCreated, 3, true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				m, logger, clock := newLog(t, container.Bag{})
				_, panicked := serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
					clock.Advance(1500 * time.Millisecond)
					if tc.write {
						w.WriteHeader(http.StatusCreated)
						_, _ = w.Write([]byte("abc"))
					}
					panic(errBoom)
				})
				testutil.Equal(t, "panicked", panicked, any(errBoom))

				got := logger.all()
				testutil.Equal(t, "count", len(got), 2)
				testutil.Equal(t, "level", got[1].level, flam.LogError)
				body := "abc"
				if !tc.write {
					body = ""
				}
				want := sent(requested(nil), responded(tc.status, tc.size, container.Bag{log.ContextFieldBody: body})).(container.Bag)
				want[log.ContextFieldPanicked] = true
				testutil.Equal(t, "ctx", got[1].ctx, flam.IBag(want))
			})
		}
	})

	t.Run("should log the id of the request held by its context", func(t *testing.T) {
		m, logger, _ := newLog(t, container.Bag{})
		req := logRequest(http.MethodGet, "/items/1", nil)
		req = req.WithContext(rest.WithRequestId(req.Context(), "req-1"))
		serveLog(m, logEndpoint(0), req, func(http.ResponseWriter, *http.Request) {})

		got := logger.all()
		testutil.Equal(t, "count", len(got), 2)
		request := requested(container.Bag{log.ContextFieldRequestId: "req-1"})
		testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
		testutil.Equal(t, "response request", got[1].ctx.Get(log.ContextFieldRequest), any(request))
		testutil.Equal(t, "field", log.ContextFieldRequestId, "request_id")
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger := &entries{}
		clock := testutil.NewClock(epoch)
		handlers := []http.HandlerFunc{
			func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ab")) },
			func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Secret", "s")
				w.Header().Set("Authorization", "a")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`{"a":1}`))
			},
		}
		m, err := createLog(t, container.Bag{
			log.ConfigFieldChannel:       "api",
			log.ConfigFieldLevelRequest:  "debug",
			log.ConfigFieldParseJSON:     true,
			log.ConfigFieldMaxBody:       3,
			log.ConfigFieldRedactHeaders: []any{"x-secret"},
			log.ConfigFieldRedactParams:  []any{"A"},
		}, map[string]any{
			log.DefaultLevelResponseConfigPath: 2,
			log.DefaultLevelErrorConfigPath:    3,
		}, logger, clock)
		testutil.NoError(t, err)
		for _, h := range handlers {
			serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1?a=1&B=2&token=t", nil), h)
		}

		m, err = createLog(t, container.Bag{
			log.ConfigFieldLevelResponse: "notice",
			log.ConfigFieldLevelError:    "fatal",
		}, map[string]any{
			log.DefaultChannelConfigPath:       "web",
			log.DefaultLevelRequestConfigPath:  "warning",
			log.DefaultParseJSONConfigPath:     true,
			log.DefaultMaxBodyConfigPath:       10,
			log.DefaultRedactHeadersConfigPath: []string{"Content-Type"},
			log.DefaultRedactParamsConfigPath:  []string{"b"},
		}, logger, clock)
		testutil.NoError(t, err)
		for _, h := range handlers {
			serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1?a=1&B=2&token=t", nil), h)
		}

		got := logger.all()
		testutil.Equal(t, "count", len(got), 8)
		var levels []flam.LogLevel
		var channels []string
		for _, e := range got {
			levels = append(levels, e.level)
			channels = append(channels, e.channel)
		}
		testutil.Equal(t, "levels", levels, []flam.LogLevel{
			flam.LogDebug, flam.LogError, flam.LogDebug, flam.LogWarning,
			flam.LogWarning, flam.LogNotice, flam.LogWarning, flam.LogFatal,
		})
		testutil.Equal(t, "channels", channels, []string{"api", "api", "api", "api", "web", "web", "web", "web"})
		testutil.Equal(t, "first params", got[0].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldParams),
			any(container.Bag{"a": "[redacted]", "B": "2", "token": "t"}))
		testutil.Equal(t, "second params", got[4].ctx.Get(log.ContextFieldRequest+"."+log.ContextFieldParams),
			any(container.Bag{"a": "1", "B": "[redacted]", "token": "t"}))
		testutil.Equal(t, "first response", got[3].ctx.Get(log.ContextFieldResponse), any(responded(http.StatusBadGateway, 7, container.Bag{
			log.ContextFieldHeaders:       container.Bag{"X-Secret": "[redacted]", "Authorization": "a", "Content-Type": "application/json"},
			log.ContextFieldBody:          `{"a`,
			log.ContextFieldBodyTruncated: true,
		})))
		testutil.Equal(t, "second response", got[7].ctx.Get(log.ContextFieldResponse), any(responded(http.StatusBadGateway, 7, container.Bag{
			log.ContextFieldHeaders:  container.Bag{"X-Secret": "s", "Authorization": "a", "Content-Type": "[redacted]"},
			log.ContextFieldBody:     `{"a":1}`,
			log.ContextFieldBodyJSON: map[string]any{"a": float64(1)},
		})))
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger := &entries{}
		m, err := create(t, bare{log.NewBundle()}, log.Driver, container.Bag{}, nil,
			deps{logger: logger, clock: testutil.NewClock(epoch)})
		testutil.NoError(t, err)
		long := strings.Repeat("x", 65537)
		req := logRequest(http.MethodPost, "/items/1?apiKey=k&q=1", strings.NewReader(`{"a":1}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Proxy-Authorization", "p")
		serveLog(m, logEndpoint(0), req, func(http.ResponseWriter, *http.Request) {})
		serveLog(m, logEndpoint(0), logRequest(http.MethodGet, "/items/1", nil), func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(long))
		})

		got := logger.all()
		testutil.Equal(t, "count", len(got), 4)
		testutil.Equal(t, "levels", []flam.LogLevel{got[0].level, got[1].level, got[2].level, got[3].level},
			[]flam.LogLevel{flam.LogInfo, flam.LogInfo, flam.LogInfo, flam.LogError})
		testutil.Equal(t, "channel", got[0].channel, "flam.rest")
		testutil.Equal(t, "request", got[0].ctx.Get(log.ContextFieldRequest), any(requested(container.Bag{
			log.ContextFieldMethod:  http.MethodPost,
			log.ContextFieldHeaders: container.Bag{"Content-Type": "application/json", "Proxy-Authorization": "[redacted]"},
			log.ContextFieldParams:  container.Bag{"apiKey": "[redacted]", "q": "1"},
			log.ContextFieldBody:    `{"a":1}`,
		})))
		testutil.Equal(t, "response", got[3].ctx.Get(log.ContextFieldResponse), any(responded(http.StatusBadGateway, 65537, container.Bag{
			log.ContextFieldBody:          long[:65536],
			log.ContextFieldBodyTruncated: true,
		})))
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{log.ConfigFieldChannel, ""},
			{log.ConfigFieldChannel, 1},
			{log.ConfigFieldLevelRequest, "loud"},
			{log.ConfigFieldLevelResponse, []any{}},
			{log.ConfigFieldLevelError, "x"},
			{log.ConfigFieldParseJSON, "true"},
			{log.ConfigFieldMaxBody, -1},
			{log.ConfigFieldMaxBody, 1.5},
			{log.ConfigFieldMaxBody, int64(4)},
			{log.ConfigFieldMaxBody, "4"},
			{log.ConfigFieldRedactHeaders, "Authorization"},
			{log.ConfigFieldRedactHeaders, []any{"Authorization", 1}},
			{log.ConfigFieldRedactParams, "token"},
			{log.ConfigFieldRedactParams, []any{"token", 1}},
		} {
			_, err := createLog(t, container.Bag{tc.field: tc.value}, nil, &entries{}, testutil.NewClock(epoch))
			testutil.Error(t, errors.Unwrap(err), log.ErrInvalidField,
				"unable to create log rest middleware: invalid log rest middleware field",
				map[string]any{log.ErrorFieldID: "main", log.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		for path, field := range map[string]string{
			log.DefaultParseJSONConfigPath:     log.ConfigFieldParseJSON,
			log.DefaultMaxBodyConfigPath:       log.ConfigFieldMaxBody,
			log.DefaultRedactHeadersConfigPath: log.ConfigFieldRedactHeaders,
			log.DefaultRedactParamsConfigPath:  log.ConfigFieldRedactParams,
		} {
			_, err := createLog(t, container.Bag{}, map[string]any{path: "x"}, &entries{}, testutil.NewClock(epoch))
			testutil.Error(t, errors.Unwrap(err), log.ErrInvalidField,
				"unable to create log rest middleware: invalid log rest middleware field",
				map[string]any{log.ErrorFieldID: "main", log.ErrorFieldField: field})
		}
	})

	t.Run("should fail without logger in the container", func(t *testing.T) {
		_, err := createLog(t, container.Bag{}, nil, nil, testutil.NewClock(epoch))
		testutil.Error(t, errors.Unwrap(err), log.ErrMissingLogger,
			"unable to create log rest middleware: logger not provided by the container",
			map[string]any{log.ErrorFieldID: "main"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createLog(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, &entries{}, testutil.NewClock(epoch))
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

// logContextKey is the key of a value of the contexts of the log tests.
type logContextKey struct{}

// logContextEntries is a flam.IContextLogger recording the entries
// signaled with a context, along with the contexts, whose other methods
// panic.
type logContextEntries struct {
	flam.IContextLogger

	recorded entries
	mu       sync.Mutex
	ctxs     []context.Context
}

func (e *logContextEntries) SignalContext(ctx context.Context, level flam.LogLevel, channel, message string, bags ...flam.IBag) {
	e.mu.Lock()
	e.ctxs = append(e.ctxs, ctx)
	e.mu.Unlock()
	e.recorded.Signal(level, channel, message, bags...)
}

// contexts returns a copy of the contexts recorded.
func (e *logContextEntries) contexts() []context.Context {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]context.Context(nil), e.ctxs...)
}

func Test_Log_Context(t *testing.T) {
	t.Run("should signal with the context of the request when the logger takes one", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			panic bool
			level flam.LogLevel
		}{
			{"served", false, flam.LogInfo},
			{"panicked", true, flam.LogError},
		} {
			t.Run(tc.name, func(t *testing.T) {
				logger := &logContextEntries{}
				clock := testutil.NewClock(epoch)
				m, err := createLog(t, container.Bag{}, nil, logger, clock)
				testutil.NoError(t, err)
				req := logRequest(http.MethodGet, "/items/1", nil)
				ctx := rest.WithRequestId(context.WithValue(req.Context(), logContextKey{}, "v"), "req-1")
				req = req.WithContext(ctx)
				_, panicked := serveLog(m, logEndpoint(0), req, func(http.ResponseWriter, *http.Request) {
					clock.Advance(1500 * time.Millisecond)
					if tc.panic {
						panic("boom")
					}
				})
				testutil.Equal(t, "panicked", panicked != nil, tc.panic)

				got := logger.recorded.all()
				testutil.Equal(t, "count", len(got), 2)
				request := requested(container.Bag{log.ContextFieldRequestId: "req-1"})
				testutil.Equal(t, "request level", got[0].level, flam.LogInfo)
				testutil.Equal(t, "request channel", got[0].channel, "flam.rest")
				testutil.Equal(t, "request message", got[0].message, "rest request received")
				testutil.Equal(t, "request ctx", got[0].ctx, flam.IBag(container.Bag{log.ContextFieldRequest: request}))
				testutil.Equal(t, "response level", got[1].level, tc.level)
				testutil.Equal(t, "response message", got[1].message, "rest response sent")
				testutil.Equal(t, "response request", got[1].ctx.Get(log.ContextFieldRequest), any(request))

				contexts := logger.contexts()
				testutil.Equal(t, "contexts", len(contexts), 2)
				for _, c := range contexts {
					testutil.Equal(t, "value", c.Value(logContextKey{}), any("v"))
					id, _ := rest.RequestIdOf(c)
					testutil.Equal(t, "request id", id, "req-1")
				}
			})
		}
	})
}
