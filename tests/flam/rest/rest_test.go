package rest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// middleware is a middleware writing its id to the trace header around
// the handler it wraps, recording the endpoints it wraps, or wrapping into
// nil those of the pattern nilPattern when nilHandler.
type middleware struct {
	id         string
	nilHandler bool
	nilPattern string
	endpoints  []flam.RestEndpoint
}

func (m *middleware) Id() string { return m.id }

func (m *middleware) Wrap(endpoint flam.RestEndpoint, next http.Handler) http.Handler {
	m.endpoints = append(m.endpoints, endpoint)
	if m.nilHandler && endpoint.Pattern == m.nilPattern {
		return nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Add("Trace", m.id)
		next.ServeHTTP(w, req)
	})
}

// handler is a handler writing the endpoint of its request context.
func handler(w http.ResponseWriter, req *http.Request) {
	endpoint, ok := rest.EndpointOf(req.Context())
	if !ok {
		http.Error(w, "no endpoint", http.StatusInternalServerError)
		return
	}
	_, _ = io.WriteString(w, endpoint.Pattern)
}

// serve serves the request of the method and target with the handler,
// returning the response.
func serve(h http.Handler, method, target string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, target, body))
	return w
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"ServerConfigPath":             {rest.ServerConfigPath, "flam.rest.servers"},
		"ServerCreatorGroup":           {rest.ServerCreatorGroup, "flam.rest.servers.creator"},
		"LoggerConfigPath":             {rest.LoggerConfigPath, "flam.rest.loggers"},
		"LoggerCreatorGroup":           {rest.LoggerCreatorGroup, "flam.rest.loggers.creator"},
		"MiddlewareConfigPath":         {rest.MiddlewareConfigPath, "flam.rest.middlewares"},
		"MiddlewareCreatorGroup":       {rest.MiddlewareCreatorGroup, "flam.rest.middlewares.creator"},
		"EndpointGroup":                {rest.EndpointGroup, "flam.rest.endpoints"},
		"ServiceIdConfigPath":          {rest.ServiceIdConfigPath, "flam.rest.service_id"},
		"DefaultLoggerConfigPath":      {rest.DefaultLoggerConfigPath, "flam.rest.defaults.servers.logger"},
		"DefaultMiddlewaresConfigPath": {rest.DefaultMiddlewaresConfigPath, "flam.rest.defaults.servers.middlewares"},
		"ErrorFieldID":                 {rest.ErrorFieldID, "id"},
		"ErrorFieldEndpoint":           {rest.ErrorFieldEndpoint, "endpoint"},
		"ErrorFieldPattern":            {rest.ErrorFieldPattern, "pattern"},
		"ErrorFieldField":              {rest.ErrorFieldField, "field"},
		"ErrorFieldSource":             {rest.ErrorFieldSource, "source"},
		"SourcePath":                   {rest.SourcePath, "path"},
		"SourceQuery":                  {rest.SourceQuery, "query"},
		"SourceBody":                   {rest.SourceBody, "body"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrNilMiddleware":    rest.ErrNilMiddleware,
		"ErrInvalidEndpoint":  rest.ErrInvalidEndpoint,
		"ErrMiddleware":       rest.ErrMiddleware,
		"ErrNotFound":         rest.ErrNotFound,
		"ErrMethodNotAllowed": rest.ErrMethodNotAllowed,
		"ErrServer":           rest.ErrServer,
		"ErrDecode":           rest.ErrDecode,
		"ErrEncode":           rest.ErrEncode,
		"ErrWrite":            rest.ErrWrite,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_NewRouter(t *testing.T) {
	t.Run("should fail with a nil middleware", func(t *testing.T) {
		var nilMiddleware *middleware
		router, err := rest.NewRouter(1, &middleware{id: "a"}, nilMiddleware)
		testutil.Equal(t, "router", router, (*rest.Router)(nil))
		testutil.Error(t, err, rest.ErrNilMiddleware, "unable to create rest router: nil rest middleware", map[string]any{})
	})

	t.Run("should fail when a middleware returns no handler for the unmatched requests", func(t *testing.T) {
		outer, inner := &middleware{id: "outer"}, &middleware{id: "inner", nilHandler: true}
		router, err := rest.NewRouter(7, outer, inner)
		testutil.Equal(t, "router", router, (*rest.Router)(nil))
		testutil.Error(t, err, rest.ErrMiddleware, "unable to create rest router: rest middleware returned no handler",
			map[string]any{rest.ErrorFieldEndpoint: 0, rest.ErrorFieldPattern: "", rest.ErrorFieldID: "inner"})
		testutil.Equal(t, "outer not called", len(outer.endpoints), 0)
	})

	t.Run("should create a router with no middleware", func(t *testing.T) {
		router, err := rest.NewRouter(0)
		testutil.NoError(t, err)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Pattern: "/a"}, http.HandlerFunc(handler)))
		w := serve(router, http.MethodGet, "/a", nil)
		testutil.Equal(t, "body", w.Body.String(), "/a")
	})
}

