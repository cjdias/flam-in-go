package middlewares

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/bodylimit"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// tooLargeBody is the problem answering a request of the endpoint 7 of the
// service 3 announcing a body too large.
const tooLargeBody = `{"detail":"request body too large","id":"s:3.e:7","status":413,` +
	`"title":"Request Entity Too Large","type":"https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/413"}`

// createBodyLimit returns the body limit middleware "main" of the fields,
// created with the body limit bundle and the values.
func createBodyLimit(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, bodylimit.NewBundle(), bodylimit.Driver, fields, values, deps{})
}

// bodyRead is what a handler read from the body of its request.
type bodyRead struct {
	called bool
	data   string
	err    error
}

// serveBodyLimit serves the request through the middleware, with a router
// of the service 3 holding the endpoint 7 of the pattern "POST /items",
// whose handler reads the body whole, returning the recorded response and
// what the handler read.
func serveBodyLimit(t *testing.T, m flam.IRestMiddleware, req *http.Request) (*httptest.ResponseRecorder, bodyRead) {
	t.Helper()
	router, err := rest.NewRouter(3, m)
	testutil.NoError(t, err)
	var read bodyRead
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: "POST /items"}, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		read.called = true
		data, err := io.ReadAll(req.Body)
		read.data, read.err = string(data), err
		_, _ = w.Write([]byte("ok"))
	})))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec, read
}

// unsized is a request body of unknown length, so that the request
// announces none.
type unsized struct{ io.Reader }

func Test_BodyLimit_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := bodylimit.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.bodylimit")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + bodylimit.Id: container.Bag{factory.ConfigFieldDriver: bodylimit.Driver},
			bodylimit.DefaultMaxBytesConfigPath:            1048576,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			bodylimit.Id:                        "bodylimit",
			bodylimit.Driver:                    "flam.rest.middlewares.driver.bodylimit",
			bodylimit.ConfigFieldMaxBytes:       "max_bytes",
			bodylimit.DefaultMaxBytesConfigPath: "flam.rest.defaults.middlewares.bodylimit.max_bytes",
		} {
			testutil.Equal(t, "const", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(bodylimit.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.bodylimit", "provider/1"})
	})
}

func Test_BodyLimit(t *testing.T) {
	t.Run("should serve the requests of a body within the limit", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: 5}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", m.Id(), "main")

		for name, body := range map[string]io.Reader{
			"sized":   strings.NewReader("12345"),
			"unsized": unsized{strings.NewReader("12345")},
			"empty":   nil,
		} {
			rec, read := serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", body))
			testutil.Equal(t, name+" status", rec.Code, http.StatusOK)
			testutil.Equal(t, name+" called", read.called, true)
			testutil.Equal(t, name+" err", read.err, nil)
			if body != nil {
				testutil.Equal(t, name+" data", read.data, "12345")
			}
		}
	})

	t.Run("should answer a request announcing a body too large with a 413 envelope", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: 5}, nil)
		testutil.NoError(t, err)

		rec, read := serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("123456")))
		testutil.Equal(t, "called", read.called, false)
		testutil.Equal(t, "status", rec.Code, http.StatusRequestEntityTooLarge)
		testutil.Equal(t, "content type", rec.Header().Get("Content-Type"), flamenvelope.ContentTypeProblem)
		testutil.Equal(t, "body", rec.Body.String(), tooLargeBody)
	})

	t.Run("should fail the reads past the limit of a body of unknown length", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: 5}, nil)
		testutil.NoError(t, err)

		rec, read := serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", unsized{strings.NewReader("123456")}))
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "called", read.called, true)
		testutil.Equal(t, "data", read.data, "12345")
		var maxBytes *http.MaxBytesError
		testutil.Equal(t, "max bytes error", errors.As(read.err, &maxBytes), true)
		testutil.Equal(t, "limit", maxBytes.Limit, int64(5))
	})

	t.Run("should fail the decoding of a JSON body past the limit", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: 8}, nil)
		testutil.NoError(t, err)

		var decodeErr error
		req := httptest.NewRequest(http.MethodPost, "/items", unsized{strings.NewReader(`{"name":"long"}`)})
		m.Wrap(logEndpoint(0), http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			var v map[string]any
			decodeErr = rest.DecodeJSON(req, &v)
		})).ServeHTTP(httptest.NewRecorder(), req)
		testutil.Equal(t, "decode", errors.Is(decodeErr, rest.ErrDecode), true)
		var maxBytes *http.MaxBytesError
		testutil.Equal(t, "max bytes error", errors.As(decodeErr, &maxBytes), true)
	})

	t.Run("should leave a request without body as it is", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{}, nil)
		testutil.NoError(t, err)
		for name, body := range map[string]io.ReadCloser{"nil": nil, "no body": http.NoBody} {
			req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
			req.Body = body
			var got io.ReadCloser
			m.Wrap(logEndpoint(0), http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
				got = req.Body
			})).ServeHTTP(httptest.NewRecorder(), req)
			testutil.Equal(t, name+" body", got, body)
		}
	})

	t.Run("should take the field, then its default", func(t *testing.T) {
		m, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: 3}, map[string]any{bodylimit.DefaultMaxBytesConfigPath: 10})
		testutil.NoError(t, err)
		rec, _ := serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("1234")))
		testutil.Equal(t, "field status", rec.Code, http.StatusRequestEntityTooLarge)

		m, err = createBodyLimit(t, container.Bag{}, map[string]any{bodylimit.DefaultMaxBytesConfigPath: 4})
		testutil.NoError(t, err)
		rec, _ = serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("1234")))
		testutil.Equal(t, "default status", rec.Code, http.StatusOK)
		rec, _ = serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader("12345")))
		testutil.Equal(t, "default over status", rec.Code, http.StatusRequestEntityTooLarge)
	})

	t.Run("should use the built-in default when the default is missing", func(t *testing.T) {
		m, err := create(t, bare{bodylimit.NewBundle()}, bodylimit.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		rec, _ := serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(strings.Repeat("x", 1<<20))))
		testutil.Equal(t, "max status", rec.Code, http.StatusOK)
		rec, _ = serveBodyLimit(t, m, httptest.NewRequest(http.MethodPost, "/items", strings.NewReader(strings.Repeat("x", 1<<20+1))))
		testutil.Equal(t, "over status", rec.Code, http.StatusRequestEntityTooLarge)
	})

	t.Run("should fail with an invalid field or default", func(t *testing.T) {
		for _, value := range []any{0, -1, "1", 1.5, int64(1), nil} {
			_, err := createBodyLimit(t, container.Bag{bodylimit.ConfigFieldMaxBytes: value}, nil)
			testutil.Error(t, errors.Unwrap(err), bodylimit.ErrInvalidField,
				"unable to create body limit rest middleware: invalid body limit rest middleware field",
				map[string]any{bodylimit.ErrorFieldID: "main", bodylimit.ErrorFieldField: bodylimit.ConfigFieldMaxBytes})
		}
		_, err := createBodyLimit(t, container.Bag{}, map[string]any{bodylimit.DefaultMaxBytesConfigPath: "1MiB"})
		testutil.Error(t, errors.Unwrap(err), bodylimit.ErrInvalidField,
			"unable to create body limit rest middleware: invalid body limit rest middleware field",
			map[string]any{bodylimit.ErrorFieldID: "main", bodylimit.ErrorFieldField: bodylimit.ConfigFieldMaxBytes})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createBodyLimit(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
