package middlewares

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/compress"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// compressBody is a JSON body longer than the default minimum size.
var compressBody = `{"items":[` + strings.Repeat(`"item",`, 200) + `"last"]}`

// createCompress returns the compress middleware "main" of the fields,
// created with the compress bundle and the values.
func createCompress(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, compress.NewBundle(), compress.Driver, fields, values, deps{})
}

// newCompress returns the compress middleware of the fields, failing the
// test when it cannot be created.
func newCompress(t *testing.T, fields container.Bag) flam.IRestMiddleware {
	t.Helper()
	m, err := createCompress(t, fields, nil)
	testutil.NoError(t, err)
	return m
}

// statusRecorder is a response recorder recording every status written,
// informational ones included, which the recorder it embeds does not see,
// as it would take them for the final one.
type statusRecorder struct {
	*httptest.ResponseRecorder

	statuses []int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.statuses = append(r.statuses, status)
	if status >= 200 {
		r.ResponseRecorder.WriteHeader(status)
	}
}

// failingRecorder is a response recorder whose writes fail.
type failingRecorder struct{ *httptest.ResponseRecorder }

func (failingRecorder) Write([]byte) (int, error) { return 0, errBoom }

// serveCompress serves a request of the method and the Accept-Encoding
// header, none when empty, through the middleware into w, with the
// handler.
func serveCompress(m flam.IRestMiddleware, w http.ResponseWriter, method, acceptEncoding string, handler http.HandlerFunc) {
	req := httptest.NewRequest(method, "/items/1", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	m.Wrap(logEndpoint(0), handler).ServeHTTP(w, req)
}

// writeJSON returns the handler answering with the body as JSON, in parts
// of the sizes, the rest of the body last.
func writeJSON(body string, parts ...int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rest := body
		for _, size := range parts {
			_, _ = w.Write([]byte(rest[:size]))
			rest = rest[size:]
		}
		_, _ = w.Write([]byte(rest))
	}
}

// decompress returns the body of the recorded response, decompressed by
// its Content-Encoding.
func decompress(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var r io.Reader = rec.Body
	switch rec.Header().Get("Content-Encoding") {
	case compress.EncodingGzip:
		zr, err := gzip.NewReader(rec.Body)
		testutil.NoError(t, err)
		r = zr
	case compress.EncodingDeflate:
		r = flate.NewReader(rec.Body)
	}
	data, err := io.ReadAll(r)
	testutil.NoError(t, err)
	return string(data)
}

func Test_Compress_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := compress.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.compress")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + compress.Id: container.Bag{factory.ConfigFieldDriver: compress.Driver},
			compress.DefaultLevelConfigPath:               -1,
			compress.DefaultMinSizeConfigPath:             1024,
			compress.DefaultContentTypesConfigPath:        []string{"application/json", "application/problem+json", "text/*"},
			compress.DefaultEncodingsConfigPath:           []string{"gzip", "deflate"},
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			compress.Driver:                        "flam.rest.middlewares.driver.compress",
			compress.DefaultLevelConfigPath:        "flam.rest.defaults.middlewares.compress.level",
			compress.DefaultMinSizeConfigPath:      "flam.rest.defaults.middlewares.compress.min_size",
			compress.DefaultContentTypesConfigPath: "flam.rest.defaults.middlewares.compress.content_types",
			compress.DefaultEncodingsConfigPath:    "flam.rest.defaults.middlewares.compress.encodings",
			compress.EncodingGzip:                  "gzip",
			compress.EncodingDeflate:               "deflate",
		} {
			testutil.Equal(t, "const", got, want)
		}
		testutil.Equal(t, "Id", compress.Id, "compress")
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(compress.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.compress", "provider/1"})
	})
}