func Test_Router_Handle(t *testing.T) {
	ok := http.HandlerFunc(handler)
	for name, tc := range map[string]struct {
		endpoint flam.RestEndpoint
		handler  http.Handler
		field    string
	}{
		"no pattern":  {flam.RestEndpoint{Id: 1}, ok, "pattern"},
		"low status":  {flam.RestEndpoint{Id: 1, Pattern: "/a", Status: 199}, ok, "status"},
		"high status": {flam.RestEndpoint{Id: 1, Pattern: "/a", Status: 600}, ok, "status"},
		"nil handler": {flam.RestEndpoint{Id: 1, Pattern: "/a"}, nil, "handler"},
		"nil func":    {flam.RestEndpoint{Id: 1, Pattern: "/a"}, http.HandlerFunc(nil), "handler"},
	} {
		t.Run("should fail with "+name, func(t *testing.T) {
			router, _ := rest.NewRouter(7)
			err := router.Handle(tc.endpoint, tc.handler)
			if !errors.Is(err, rest.ErrInvalidEndpoint) {
				t.Fatalf("Handle() = %v, want ErrInvalidEndpoint", err)
			}
			var e *flam.Error
			errors.As(err, &e)
			testutil.Context(t, e, map[string]any{
				rest.ErrorFieldEndpoint: 1,
				rest.ErrorFieldPattern:  tc.endpoint.Pattern,
				rest.ErrorFieldField:    tc.field,
			})
		})
	}

	t.Run("should fail with an invalid pattern, with the error of the mux", func(t *testing.T) {
		router, _ := rest.NewRouter(7)
		err := router.Handle(flam.RestEndpoint{Id: 2, Pattern: "/a/{"}, ok)
		if !errors.Is(err, rest.ErrInvalidEndpoint) || !strings.HasPrefix(err.Error(), "unable to handle rest endpoint: invalid rest endpoint: parsing \"/a/{\"") {
			t.Fatalf("Handle() = %v, want the error of the pattern", err)
		}
	})

	t.Run("should fail with a conflicting pattern", func(t *testing.T) {
		router, _ := rest.NewRouter(7)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 1, Pattern: "GET /a"}, ok))
		err := router.Handle(flam.RestEndpoint{Id: 2, Pattern: "GET /a"}, ok)
		if !errors.Is(err, rest.ErrInvalidEndpoint) || !strings.Contains(err.Error(), "conflicts with pattern") {
			t.Fatalf("Handle() = %v, want the conflict", err)
		}
		w := serve(router, http.MethodGet, "/a", nil)
		testutil.Equal(t, "first kept", w.Body.String(), "GET /a")
	})

	t.Run("should fail when a middleware returns no handler", func(t *testing.T) {
		outer, inner := &middleware{id: "outer"}, &middleware{id: "inner", nilHandler: true, nilPattern: "/a"}
		router, _ := rest.NewRouter(7, outer, inner)
		testutil.Error(t, router.Handle(flam.RestEndpoint{Id: 3, Pattern: "/a"}, ok), rest.ErrMiddleware,
			"unable to handle rest endpoint: rest middleware returned no handler",
			map[string]any{rest.ErrorFieldEndpoint: 3, rest.ErrorFieldPattern: "/a", rest.ErrorFieldID: "inner"})
		testutil.Equal(t, "outer not called", len(outer.endpoints), 1)
		w := serve(router, http.MethodGet, "/a", nil)
		testutil.Equal(t, "not registered", w.Code, http.StatusNotFound)
	})

	t.Run("should wrap the handler in the middlewares, the first outermost", func(t *testing.T) {
		outer, inner := &middleware{id: "outer"}, &middleware{id: "inner"}
		router, _ := rest.NewRouter(7, outer, inner)
		endpoint := flam.RestEndpoint{ServiceId: 99, Id: 4, Pattern: "POST /users/{id}", Status: http.StatusCreated}
		testutil.NoError(t, router.Handle(endpoint, ok))

		endpoint.ServiceId = 7
		unmatched := flam.RestEndpoint{ServiceId: 7}
		testutil.Equal(t, "outer endpoints", outer.endpoints, []flam.RestEndpoint{unmatched, endpoint})
		testutil.Equal(t, "inner endpoints", inner.endpoints, []flam.RestEndpoint{unmatched, endpoint})

		w := serve(router, http.MethodPost, "/users/12", nil)
		testutil.Equal(t, "status", w.Code, http.StatusOK)
		testutil.Equal(t, "body", w.Body.String(), "POST /users/{id}")
		testutil.Equal(t, "trace", w.Header().Values("Trace"), []string{"outer", "inner"})

	})

	t.Run("should give the endpoint to the middlewares in the request context", func(t *testing.T) {
		var got flam.RestEndpoint
		router, _ := rest.NewRouter(7, &contextMiddleware{endpoint: &got})
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 5, Pattern: "/a"}, ok))
		serve(router, http.MethodGet, "/a", nil)
		testutil.Equal(t, "endpoint", got, flam.RestEndpoint{ServiceId: 7, Id: 5, Pattern: "/a"})
	})
}

