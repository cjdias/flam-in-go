package middlewares

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/etag"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createEtag returns the etag middleware "main" of the fields, created
// with the etag bundle and the values.
func createEtag(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, etag.NewBundle(), etag.Driver, fields, values, deps{})
}

// newEtag returns the etag middleware of the fields, failing the test when
// it cannot be created.
func newEtag(t *testing.T, fields container.Bag) flam.IRestMiddleware {
	t.Helper()
	m, err := createEtag(t, fields, nil)
	testutil.NoError(t, err)
	return m
}

// tagOf returns the weak entity tag the middleware computes by default
// for the body.
func tagOf(body string) string {
	return "W/" + strongTagOf(body)
}

// strongTagOf returns the strong entity tag the middleware computes for
// the body when strong.
func strongTagOf(body string) string {
	sum := sha256.Sum256([]byte(body))
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// serveEtag serves a request of the method and the If-None-Match headers
// through the middleware into w, with the handler.
func serveEtag(m flam.IRestMiddleware, w http.ResponseWriter, method string, handler http.HandlerFunc, ifNoneMatch ...string) {
	req := httptest.NewRequest(method, "/items/1", nil)
	for _, value := range ifNoneMatch {
		req.Header.Add("If-None-Match", value)
	}
	m.Wrap(logEndpoint(0), handler).ServeHTTP(w, req)
}

// writeText returns the handler answering with the body as text, along
// with the header, in parts of the sizes, the rest of the body last.
func writeText(header http.Header, body string, parts ...int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		for key, values := range header {
			w.Header()[key] = values
		}
		w.Header().Set("Content-Type", "text/plain")
		rest := body
		for _, size := range parts {
			_, _ = w.Write([]byte(rest[:size]))
			rest = rest[size:]
		}
		_, _ = w.Write([]byte(rest))
	}
}

func Test_Etag_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := etag.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.etag")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + etag.Id: container.Bag{factory.ConfigFieldDriver: etag.Driver},
			etag.DefaultMaxSizeConfigPath:             1048576,
			etag.DefaultStrongConfigPath:              false,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			etag.Driver:                   "flam.rest.middlewares.driver.etag",
			etag.DefaultMaxSizeConfigPath: "flam.rest.defaults.middlewares.etag.max_size",
			etag.DefaultStrongConfigPath:  "flam.rest.defaults.middlewares.etag.strong",
			etag.ConfigFieldMaxSize:       "max_size",
			etag.ConfigFieldStrong:        "strong",
		} {
			testutil.Equal(t, "const", got, want)
		}
		testutil.Equal(t, "Id", etag.Id, "etag")
		testutil.Equal(t, "TagLength", etag.TagLength, 16)
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(etag.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.etag", "provider/1"})
	})
}