func Test_Compress(t *testing.T) {
	t.Run("should compress a response reaching the minimum size", func(t *testing.T) {
		m := newCompress(t, container.Bag{})
		testutil.Equal(t, "Id()", m.Id(), "main")

		for range 2 { // the second one reuses the pooled compressor
			rec := httptest.NewRecorder()
			var status, size int
			serveCompress(m, rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Length", "9999")
				w.Header().Set("Accept-Ranges", "bytes")
				_, _ = w.Write([]byte(compressBody[:10]))
				_, _ = w.Write([]byte(compressBody[10:]))
				rw := w.(flam.IRestResponseWriter)
				status, size = rw.Status(), rw.Size()
			})
			testutil.Equal(t, "status", rec.Code, http.StatusOK)
			testutil.Equal(t, "header", rec.Header(), http.Header{
				"Content-Type":     {"application/json"},
				"Content-Encoding": {"gzip"},
				"Vary":             {"Accept-Encoding"},
			})
			testutil.Equal(t, "body", decompress(t, rec), compressBody)
			testutil.Equal(t, "recorded status", status, http.StatusOK)
			testutil.Equal(t, "recorded size", size, len(compressBody))
		}
	})

	t.Run("should negotiate the encoding", func(t *testing.T) {
		for name, tc := range map[string]struct {
			encodings []string
			accept    string
			want      string
		}{
			"gzip":                 {nil, "gzip", "gzip"},
			"deflate":              {nil, "deflate", "deflate"},
			"x-gzip":               {nil, "X-GZIP", "gzip"},
			"tie first listed":     {nil, "deflate, gzip", "gzip"},
			"tie own order":        {[]string{"deflate", "gzip"}, "gzip, deflate", "deflate"},
			"higher quality":       {nil, "gzip;q=0.5, deflate;q=0.6", "deflate"},
			"refused":              {nil, "gzip;q=0, deflate;q=0.1", "deflate"},
			"wildcard":             {nil, "*;q=0.3", "gzip"},
			"wildcard after":       {nil, "gzip;q=0, *", "deflate"},
			"empty entries":        {nil, " , ;q=1,gzip ; Q = 0.9 ; level=1", "gzip"},
			"other parameter":      {nil, "gzip;level=9", "gzip"},
			"unknown only":         {nil, "br, identity", ""},
			"invalid quality":      {nil, "gzip;q=high", ""},
			"quality over 1":       {nil, "gzip;q=1.5", ""},
			"negative quality":     {nil, "gzip;q=-1", ""},
			"NaN quality":          {nil, "gzip;q=NaN", ""},
			"unsupported encoding": {[]string{"gzip"}, "deflate", ""},
		} {
			fields := container.Bag{}
			if tc.encodings != nil {
				fields[compress.ConfigFieldEncodings] = tc.encodings
			}
			rec := httptest.NewRecorder()
			serveCompress(newCompress(t, fields), rec, http.MethodGet, tc.accept, writeJSON(compressBody))
			testutil.Equal(t, name+" encoding", rec.Header().Get("Content-Encoding"), tc.want)
			testutil.Equal(t, name+" body", decompress(t, rec), compressBody)
		}
	})

	t.Run("should take every Accept-Encoding header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
		req.Header.Add("Accept-Encoding", "gzip;q=0.1")
		req.Header.Add("Accept-Encoding", "deflate")
		rec := httptest.NewRecorder()
		newCompress(t, container.Bag{}).Wrap(logEndpoint(0), writeJSON(compressBody)).ServeHTTP(rec, req)
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "deflate")
	})

	t.Run("should send as they are the responses it may not compress", func(t *testing.T) {
		for name, tc := range map[string]struct {
			method  string
			accept  string
			status  int
			header  http.Header
			body    string
			want    http.Header
			payload string
		}{
			"no Accept-Encoding": {
				method: http.MethodGet, status: http.StatusOK,
				header: http.Header{"Content-Type": {"application/json"}}, body: compressBody,
				want: http.Header{"Content-Type": {"application/json"}, "Vary": {"Accept-Encoding"}}, payload: compressBody,
			},
			"HEAD": {
				method: http.MethodHead, accept: "gzip", status: http.StatusOK,
				header: http.Header{"Content-Type": {"application/json"}}, body: compressBody,
				want: http.Header{"Content-Type": {"application/json"}, "Vary": {"Accept-Encoding"}}, payload: compressBody,
			},
			"below the minimum size": {
				method: http.MethodGet, accept: "gzip", status: http.StatusCreated,
				header: http.Header{"Content-Type": {"application/json"}, "Content-Length": {"2"}}, body: "{}",
				want: http.Header{"Content-Type": {"application/json"}, "Content-Length": {"2"}, "Vary": {"Accept-Encoding"}}, payload: "{}",
			},
			"other content type": {
				method: http.MethodGet, accept: "gzip", status: http.StatusOK,
				header: http.Header{"Content-Type": {"image/png"}}, body: compressBody,
				want: http.Header{"Content-Type": {"image/png"}, "Vary": {"Accept-Encoding"}}, payload: compressBody,
			},
			"already encoded": {
				method: http.MethodGet, accept: "gzip", status: http.StatusOK,
				header: http.Header{"Content-Type": {"text/plain"}, "Content-Encoding": {"br"}}, body: compressBody,
				want: http.Header{"Content-Type": {"text/plain"}, "Content-Encoding": {"br"}, "Vary": {"Accept-Encoding"}}, payload: compressBody,
			},
			"partial": {
				method: http.MethodGet, accept: "gzip", status: http.StatusPartialContent,
				header: http.Header{"Content-Type": {"text/plain"}, "Content-Range": {"bytes 0-1/10"}}, body: "ab",
				want: http.Header{"Content-Type": {"text/plain"}, "Content-Range": {"bytes 0-1/10"}, "Vary": {"Accept-Encoding"}}, payload: "ab",
			},
			"no content": {
				method: http.MethodPost, accept: "gzip", status: http.StatusNoContent,
				header: http.Header{"Content-Type": {"text/plain"}},
				want:   http.Header{"Content-Type": {"text/plain"}, "Vary": {"Accept-Encoding"}},
			},
			"not modified": {
				method: http.MethodGet, accept: "gzip", status: http.StatusNotModified,
				header: http.Header{"Content-Type": {"text/plain"}},
				want:   http.Header{"Content-Type": {"text/plain"}, "Vary": {"Accept-Encoding"}},
			},
			"sniffing suppressed": {
				method: http.MethodGet, accept: "gzip", status: http.StatusOK,
				header: http.Header{"Content-Type": nil}, body: compressBody,
				want: http.Header{"Content-Type": nil, "Vary": {"Accept-Encoding"}}, payload: compressBody,
			},
			"sniffed other content type": {
				method: http.MethodGet, accept: "gzip", status: http.StatusOK,
				header: http.Header{}, body: "\x89PNG\x0D\x0A\x1A\x0A" + compressBody,
				want: http.Header{"Content-Type": {"image/png"}, "Vary": {"Accept-Encoding"}}, payload: "\x89PNG\x0D\x0A\x1A\x0A" + compressBody,
			},
			"empty body": {
				method: http.MethodGet, accept: "gzip", status: http.StatusOK,
				header: http.Header{"Content-Type": {"text/plain"}},
				want:   http.Header{"Content-Type": {"text/plain"}, "Vary": {"Accept-Encoding"}},
			},
		} {
			rec := httptest.NewRecorder()
			serveCompress(newCompress(t, container.Bag{}), rec, tc.method, tc.accept, func(w http.ResponseWriter, _ *http.Request) {
				for key, values := range tc.header {
					w.Header()[key] = values
				}
				w.WriteHeader(tc.status)
				w.WriteHeader(http.StatusTeapot)
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			})
			testutil.Equal(t, name+" status", rec.Code, tc.status)
			testutil.Equal(t, name+" header", rec.Header(), tc.want)
			testutil.Equal(t, name+" body", rec.Body.String(), tc.payload)
		}
	})

	t.Run("should send nothing when the handler writes nothing", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(http.ResponseWriter, *http.Request) {})
		testutil.Equal(t, "written", rec.Result().Header, http.Header{"Vary": {"Accept-Encoding"}})
		testutil.Equal(t, "body", rec.Body.Len(), 0)
	})

	t.Run("should keep the Vary header listing Accept-Encoding or any header", func(t *testing.T) {
		for vary, want := range map[string][]string{
			"Origin":                  {"Origin", "Accept-Encoding"},
			"Origin, accept-encoding": {"Origin, accept-encoding"},
			"*":                       {"*"},
		} {
			rec := httptest.NewRecorder()
			rec.Header().Set("Vary", vary)
			serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "", writeJSON("{}"))
			testutil.Equal(t, vary, rec.Header().Values("Vary"), want)
		}
	})

	t.Run("should match the content types", func(t *testing.T) {
		for name, tc := range map[string]struct {
			types       []string
			contentType string
			want        string
		}{
			"exact":              {nil, "application/json", "gzip"},
			"case and params":    {nil, " Application/Problem+JSON ; charset=utf-8", "gzip"},
			"type wildcard":      {nil, "text/csv", "gzip"},
			"other type":         {nil, "application/xml", ""},
			"empty":              {nil, " ; charset=utf-8", ""},
			"any":                {[]string{"*/*"}, "image/svg+xml", "gzip"},
			"own type":           {[]string{" Image/SVG+XML "}, "image/svg+xml", "gzip"},
			"own type wildcard":  {[]string{"image/*"}, "image/png", "gzip"},
			"own type no prefix": {[]string{"image/*"}, "text/plain", ""},
		} {
			fields := container.Bag{}
			if tc.types != nil {
				fields[compress.ConfigFieldContentTypes] = tc.types
			}
			rec := httptest.NewRecorder()
			serveCompress(newCompress(t, fields), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				_, _ = w.Write([]byte(compressBody))
			})
			testutil.Equal(t, name, rec.Header().Get("Content-Encoding"), tc.want)
			testutil.Equal(t, name+" body", decompress(t, rec), compressBody)
		}
	})

	t.Run("should sniff the content type of the body", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(compressBody))
		})
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Content-Type":     {"text/plain; charset=utf-8"},
			"Content-Encoding": {"gzip"},
			"Vary":             {"Accept-Encoding"},
		})
		testutil.Equal(t, "body", decompress(t, rec), compressBody)
	})

	t.Run("should compress any body with a minimum size of 0", func(t *testing.T) {
		m := newCompress(t, container.Bag{compress.ConfigFieldMinSize: 0, compress.ConfigFieldLevel: 9})
		rec := httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "gzip", writeJSON("{}"))
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "gzip")
		testutil.Equal(t, "body", decompress(t, rec), "{}")

		rec = httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(nil)
		})
		testutil.Equal(t, "empty encoding", rec.Header().Get("Content-Encoding"), "")
		testutil.Equal(t, "empty body", rec.Body.Len(), 0)
	})

	t.Run("should compress the response the handler flushes", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var mid string
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "deflate", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: 1\n\n"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
			// the stream is not closed yet, so that only what was flushed is read
			flushed := make([]byte, len("data: 1\n\n"))
			_, err := io.ReadFull(flate.NewReader(bytes.NewReader(rec.Body.Bytes())), flushed)
			testutil.NoError(t, err)
			mid = string(flushed)
			_, _ = w.Write([]byte("data: 2\n\n"))
			testutil.NoError(t, http.NewResponseController(w).Flush())
		})
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "deflate")
		testutil.Equal(t, "flushed body", mid, "data: 1\n\n")
		testutil.Equal(t, "body", decompress(t, rec), "data: 1\n\ndata: 2\n\n")
	})

	t.Run("should send as it is the response the handler flushes without body or type", func(t *testing.T) {
		rec := httptest.NewRecorder()
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			testutil.NoError(t, http.NewResponseController(w).Flush())
			testutil.NoError(t, http.NewResponseController(w).Flush())
			_, _ = w.Write([]byte("late"))
		})
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "")
		testutil.Equal(t, "body", rec.Body.String(), "late")
	})

	t.Run("should flush the plain response directly", func(t *testing.T) {
		rec := httptest.NewRecorder()
		writer := &compressFlusher{}
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
			writer.flush(w)
		})
		testutil.Equal(t, "flushed", rec.Flushed, true)
		testutil.Equal(t, "body", rec.Body.String(), "x")
	})

	t.Run("should write the informational statuses through", func(t *testing.T) {
		rec := &statusRecorder{ResponseRecorder: httptest.NewRecorder()}
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusEarlyHints)
			writeJSON(compressBody)(w, nil)
			w.WriteHeader(http.StatusEarlyHints)
		})
		testutil.Equal(t, "statuses", rec.statuses, []int{http.StatusEarlyHints, http.StatusOK})
		testutil.Equal(t, "body", decompress(t, rec.ResponseRecorder), compressBody)
	})

	t.Run("should unwrap to the writer it wraps", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var unwrapped http.ResponseWriter
		serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
			cw := w.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
			unwrapped = cw.(interface{ Unwrap() http.ResponseWriter }).Unwrap()
		})
		testutil.Equal(t, "unwrapped", unwrapped == http.ResponseWriter(rec), true)
	})

	t.Run("should fail the writes failing", func(t *testing.T) {
		for name, tc := range map[string]struct {
			contentType string
			parts       []int
		}{
			"compressed":      {"application/json", nil},
			"plain":           {"", nil},
			"compressed late": {"application/json", []int{len(compressBody) - 1}},
		} {
			rec := failingRecorder{httptest.NewRecorder()}
			var errs []error
			serveCompress(newCompress(t, container.Bag{compress.ConfigFieldMinSize: 4}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
				if tc.contentType != "" {
					w.Header().Set("Content-Type", tc.contentType)
				} else {
					w.Header().Set("Content-Type", "image/png")
				}
				data := []byte(compressBody)
				for _, size := range tc.parts {
					_, err := w.Write(data[:size])
					errs = append(errs, err)
					data = data[size:]
				}
				n, err := w.Write(data)
				testutil.Equal(t, name+" written", n, 0)
				errs = append(errs, err)
			})
			testutil.Equal(t, name+" error", errors.Is(errs[len(errs)-1], errBoom), true)
		}
	})

	t.Run("should leave the response unsent when the handler panics", func(t *testing.T) {
		rec := httptest.NewRecorder()
		testutil.Panics(t, "ServeHTTP", func() {
			serveCompress(newCompress(t, container.Bag{}), rec, http.MethodGet, "gzip", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("{}"))
				panic(errBoom)
			})
		})
		testutil.Equal(t, "header", rec.Header(), http.Header{"Vary": {"Accept-Encoding"}})
		testutil.Equal(t, "body", rec.Body.Len(), 0)
	})

	t.Run("should pass the requests of a streaming endpoint through", func(t *testing.T) {
		rec := httptest.NewRecorder()
		var given http.ResponseWriter
		req := httptest.NewRequest(http.MethodGet, "/events", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		newCompress(t, container.Bag{}).Wrap(flam.RestEndpoint{Pattern: "GET /events", Streaming: true},
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				given = w
				writeJSON(compressBody)(w, nil)
			})).ServeHTTP(rec, req)
		testutil.Equal(t, "writer", given == http.ResponseWriter(rec), true)
		testutil.Equal(t, "header", rec.Header(), http.Header{"Content-Type": {"application/json"}})
		testutil.Equal(t, "body", rec.Body.String(), compressBody)
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		m, err := createCompress(t, container.Bag{compress.ConfigFieldMinSize: 3}, map[string]any{
			compress.DefaultLevelConfigPath:        1,
			compress.DefaultContentTypesConfigPath: []any{"text/csv"},
			compress.DefaultEncodingsConfigPath:    []any{"deflate"},
		})
		testutil.NoError(t, err)
		rec := httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "gzip, deflate", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("a,b"))
		})
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "deflate")
		testutil.Equal(t, "body", decompress(t, rec), "a,b")

		m, err = createCompress(t, container.Bag{
			compress.ConfigFieldLevel:        flate.HuffmanOnly,
			compress.ConfigFieldContentTypes: []string{"text/csv"},
			compress.ConfigFieldEncodings:    []string{"gzip"},
		}, map[string]any{compress.DefaultMinSizeConfigPath: 2})
		testutil.NoError(t, err)
		rec = httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "gzip, deflate", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/csv")
			_, _ = w.Write([]byte("ab"))
		})
		testutil.Equal(t, "second encoding", rec.Header().Get("Content-Encoding"), "gzip")
		testutil.Equal(t, "second body", decompress(t, rec), "ab")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		m, err := create(t, bare{compress.NewBundle()}, compress.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		rec := httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "deflate", writeJSON(compressBody))
		testutil.Equal(t, "encoding", rec.Header().Get("Content-Encoding"), "deflate")
		testutil.Equal(t, "body", decompress(t, rec), compressBody)

		rec = httptest.NewRecorder()
		serveCompress(m, rec, http.MethodGet, "gzip", writeJSON(compressBody[:1023]))
		testutil.Equal(t, "small encoding", rec.Header().Get("Content-Encoding"), "")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{compress.ConfigFieldLevel, -3},
			{compress.ConfigFieldLevel, 10},
			{compress.ConfigFieldLevel, "1"},
			{compress.ConfigFieldMinSize, -1},
			{compress.ConfigFieldMinSize, 1.5},
			{compress.ConfigFieldContentTypes, "text/*"},
			{compress.ConfigFieldContentTypes, []any{"text/*", 1}},
			{compress.ConfigFieldContentTypes, []any{" "}},
			{compress.ConfigFieldEncodings, []any{}},
			{compress.ConfigFieldEncodings, []any{"gzip", "br"}},
			{compress.ConfigFieldEncodings, "gzip"},
		} {
			_, err := createCompress(t, container.Bag{tc.field: tc.value}, nil)
			testutil.Error(t, errors.Unwrap(err), compress.ErrInvalidField,
				"unable to create compress rest middleware: invalid compress rest middleware field",
				map[string]any{compress.ErrorFieldID: "main", compress.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		for path, field := range map[string]string{
			compress.DefaultLevelConfigPath:        compress.ConfigFieldLevel,
			compress.DefaultMinSizeConfigPath:      compress.ConfigFieldMinSize,
			compress.DefaultContentTypesConfigPath: compress.ConfigFieldContentTypes,
			compress.DefaultEncodingsConfigPath:    compress.ConfigFieldEncodings,
		} {
			_, err := createCompress(t, container.Bag{}, map[string]any{path: "x"})
			testutil.Error(t, errors.Unwrap(err), compress.ErrInvalidField,
				"unable to create compress rest middleware: invalid compress rest middleware field",
				map[string]any{compress.ErrorFieldID: "main", compress.ErrorFieldField: field})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createCompress(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

// compressFlusher flushes the writers given to it.
type compressFlusher struct{}

func (compressFlusher) flush(w http.ResponseWriter) {
	w.(http.Flusher).Flush()
}