// contextMiddleware is a middleware storing the endpoint of the context of
// the requests.
type contextMiddleware struct{ endpoint *flam.RestEndpoint }

func (m *contextMiddleware) Id() string { return "context" }

func (m *contextMiddleware) Wrap(_ flam.RestEndpoint, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		*m.endpoint, _ = rest.EndpointOf(req.Context())
		next.ServeHTTP(w, req)
	})
}

// otherKey is a context key other than the one of the endpoint.
type otherKey struct{}

func Test_Router_Unmatched(t *testing.T) {
	newRouter := func(t *testing.T) *rest.Router {
		t.Helper()
		router, _ := rest.NewRouter(7, &middleware{id: "outer"})
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 1, Pattern: "GET /users/{id}"}, http.HandlerFunc(handler)))
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 2, Pattern: "GET /items/"}, http.HandlerFunc(handler)))
		return router
	}

	t.Run("should answer a path matching no pattern with an envelope error", func(t *testing.T) {
		w := serve(newRouter(t), http.MethodGet, "/other", nil)
		testutil.Equal(t, "status", w.Code, http.StatusNotFound)
		testutil.Equal(t, "headers", w.Header(), http.Header{"Content-Type": {"application/problem+json"}, "Trace": {"outer"}})
		testutil.Equal(t, "body", w.Body.String(),
			`{"detail":"rest route not found","id":"s:7","status":404,"title":"Not Found","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404"}`)
	})

	t.Run("should answer a method matching no pattern with an envelope error", func(t *testing.T) {
		w := serve(newRouter(t), http.MethodPost, "/users/3", nil)
		testutil.Equal(t, "status", w.Code, http.StatusMethodNotAllowed)
		testutil.Equal(t, "headers", w.Header(), http.Header{
			"Allow":        {"GET, HEAD"},
			"Content-Type": {"application/problem+json"},
			"Trace":        {"outer"},
		})
		testutil.Equal(t, "body", w.Body.String(),
			`{"detail":"rest method not allowed","id":"s:7","status":405,"title":"Method Not Allowed","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/405"}`)
	})

	t.Run("should leave the redirections of the mux", func(t *testing.T) {
		w := serve(newRouter(t), http.MethodGet, "/items", nil)
		testutil.Equal(t, "status", w.Code, http.StatusTemporaryRedirect)
		testutil.Equal(t, "location", w.Header().Get("Location"), "/items/")
		testutil.Equal(t, "no middleware", len(w.Header().Values("Trace")), 0)
	})
}