func Test_Etag(t *testing.T) {
	t.Run("should tag the 200 response to a GET request", func(t *testing.T) {
		m := newEtag(t, container.Bag{})
		testutil.Equal(t, "Id()", m.Id(), "main")

		rec := httptest.NewRecorder()
		serveEtag(m, rec, http.MethodGet, writeText(nil, "hello world", 5))
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Content-Type": {"text/plain"},
			"Etag":         {tagOf("hello world")},
		})
		testutil.Equal(t, "body", rec.Body.String(), "hello world")
		testutil.Equal(t, "tag", len(tagOf("")), 36)
	})

	t.Run("should answer the matching conditional requests with a 304", func(t *testing.T) {
		tag, strong := tagOf("hello"), strongTagOf("hello")
		for name, tc := range map[string]struct {
			method      string
			ifNoneMatch []string
			status      int
		}{
			"strong":           {http.MethodGet, []string{strong}, http.StatusNotModified},
			"weak":             {http.MethodGet, []string{"W/" + strong}, http.StatusNotModified},
			"listed":           {http.MethodGet, []string{`"a", W/"b",` + strong}, http.StatusNotModified},
			"second header":    {http.MethodGet, []string{`"a"`, " " + strong}, http.StatusNotModified},
			"any":              {http.MethodGet, []string{"*"}, http.StatusNotModified},
			"HEAD":             {http.MethodHead, []string{strong}, http.StatusNotModified},
			"other":            {http.MethodGet, []string{`"a", W/"b"`}, http.StatusOK},
			"empty":            {http.MethodGet, []string{" , "}, http.StatusOK},
			"unquoted":         {http.MethodGet, []string{"abc, " + strong}, http.StatusOK},
			"weak unquoted":    {http.MethodGet, []string{"W/abc, " + strong}, http.StatusOK},
			"weak alone":       {http.MethodGet, []string{"W/"}, http.StatusOK},
			"unterminated":     {http.MethodGet, []string{`"abc`}, http.StatusOK},
			"no header":        {http.MethodGet, nil, http.StatusOK},
			"prefix of tag":    {http.MethodGet, []string{strong[:10] + `"`}, http.StatusOK},
			"tag without list": {http.MethodGet, []string{strong + strong}, http.StatusNotModified},
		} {
			rec := httptest.NewRecorder()
			var status int
			serveEtag(newEtag(t, container.Bag{}), rec, tc.method, func(w http.ResponseWriter, req *http.Request) {
				writeText(http.Header{"Content-Length": {"5"}, "Content-Encoding": {"identity"}, "Last-Modified": {"x"}, "Cache-Control": {"no-cache"}}, "hello")(w, req)
				status = w.(flam.IRestResponseWriter).Status()
			}, tc.ifNoneMatch...)
			testutil.Equal(t, name+" status", rec.Code, tc.status)
			testutil.Equal(t, name+" recorded status", status, http.StatusOK)
			if tc.status == http.StatusOK {
				testutil.Equal(t, name+" body", rec.Body.String(), "hello")
				testutil.Equal(t, name+" header", rec.Header(), http.Header{
					"Content-Type": {"text/plain"}, "Content-Length": {"5"}, "Content-Encoding": {"identity"},
					"Last-Modified": {"x"}, "Cache-Control": {"no-cache"}, "Etag": {tag},
				})
				continue
			}
			testutil.Equal(t, name+" body", rec.Body.Len(), 0)
			testutil.Equal(t, name+" header", rec.Header(), http.Header{"Cache-Control": {"no-cache"}, "Etag": {tag}})
		}
	})

	t.Run("should tag with weak tags unless strong", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields container.Bag
			values map[string]any
			tag    string
		}{
			"default":        {container.Bag{}, nil, tagOf("hello")},
			"strong":         {container.Bag{etag.ConfigFieldStrong: true}, nil, strongTagOf("hello")},
			"weak":           {container.Bag{etag.ConfigFieldStrong: false}, map[string]any{etag.DefaultStrongConfigPath: true}, tagOf("hello")},
			"strong default": {container.Bag{}, map[string]any{etag.DefaultStrongConfigPath: true}, strongTagOf("hello")},
		} {
			m, err := createEtag(t, tc.fields, tc.values)
			testutil.NoError(t, err)
			rec := httptest.NewRecorder()
			serveEtag(m, rec, http.MethodGet, writeText(nil, "hello"))
			testutil.Equal(t, name+" tag", rec.Header().Get("ETag"), tc.tag)
			for _, ifNoneMatch := range []string{tagOf("hello"), strongTagOf("hello")} {
				rec = httptest.NewRecorder()
				serveEtag(m, rec, http.MethodGet, writeText(nil, "hello"), ifNoneMatch)
				testutil.Equal(t, name+" status of "+ifNoneMatch, rec.Code, http.StatusNotModified)
			}
		}
	})

	t.Run("should tell the encodings apart by strong tags only when listed before compress", func(t *testing.T) {
		serve := func(t *testing.T, acceptEncoding string, mws ...flam.IRestMiddleware) *httptest.ResponseRecorder {
			t.Helper()
			router, err := rest.NewRouter(3, mws...)
			testutil.NoError(t, err)
			testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: "GET /items/{id}"},
				http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(compressBody))
				})))
			req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
			if acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", acceptEncoding)
			}
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			return rec
		}

		// compress listed first: the tag of the uncompressed body, shared
		weak := newEtag(t, container.Bag{})
		gzipped := serve(t, "gzip", newCompress(t, container.Bag{}), weak)
		identity := serve(t, "", newCompress(t, container.Bag{}), weak)
		testutil.Equal(t, "gzip encoding", gzipped.Header().Get("Content-Encoding"), "gzip")
		testutil.Equal(t, "identity encoding", identity.Header().Get("Content-Encoding"), "")
		testutil.Equal(t, "gzip tag", gzipped.Header().Get("ETag"), tagOf(compressBody))
		testutil.Equal(t, "identity tag", identity.Header().Get("ETag"), tagOf(compressBody))

		// etag listed first: the strong tags of the bodies sent, different
		strong := newEtag(t, container.Bag{etag.ConfigFieldStrong: true})
		gzipped = serve(t, "gzip", strong, newCompress(t, container.Bag{}))
		identity = serve(t, "", strong, newCompress(t, container.Bag{}))
		testutil.Equal(t, "gzip encoding", gzipped.Header().Get("Content-Encoding"), "gzip")
		testutil.Equal(t, "strong gzip tag", gzipped.Header().Get("ETag"), strongTagOf(gzipped.Body.String()))
		testutil.Equal(t, "strong identity tag", identity.Header().Get("ETag"), strongTagOf(compressBody))
		if gzipped.Header().Get("ETag") == identity.Header().Get("ETag") {
			t.Fatal("the gzip and identity responses share a strong tag")
		}
		testutil.Equal(t, "vary", gzipped.Header().Values("Vary"), []string{"Accept-Encoding"})
	})

	t.Run("should keep the tag the handler set", func(t *testing.T) {
		for name, tc := range map[string]struct {
			before      bool
			ifNoneMatch []string
			status      int
		}{
			"before, unconditional": {true, nil, http.StatusOK},
			"before, matching":      {true, []string{`"own"`}, http.StatusNotModified},
			"before, other":         {true, []string{`"other"`}, http.StatusOK},
			"after, unconditional":  {false, nil, http.StatusOK},
			"after, matching":       {false, []string{`"own"`}, http.StatusNotModified},
		} {
			rec := httptest.NewRecorder()
			var sent int
			var n int
			serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
				if tc.before {
					w.Header().Set("ETag", `W/"own"`)
				}
				n, _ = w.Write([]byte("hello"))
				sent = rec.Body.Len()
				if !tc.before {
					w.Header().Set("ETag", `W/"own"`)
				}
			}, tc.ifNoneMatch...)
			testutil.Equal(t, name+" status", rec.Code, tc.status)
			testutil.Equal(t, name+" written", n, 5)
			testutil.Equal(t, name+" tag", rec.Header().Get("ETag"), `W/"own"`)
			if tc.before && tc.status == http.StatusOK {
				testutil.Equal(t, name+" unbuffered", sent, 5)
			} else {
				testutil.Equal(t, name+" buffered", sent, 0)
			}
			if tc.status == http.StatusOK {
				testutil.Equal(t, name+" body", rec.Body.String(), "hello")
			} else {
				testutil.Equal(t, name+" body", rec.Body.Len(), 0)
			}
		}
	})

	t.Run("should tag the response to a HEAD request with a body only", func(t *testing.T) {
		m := newEtag(t, container.Bag{})
		rec := httptest.NewRecorder()
		serveEtag(m, rec, http.MethodHead, writeText(nil, "hello"))
		testutil.Equal(t, "tag", rec.Header().Get("ETag"), tagOf("hello"))

		rec = httptest.NewRecorder()
		serveEtag(m, rec, http.MethodHead, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
		}, "*")
		testutil.Equal(t, "empty status", rec.Code, http.StatusOK)
		testutil.Equal(t, "empty header", rec.Header(), http.Header{"Content-Type": {"text/plain"}})
	})

	t.Run("should tag the empty body of a GET request", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "tag", rec.Header().Get("ETag"), tagOf(""))
		testutil.Equal(t, "body", rec.Body.Len(), 0)
	})

	t.Run("should send as they are the other responses", func(t *testing.T) {
		for name, tc := range map[string]struct {
			method string
			status int
		}{
			"POST":      {http.MethodPost, http.StatusOK},
			"created":   {http.MethodGet, http.StatusCreated},
			"not found": {http.MethodGet, http.StatusNotFound},
		} {
			rec := httptest.NewRecorder()
			var sent int
			serveEtag(newEtag(t, container.Bag{}), rec, tc.method, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				w.WriteHeader(http.StatusTeapot)
				_, _ = w.Write([]byte("hello"))
				sent = rec.Body.Len()
			}, "*")
			testutil.Equal(t, name+" status", rec.Code, tc.status)
			testutil.Equal(t, name+" unbuffered", sent, 5)
			testutil.Equal(t, name+" tag", rec.Header().Get("ETag"), "")
			testutil.Equal(t, name+" body", rec.Body.String(), "hello")
		}
	})

	t.Run("should send nothing when the handler writes nothing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(http.ResponseWriter, *http.Request) {})
		testutil.Equal(t, "header", rec.Header(), http.Header{})
		testutil.Equal(t, "written", rec.Result().StatusCode, http.StatusOK)
		testutil.Equal(t, "body", rec.Body.Len(), 0)
	})

	t.Run("should send untagged a body longer than the maximum size", func(t *testing.T) {
		m := newEtag(t, container.Bag{etag.ConfigFieldMaxSize: 4})
		for name, parts := range map[string][]int{"whole": nil, "parts": {2, 2}} {
			rec := httptest.NewRecorder()
			serveEtag(m, rec, http.MethodGet, writeText(nil, "hello", parts...), "*")
			testutil.Equal(t, name+" status", rec.Code, http.StatusOK)
			testutil.Equal(t, name+" tag", rec.Header().Get("ETag"), "")
			testutil.Equal(t, name+" body", rec.Body.String(), "hello")
		}

		rec := httptest.NewRecorder()
		serveEtag(m, rec, http.MethodGet, writeText(nil, "four"))
		testutil.Equal(t, "fitting tag", rec.Header().Get("ETag"), tagOf("four"))
	})

	t.Run("should send untagged the response the handler flushes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			testutil.NoError(t, http.NewResponseController(w).Flush())
			_, _ = w.Write([]byte("a"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
		}, "*")
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "tag", rec.Header().Get("ETag"), "")
		testutil.Equal(t, "body", rec.Body.String(), "a")

		rec = httptest.NewRecorder()
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("abc"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
		})
		testutil.Equal(t, "buffered body", rec.Body.String(), "abc")
		testutil.Equal(t, "buffered tag", rec.Header().Get("ETag"), "")
	})

	t.Run("should write the informational statuses through", func(t *testing.T) {
		rec := &statusRecorder{ResponseRecorder: httptest.NewRecorder()}
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			writeText(nil, "hello")(w, req)
		})
		testutil.Equal(t, "statuses", rec.statuses, []int{http.StatusEarlyHints, http.StatusOK})
		testutil.Equal(t, "tag", rec.Header().Get("ETag"), tagOf("hello"))
	})

	t.Run("should unwrap to the writer it wraps", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var unwrapped http.ResponseWriter
		serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
			ew := w.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
			unwrapped = ew.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
		})
		testutil.Equal(t, "unwrapped", unwrapped == http.ResponseWriter(rec), true)
	})

	t.Run("should fail the writes failing", func(t *testing.T) {
		for name, parts := range map[string][]int{"spilled": {3}, "plain": {3, 2}} {
			var errs []error
			serveEtag(newEtag(t, container.Bag{etag.ConfigFieldMaxSize: 4}), failingRecorder{httptest.NewRecorder()}, http.MethodGet,
				func(w http.ResponseWriter, _ *http.Request) {
					data := []byte("hello world")
					for _, size := range parts {
						n, err := w.Write(data[:size])
						if err != nil {
							testutil.Equal(t, name+" written", n, 0)
						}
						errs = append(errs, err)
						data = data[size:]
					}
					_, err := w.Write(data)
					errs = append(errs, err)
				})
			testutil.Equal(t, name+" error", errors.Is(errs[len(errs)-1], errBoom), true)
		}
	})

	t.Run("should leave the response unsent when the handler panics", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testutil.Panics(t, "ServeHTTP", func() {
			serveEtag(newEtag(t, container.Bag{}), rec, http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("hello"))
				panic(errBoom)
			})
		})
		testutil.Equal(t, "header", rec.Header(), http.Header{})
		testutil.Equal(t, "body", rec.Body.Len(), 0)
	})

	t.Run("should pass the requests of a streaming endpoint through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var given http.ResponseWriter
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("If-None-Match", "*")
		newEtag(t, container.Bag{}).Wrap(flam.RestEndpoint{Pattern: "GET /events", Streaming: true},
			http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				given = w
				writeText(nil, "hello")(w, req)
			})).ServeHTTP(rec, req)
		testutil.Equal(t, "writer", given == http.ResponseWriter(rec), true)
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "header", rec.Header(), http.Header{"Content-Type": {"text/plain"}})
		testutil.Equal(t, "body", rec.Body.String(), "hello")
	})

	t.Run("should take the field, then its defaults", func(t *testing.T) {
		m, err := createEtag(t, container.Bag{}, map[string]any{etag.DefaultMaxSizeConfigPath: 2})
		testutil.NoError(t, err)
		rec := httptest.NewRecorder()
		serveEtag(m, rec, http.MethodGet, writeText(nil, "abc"))
		testutil.Equal(t, "default tag", rec.Header().Get("ETag"), "")

		m, err = createEtag(t, container.Bag{etag.ConfigFieldMaxSize: 3}, map[string]any{etag.DefaultMaxSizeConfigPath: 2})
		testutil.NoError(t, err)
		rec = httptest.NewRecorder()
		serveEtag(m, rec, http.MethodGet, writeText(nil, "abc"))
		testutil.Equal(t, "field tag", rec.Header().Get("ETag"), tagOf("abc"))

		m, err = create(t, bare{etag.NewBundle()}, etag.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		rec = httptest.NewRecorder()
		big := string(make([]byte, 1<<20))
		serveEtag(m, rec, http.MethodGet, writeText(nil, big))
		testutil.Equal(t, "built-in tag", rec.Header().Get("ETag"), tagOf(big))
		rec = httptest.NewRecorder()
		serveEtag(m, rec, http.MethodGet, writeText(nil, big+"x"))
		testutil.Equal(t, "built-in longer tag", rec.Header().Get("ETag"), "")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, value := range []any{0, -1, "1", 1.5} {
			_, err := createEtag(t, container.Bag{etag.ConfigFieldMaxSize: value}, nil)
			testutil.Error(t, errors.Unwrap(err), etag.ErrInvalidField,
				"unable to create etag rest middleware: invalid etag rest middleware field",
				map[string]any{etag.ErrorFieldID: "main", etag.ErrorFieldField: etag.ConfigFieldMaxSize})
		}
		_, err := createEtag(t, container.Bag{}, map[string]any{etag.DefaultMaxSizeConfigPath: "x"})
		testutil.Error(t, errors.Unwrap(err), etag.ErrInvalidField,
			"unable to create etag rest middleware: invalid etag rest middleware field",
			map[string]any{etag.ErrorFieldID: "main", etag.ErrorFieldField: etag.ConfigFieldMaxSize})
		for _, value := range []any{"true", 1} {
			_, err = createEtag(t, container.Bag{etag.ConfigFieldStrong: value}, nil)
			testutil.Error(t, errors.Unwrap(err), etag.ErrInvalidField,
				"unable to create etag rest middleware: invalid etag rest middleware field",
				map[string]any{etag.ErrorFieldID: "main", etag.ErrorFieldField: etag.ConfigFieldStrong})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createEtag(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