func Test_EndpointOf(t *testing.T) {
	var nilCtx context.Context
	for name, ctx := range map[string]context.Context{
		"nil":       nilCtx,
		"no value":  context.Background(),
		"other key": context.WithValue(context.Background(), otherKey{}, "x"),
	} {
		endpoint, ok := rest.EndpointOf(ctx)
		testutil.Equal(t, name+" endpoint", endpoint, flam.RestEndpoint{})
		testutil.Equal(t, name+" ok", ok, false)
	}
}

// params are the decoded parameters of a request.
type params struct {
	Id   int    `schema:"id"`
	Path string `schema:"path"`
	Name string `schema:"name"`
}

// decoded registers the handler decoding the requests of the pattern with
// decode, writing the decoded parameters or the error.
func decoded(t *testing.T, pattern string, decode func(req *http.Request, p *params) error) http.Handler {
	t.Helper()
	router, _ := rest.NewRouter(0)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Pattern: pattern}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		p := params{}
		if err := decode(req, &p); err != nil {
			_, _ = io.WriteString(w, err.Error())
			return
		}
		_, _ = fmt.Fprintf(w, "%d|%s|%s", p.Id, p.Path, p.Name)
	})))
	return router
}

func Test_DecodePath(t *testing.T) {
	decoder := rest.NewDecoder()
	decode := func(req *http.Request, p *params) error { return rest.DecodePath(decoder, req, p) }

	t.Run("should decode the path values", func(t *testing.T) {
		h := decoded(t, "GET /users/{id}/{path...}", decode)
		testutil.Equal(t, "body", serve(h, http.MethodGet, "/users/7/a/b?name=x", nil).Body.String(), "7|a/b|")
	})

	t.Run("should skip the end anchor", func(t *testing.T) {
		h := decoded(t, "GET /users/{id}/{$}", decode)
		testutil.Equal(t, "body", serve(h, http.MethodGet, "/users/3/", nil).Body.String(), "3||")
	})

	t.Run("should decode nothing without a pattern", func(t *testing.T) {
		p := params{}
		testutil.NoError(t, rest.DecodePath(decoder, httptest.NewRequest(http.MethodGet, "/users/7", nil), &p))
		testutil.Equal(t, "params", p, params{})
	})

	t.Run("should fail with an invalid value", func(t *testing.T) {
		h := decoded(t, "GET /users/{id}", decode)
		body := serve(h, http.MethodGet, "/users/x", nil).Body.String()
		if !strings.HasPrefix(body, "unable to decode rest request: invalid rest request: schema: error converting value for \"id\"") {
			t.Errorf("body = %q, want the error of the id", body)
		}
		req := httptest.NewRequest(http.MethodGet, "/users/x", nil)
		req.Pattern = "GET /users/{id}"
		req.SetPathValue("id", "x")
		err := rest.DecodePath(decoder, req, &params{})
		var e *flam.Error
		if !errors.Is(err, rest.ErrDecode) || !errors.As(err, &e) {
			t.Fatalf("DecodePath() = %v, want ErrDecode", err)
		}
		testutil.Context(t, e, map[string]any{rest.ErrorFieldSource: rest.SourcePath})
	})
}

func Test_DecodeQuery(t *testing.T) {
	decoder := rest.NewDecoder()

	t.Run("should decode the query, ignoring the unknown keys", func(t *testing.T) {
		p := params{}
		testutil.NoError(t, rest.DecodeQuery(decoder, httptest.NewRequest(http.MethodGet, "/?id=4&name=x&other=y", nil), &p))
		testutil.Equal(t, "params", p, params{Id: 4, Name: "x"})
	})

	t.Run("should fail with an invalid value", func(t *testing.T) {
		err := rest.DecodeQuery(decoder, httptest.NewRequest(http.MethodGet, "/?id=x", nil), &params{})
		var e *flam.Error
		if !errors.Is(err, rest.ErrDecode) || !errors.As(err, &e) {
			t.Fatalf("DecodeQuery() = %v, want ErrDecode", err)
		}
		testutil.Context(t, e, map[string]any{rest.ErrorFieldSource: rest.SourceQuery})
	})
}

func Test_DecodeJSON(t *testing.T) {
	t.Run("should decode the body", func(t *testing.T) {
		p := params{}
		testutil.NoError(t, rest.DecodeJSON(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Id": 3, "Name": "x"} `)), &p))
		testutil.Equal(t, "params", p, params{Id: 3, Name: "x"})
	})

	for name, tc := range map[string]struct {
		req  *http.Request
		text string
	}{
		"nil body":   {&http.Request{}, "unable to decode rest request: invalid rest request: EOF"},
		"empty body": {httptest.NewRequest(http.MethodPost, "/", nil), "unable to decode rest request: invalid rest request: EOF"},
		"invalid": {httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Id": "x"}`)),
			"unable to decode rest request: invalid rest request: json: cannot unmarshal string into Go struct field params.Id of type int"},
		"trailing data": {httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{} {}`)),
			"unable to decode rest request: invalid rest request: data after the JSON value"},
	} {
		t.Run("should fail with "+name, func(t *testing.T) {
			testutil.Error(t, rest.DecodeJSON(tc.req, &params{}), rest.ErrDecode, tc.text,
				map[string]any{rest.ErrorFieldSource: rest.SourceBody})
		})
	}
}

// failingWriter is a response writer failing to write.
type failingWriter struct{ *httptest.ResponseRecorder }

func (w failingWriter) Write([]byte) (int, error) { return 0, errBoom }

func Test_WriteEnvelope(t *testing.T) {
	// request returns a request holding the endpoint, served by a router
	request := func(t *testing.T, endpoint flam.RestEndpoint, write func(w http.ResponseWriter, req *http.Request)) *httptest.ResponseRecorder {
		t.Helper()
		router, _ := rest.NewRouter(7)
		endpoint.Pattern = "/a"
		testutil.NoError(t, router.Handle(endpoint, http.HandlerFunc(write)))
		return serve(router, http.MethodGet, "/a", nil)
	}

	for name, tc := range map[string]struct {
		endpoint    flam.RestEndpoint
		env         envelope.Envelope
		status      int
		contentType string
		body        string
	}{
		"data": {
			flam.RestEndpoint{Id: 2}, envelope.NewEnvelope("x"),
			http.StatusOK, "application/json", `{"data":"x"}`,
		},
		"endpoint status": {
			flam.RestEndpoint{Id: 2, Status: http.StatusCreated}, envelope.NewEnvelope("x"),
			http.StatusCreated, "application/json", `{"data":"x"}`,
		},
		"no body": {
			flam.RestEndpoint{Id: 2, Status: http.StatusNoContent}, envelope.NewEnvelope(),
			http.StatusNoContent, "application/json", ``,
		},
		"error": {
			flam.RestEndpoint{Id: 2, Status: http.StatusCreated},
			envelope.NewEnvelope().WithNewError(http.StatusConflict, errBoom, map[string]any{"field": "name"}),
			http.StatusConflict, "application/problem+json",
			`{"detail":"boom","field":"name","id":"s:7.e:2","status":409,"title":"Conflict","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/409"}`,
		},
		"invalid error status": {
			flam.RestEndpoint{Id: 2}, envelope.NewEnvelope().WithNewError(42, errBoom),
			http.StatusInternalServerError, "application/problem+json",
			`{"detail":"boom","id":"s:7.e:2","status":500,"title":"Internal Server Error","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500"}`,
		},
	} {
		t.Run("should write "+name, func(t *testing.T) {
			w := request(t, tc.endpoint, func(w http.ResponseWriter, req *http.Request) {
				testutil.NoError(t, rest.WriteEnvelope(w, req, tc.env))
			})
			testutil.Equal(t, "status", w.Code, tc.status)
			testutil.Equal(t, "content type", w.Header().Get("Content-Type"), tc.contentType)
			testutil.Equal(t, "body", w.Body.String(), tc.body)
		})
	}

	for _, status := range []int{http.StatusContinue, http.StatusOK, http.StatusNotModified, 600} {
		t.Run(fmt.Sprintf("should write an error of the status %d as a 500", status), func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			testutil.NoError(t, rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(status, errBoom)))
			testutil.Equal(t, "status", w.Code, http.StatusInternalServerError)
			testutil.Equal(t, "body", w.Body.String(),
				`{"detail":"boom","status":500,"title":"Internal Server Error","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/500"}`)
		})
	}

	t.Run("should write no body with the status not modified", func(t *testing.T) {
		w := request(t, flam.RestEndpoint{Status: http.StatusNotModified}, func(w http.ResponseWriter, req *http.Request) {
			testutil.NoError(t, rest.WriteEnvelope(w, req, envelope.NewEnvelope("x")))
		})
		testutil.Equal(t, "status", w.Code, http.StatusNotModified)
		testutil.Equal(t, "body", w.Body.String(), "")
	})

	t.Run("should write without endpoint", func(t *testing.T) {
		w := httptest.NewRecorder()
		testutil.NoError(t, rest.WriteEnvelope(w, httptest.NewRequest(http.MethodGet, "/", nil),
			envelope.NewEnvelope().WithNewError(http.StatusNotFound, errBoom)))
		testutil.Equal(t, "status", w.Code, http.StatusNotFound)
		testutil.Equal(t, "body", w.Body.String(),
			`{"detail":"boom","status":404,"title":"Not Found","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/404"}`)
	})

	t.Run("should fail to encode, writing nothing", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := rest.WriteEnvelope(w, httptest.NewRequest(http.MethodGet, "/", nil), envelope.NewEnvelope(make(chan int)))
		testutil.Error(t, err, rest.ErrEncode,
			"unable to write rest envelope: unable to encode rest response: json: unsupported type: chan int", map[string]any{})
		testutil.Equal(t, "written", w.Header().Get("Content-Type"), "")
	})

	t.Run("should fail to write", func(t *testing.T) {
		w := failingWriter{httptest.NewRecorder()}
		err := rest.WriteEnvelope(w, httptest.NewRequest(http.MethodGet, "/", nil), envelope.NewEnvelope("x"))
		testutil.Error(t, err, rest.ErrWrite, "unable to write rest envelope: unable to write rest response: boom", map[string]any{})
	})
}

// codesWriter is a response writer recording the statuses written, and
// writing the body whatever they are.
type codesWriter struct {
	*httptest.ResponseRecorder

	codes []int
}

func (w *codesWriter) WriteHeader(status int) { w.codes = append(w.codes, status) }

func Test_NewResponseWriter(t *testing.T) {
	t.Run("should not wrap a response writer twice", func(t *testing.T) {
		w := rest.NewResponseWriter(httptest.NewRecorder())
		if rest.NewResponseWriter(w) != w {
			t.Error("NewResponseWriter() wrapped the writer again")
		}
	})

	t.Run("should answer 200 when nothing is written", func(t *testing.T) {
		w := rest.NewResponseWriter(httptest.NewRecorder())
		testutil.Equal(t, "Status()", w.Status(), http.StatusOK)
		testutil.Equal(t, "Size()", w.Size(), 0)
		testutil.Equal(t, "Written()", w.Written(), false)
	})

	t.Run("should record the status, not an informational one", func(t *testing.T) {
		rec := &codesWriter{ResponseRecorder: httptest.NewRecorder()}
		w := rest.NewResponseWriter(rec)
		w.WriteHeader(http.StatusEarlyHints)
		testutil.Equal(t, "informational", w.Written(), false)
		w.WriteHeader(http.StatusTeapot)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("abc"))
		testutil.NoError(t, err)
		_, err = w.Write([]byte("de"))
		testutil.NoError(t, err)
		testutil.Equal(t, "Status()", w.Status(), http.StatusTeapot)
		testutil.Equal(t, "Size()", w.Size(), 5)
		testutil.Equal(t, "Written()", w.Written(), true)
		testutil.Equal(t, "body", rec.Body.String(), "abc"+"de")
		testutil.Equal(t, "codes", rec.codes, []int{http.StatusEarlyHints, http.StatusTeapot, http.StatusOK})
	})

	t.Run("should record 200 on a write", func(t *testing.T) {
		w := rest.NewResponseWriter(failingWriter{httptest.NewRecorder()})
		_, err := w.Write([]byte("abc"))
		testutil.Equal(t, "error", err, errBoom)
		testutil.Equal(t, "Status()", w.Status(), http.StatusOK)
		testutil.Equal(t, "Size()", w.Size(), 0)
		testutil.Equal(t, "Written()", w.Written(), true)
	})

	t.Run("should flush, recording 200", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := rest.NewResponseWriter(rec)
		w.(http.Flusher).Flush()
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "Written()", w.Written(), true)
		w.WriteHeader(http.StatusTeapot)
		w.(http.Flusher).Flush()
		testutil.Equal(t, "Status()", w.Status(), http.StatusOK)
	})

	t.Run("should unwrap the writer it wraps", func(t *testing.T) {
		rec := httptest.NewRecorder()
		w := rest.NewResponseWriter(rec)
		testutil.Equal(t, "Unwrap()", w.(interface{ Unwrap() http.ResponseWriter }).Unwrap(), http.ResponseWriter(rec))
	})

	t.Run("should flush nothing through a writer unable to", func(t *testing.T) {
		w := rest.NewResponseWriter(struct{ http.ResponseWriter }{httptest.NewRecorder()})
		testutil.NotPanics(t, "Flush()", w.(http.Flusher).Flush)
	})
}

// server is a REST server of a fake creator.
type server struct {
	id     string
	closes int
	err    error
}

func (s *server) Id() string                { return s.id }
func (s *server) Run(context.Context) error { return nil }
func (s *server) IsRunning() bool           { return false }
func (s *server) Address() string           { return "" }
func (s *server) Close() error              { s.closes++; return s.err }

// serverCreator creates the servers of the "fake" driver, failing for the
// id "bad".
type serverCreator struct{}

func (serverCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (serverCreator) Create(bag flam.IBag) (flam.IRestServer, error) {
	id := bag.String(factory.ConfigFieldId)
	if id == "bad" {
		return nil, errBoom
	}
	return &server{id: id}, nil
}

// newApp returns an app with the config and rest bundles, the fake server
// creator, and the configuration holding the servers of the ids.
func newApp(t *testing.T, ids ...string) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(rest.NewBundle()))
	testutil.NoError(t, a.Provide(func() flam.IRestServerCreator { return serverCreator{} }, dig.Group(rest.ServerCreatorGroup)))
	bag := container.Bag{}
	for _, id := range ids {
		testutil.NoError(t, bag.Set(rest.ServerConfigPath+"."+id, container.Bag{factory.ConfigFieldDriver: "fake"}))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// processesArgs are the processes of the kennel.
type processesArgs struct {
	dig.In

	Processes []flam.IWatchdogProcess `group:"flam.watchdog.processes"`
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the factories and the decoder", func(t *testing.T) {
		b := rest.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(b))
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-6:],
			[]string{"bundle:flam.rest", "provider/0", "provider/0", "provider/0", "provider/0", "provider/1"})
		testutil.NoError(t, a.Invoke(func(
			servers flam.IRestServerFactory,
			loggers flam.IRestLoggerAdaptorFactory,
			middlewares flam.IRestMiddlewareFactory,
			args processesArgs,
		) {
			testutil.Equal(t, "servers", servers.Available(), []string{})
			testutil.Equal(t, "loggers", loggers.Available(), []string{})
			testutil.Equal(t, "middlewares", middlewares.Available(), []string{})
			testutil.Equal(t, "processes", len(args.Processes), 0)
		}))
	})

	t.Run("should provide the servers as processes", func(t *testing.T) {
		a := newApp(t, "web", "api")
		testutil.NoError(t, a.Invoke(func(args processesArgs, servers flam.IRestServerFactory) {
			ids := map[string]flam.IWatchdogProcess{}
			for _, p := range args.Processes {
				ids[p.Id()] = p
			}
			testutil.Equal(t, "processes", len(ids), 2)
			for _, id := range []string{"api", "web"} {
				stored, _ := servers.Lookup(id)
				testutil.Equal(t, id+" stored", ids[id], flam.IWatchdogProcess(stored))
			}
		}))
	})

	t.Run("should fail to provide a server failing to be created", func(t *testing.T) {
		a := newApp(t, "api", "bad")
		err := a.Invoke(func(processesArgs) {})
		var e *flam.Error
		if !errors.Is(err, rest.ErrServer) || !errors.Is(err, errBoom) || !errors.As(err, &e) {
			t.Fatalf("Invoke() = %v, want ErrServer and errBoom", err)
		}
		testutil.Equal(t, "id", e.Context()[rest.ErrorFieldID], "bad")
	})

	t.Run("should close the factories", func(t *testing.T) {
		a := newApp(t, "api")
		var api flam.IRestServer
		testutil.NoError(t, a.Invoke(func(servers flam.IRestServerFactory, middlewares flam.IRestMiddlewareFactory) {
			api, _ = servers.Get("api")
			api.(*server).err = errBoom
			testutil.NoError(t, middlewares.Store("m", &closingMiddleware{}))
		}))

		err := rest.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errBoom) || !errors.Is(err, errMiddleware) {
			t.Fatalf("Close() = %v, want the errors of the server and of the middleware", err)
		}
		testutil.Equal(t, "server closes", api.(*server).closes, 1)
		testutil.NoError(t, rest.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(rest.NewBundle()))
		if err := rest.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})

	t.Run("should add the servers to the kennel", func(t *testing.T) {
		a := newApp(t, "api")
		testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
		testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
		testutil.NoError(t, a.Invoke(func(k flam.IKennel) {
			testutil.Equal(t, "Available()", k.Available(), []string{"api"})
		}))
	})
}

var errMiddleware = errors.New("middleware")

// closingMiddleware is a middleware failing to close.
type closingMiddleware struct{ middleware }

func (m *closingMiddleware) Id() string   { return "m" }
func (m *closingMiddleware) Close() error { return errMiddleware }
