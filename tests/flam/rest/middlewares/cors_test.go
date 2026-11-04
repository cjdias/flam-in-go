package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/cors"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createCors returns the CORS middleware "main" of the fields, created with
// the CORS bundle and the values.
func createCors(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, cors.NewBundle(), cors.Driver, fields, values, deps{})
}

// newCors returns the CORS middleware of the fields, failing the test when
// it cannot be created.
func newCors(t *testing.T, fields container.Bag) flam.IRestMiddleware {
	t.Helper()
	m, err := createCors(t, fields, nil)
	testutil.NoError(t, err)
	return m
}

// serveCors serves a request of the method and the headers through the
// middleware, with a handler answering 200 "ok", returning the recorded
// response and whether the handler was called.
func serveCors(m flam.IRestMiddleware, method string, header http.Header) (*httptest.ResponseRecorder, bool) {
	req := httptest.NewRequest(method, "/items/1", nil)
	req.Header = header
	rec := httptest.NewRecorder()
	called := false
	m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(rec, req)
	return rec, called
}

// preflight returns the headers of a preflight request from the origin for
// the method and the request headers, none when empty.
func preflight(origin, method, headers string) http.Header {
	header := http.Header{"Origin": {origin}, "Access-Control-Request-Method": {method}}
	if headers != "" {
		header.Set("Access-Control-Request-Headers", headers)
	}
	return header
}

// preflightVary is the Vary header of the answers to the preflight
// requests.
var preflightVary = []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"}

func Test_Cors_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := cors.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.cors")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + cors.Id: container.Bag{factory.ConfigFieldDriver: cors.Driver},
			cors.DefaultAllowedOriginsConfigPath:      []string{"*"},
			cors.DefaultAllowedMethodsConfigPath:      []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"},
			cors.DefaultAllowedHeadersConfigPath:      []string{"Accept", "Authorization", "Content-Type", "X-Request-Id"},
			cors.DefaultExposedHeadersConfigPath:      []string{"X-Request-Id"},
			cors.DefaultAllowCredentialsConfigPath:    false,
			cors.DefaultMaxAgeConfigPath:              "10m0s",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			cors.Id:                                "cors",
			cors.Driver:                            "flam.rest.middlewares.driver.cors",
			cors.Wildcard:                          "*",
			cors.ConfigFieldAllowedOrigins:         "allowed_origins",
			cors.ConfigFieldAllowedMethods:         "allowed_methods",
			cors.ConfigFieldAllowedHeaders:         "allowed_headers",
			cors.ConfigFieldExposedHeaders:         "exposed_headers",
			cors.ConfigFieldAllowCredentials:       "allow_credentials",
			cors.ConfigFieldMaxAge:                 "max_age",
			cors.DefaultAllowedOriginsConfigPath:   "flam.rest.defaults.middlewares.cors.allowed_origins",
			cors.DefaultAllowedMethodsConfigPath:   "flam.rest.defaults.middlewares.cors.allowed_methods",
			cors.DefaultAllowedHeadersConfigPath:   "flam.rest.defaults.middlewares.cors.allowed_headers",
			cors.DefaultExposedHeadersConfigPath:   "flam.rest.defaults.middlewares.cors.exposed_headers",
			cors.DefaultAllowCredentialsConfigPath: "flam.rest.defaults.middlewares.cors.allow_credentials",
			cors.DefaultMaxAgeConfigPath:           "flam.rest.defaults.middlewares.cors.max_age",
		} {
			testutil.Equal(t, "const", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(cors.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.cors", "provider/1"})
	})
}

func Test_Cors(t *testing.T) {
	listed := container.Bag{
		cors.ConfigFieldAllowedOrigins:   []any{"https://App.example.com", " https://admin.example.com "},
		cors.ConfigFieldAllowedMethods:   []any{"GET", "PUT"},
		cors.ConfigFieldAllowedHeaders:   []any{"Content-Type", "X-Api-Key"},
		cors.ConfigFieldExposedHeaders:   []any{"X-Request-Id", "X-Total"},
		cors.ConfigFieldAllowCredentials: true,
		cors.ConfigFieldMaxAge:           90 * time.Second,
	}

	t.Run("should serve a request without origin as it is", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{"defaults": {}, "listed": listed} {
			m := newCors(t, fields)
			testutil.Equal(t, name+" Id()", m.Id(), "main")
			rec, called := serveCors(m, http.MethodGet, http.Header{})
			testutil.Equal(t, name+" called", called, true)
			testutil.Equal(t, name+" status", rec.Code, http.StatusOK)
			testutil.Equal(t, name+" allow origin", rec.Header().Values("Access-Control-Allow-Origin"), []string(nil))
			testutil.Equal(t, name+" expose", rec.Header().Values("Access-Control-Expose-Headers"), []string(nil))
		}
		rec, _ := serveCors(newCors(t, container.Bag{}), http.MethodGet, http.Header{})
		testutil.Equal(t, "any origin vary", rec.Header().Values("Vary"), []string(nil))
		rec, _ = serveCors(newCors(t, listed), http.MethodGet, http.Header{})
		testutil.Equal(t, "listed vary", rec.Header().Values("Vary"), []string{"Origin"})
	})

	t.Run("should allow any origin with the wildcard", func(t *testing.T) {
		rec, called := serveCors(newCors(t, container.Bag{}), http.MethodPost, http.Header{"Origin": {"https://any.example.org"}})
		testutil.Equal(t, "called", called, true)
		testutil.Equal(t, "body", rec.Body.String(), "ok")
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Access-Control-Allow-Origin":   {"*"},
			"Access-Control-Expose-Headers": {"X-Request-Id"},
			"Content-Type":                  {"text/plain; charset=utf-8"},
		})
	})

	t.Run("should allow a listed origin, whatever its case, with the credentials", func(t *testing.T) {
		m := newCors(t, listed)
		for _, origin := range []string{"https://app.example.com", "HTTPS://APP.EXAMPLE.COM", "https://admin.example.com"} {
			rec, called := serveCors(m, http.MethodDelete, http.Header{"Origin": {origin}})
			testutil.Equal(t, "called", called, true)
			testutil.Equal(t, "header", rec.Header(), http.Header{
				"Vary":                             {"Origin"},
				"Access-Control-Allow-Origin":      {origin},
				"Access-Control-Allow-Credentials": {"true"},
				"Access-Control-Expose-Headers":    {"X-Request-Id, X-Total"},
				"Content-Type":                     {"text/plain; charset=utf-8"},
			})
		}
	})

	t.Run("should serve a request of another origin without the CORS headers", func(t *testing.T) {
		rec, called := serveCors(newCors(t, listed), http.MethodGet, http.Header{"Origin": {"https://evil.example.com"}})
		testutil.Equal(t, "called", called, true)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Vary":         {"Origin"},
			"Content-Type": {"text/plain; charset=utf-8"},
		})
	})

	t.Run("should not expose headers when none is listed", func(t *testing.T) {
		m := newCors(t, container.Bag{cors.ConfigFieldExposedHeaders: []any{}, cors.ConfigFieldAllowedOrigins: []string{"https://a.b"}})
		rec, _ := serveCors(m, http.MethodGet, http.Header{"Origin": {"https://a.b"}})
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Vary":                        {"Origin"},
			"Access-Control-Allow-Origin": {"https://a.b"},
			"Content-Type":                {"text/plain; charset=utf-8"},
		})
	})

	t.Run("should answer an allowed preflight request itself", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields  container.Bag
			request http.Header
			want    http.Header
		}{
			"any origin": {
				fields:  container.Bag{},
				request: preflight("https://any.example.org", "PATCH", "content-type, X-Request-Id"),
				want: http.Header{
					"Vary":                         preflightVary,
					"Access-Control-Allow-Origin":  {"*"},
					"Access-Control-Allow-Methods": {"GET, HEAD, POST, PUT, PATCH, DELETE"},
					"Access-Control-Allow-Headers": {"content-type, X-Request-Id"},
					"Access-Control-Max-Age":       {"600"},
				},
			},
			"listed origin": {
				fields:  listed,
				request: preflight("https://APP.example.com", "PUT", "X-API-KEY"),
				want: http.Header{
					"Vary":                             preflightVary,
					"Access-Control-Allow-Origin":      {"https://APP.example.com"},
					"Access-Control-Allow-Credentials": {"true"},
					"Access-Control-Allow-Methods":     {"GET, PUT"},
					"Access-Control-Allow-Headers":     {"X-API-KEY"},
					"Access-Control-Max-Age":           {"90"},
				},
			},
			"no requested header": {
				fields:  listed,
				request: preflight("https://admin.example.com", "GET", ""),
				want: http.Header{
					"Vary":                             preflightVary,
					"Access-Control-Allow-Origin":      {"https://admin.example.com"},
					"Access-Control-Allow-Credentials": {"true"},
					"Access-Control-Allow-Methods":     {"GET, PUT"},
					"Access-Control-Max-Age":           {"90"},
				},
			},
			"requested headers over several values, empty names left out": {
				fields: listed,
				request: http.Header{
					"Origin":                         {"https://admin.example.com"},
					"Access-Control-Request-Method":  {"PUT"},
					"Access-Control-Request-Headers": {"x-api-key,, ", " Content-Type"},
				},
				want: http.Header{
					"Vary":                             preflightVary,
					"Access-Control-Allow-Origin":      {"https://admin.example.com"},
					"Access-Control-Allow-Credentials": {"true"},
					"Access-Control-Allow-Methods":     {"GET, PUT"},
					"Access-Control-Allow-Headers":     {"x-api-key, Content-Type"},
					"Access-Control-Max-Age":           {"90"},
				},
			},
			"any header, no max age": {
				fields:  container.Bag{cors.ConfigFieldAllowedHeaders: []any{"*"}, cors.ConfigFieldMaxAge: "0s"},
				request: preflight("https://a.b", "GET", "X-Anything"),
				want: http.Header{
					"Vary":                         preflightVary,
					"Access-Control-Allow-Origin":  {"*"},
					"Access-Control-Allow-Methods": {"GET, HEAD, POST, PUT, PATCH, DELETE"},
					"Access-Control-Allow-Headers": {"X-Anything"},
				},
			},
			"max age under a second": {
				fields:  container.Bag{cors.ConfigFieldMaxAge: "999ms"},
				request: preflight("https://a.b", "GET", ""),
				want: http.Header{
					"Vary":                         preflightVary,
					"Access-Control-Allow-Origin":  {"*"},
					"Access-Control-Allow-Methods": {"GET, HEAD, POST, PUT, PATCH, DELETE"},
				},
			},
		} {
			rec, called := serveCors(newCors(t, tc.fields), http.MethodOptions, tc.request)
			testutil.Equal(t, name+" called", called, false)
			testutil.Equal(t, name+" status", rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" body", rec.Body.String(), "")
			testutil.Equal(t, name+" header", rec.Header(), tc.want)
		}
	})

	t.Run("should answer a disallowed preflight request without the CORS headers", func(t *testing.T) {
		for name, request := range map[string]http.Header{
			"no origin":        {"Access-Control-Request-Method": {"GET"}},
			"other origin":     preflight("https://evil.example.com", "GET", ""),
			"other method":     preflight("https://app.example.com", "DELETE", ""),
			"lower method":     preflight("https://app.example.com", "put", ""),
			"other header":     preflight("https://app.example.com", "PUT", "Content-Type, X-Other"),
			"wildcard request": preflight("https://app.example.com", "PUT", "*"),
		} {
			rec, called := serveCors(newCors(t, listed), http.MethodOptions, request)
			testutil.Equal(t, name+" called", called, false)
			testutil.Equal(t, name+" status", rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" header", rec.Header(), http.Header{"Vary": preflightVary})
		}
	})

	t.Run("should serve an OPTIONS request without requested method through the handler", func(t *testing.T) {
		rec, called := serveCors(newCors(t, listed), http.MethodOptions, http.Header{"Origin": {"https://app.example.com"}})
		testutil.Equal(t, "called", called, true)
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "allow origin", rec.Header().Get("Access-Control-Allow-Origin"), "https://app.example.com")
	})

	t.Run("should answer the preflight requests reaching the unmatched handler of a router", func(t *testing.T) {
		m := newCors(t, container.Bag{})
		router, err := rest.NewRouter(3, m)
		testutil.NoError(t, err)
		testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: "GET /items/{id}"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		})))

		for _, path := range []string{"/items/1", "/unknown"} {
			req := httptest.NewRequest(http.MethodOptions, path, nil)
			req.Header = preflight("https://a.b", "GET", "Content-Type")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			testutil.Equal(t, path+" status", rec.Code, http.StatusNoContent)
			testutil.Equal(t, path+" allow origin", rec.Header().Get("Access-Control-Allow-Origin"), "*")
			testutil.Equal(t, path+" allow methods", rec.Header().Get("Access-Control-Allow-Methods"), "GET, HEAD, POST, PUT, PATCH, DELETE")
		}

		req := httptest.NewRequest(http.MethodOptions, "/items/1", nil)
		req.Header.Set("Origin", "https://a.b")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		testutil.Equal(t, "plain options status", rec.Code, http.StatusMethodNotAllowed)
		testutil.Equal(t, "plain options allow origin", rec.Header().Get("Access-Control-Allow-Origin"), "*")

		req = httptest.NewRequest(http.MethodGet, "/items/1", nil)
		req.Header.Set("Origin", "https://a.b")
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		testutil.Equal(t, "get status", rec.Code, http.StatusOK)
		testutil.Equal(t, "get body", rec.Body.String(), "ok")
		testutil.Equal(t, "get allow origin", rec.Header().Get("Access-Control-Allow-Origin"), "*")
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		m, err := createCors(t, container.Bag{
			cors.ConfigFieldAllowedOrigins: []string{"https://a.b"},
			cors.ConfigFieldAllowedMethods: []string{"POST"},
		}, map[string]any{
			cors.DefaultAllowedHeadersConfigPath:   []any{"X-One"},
			cors.DefaultExposedHeadersConfigPath:   []any{"X-Two"},
			cors.DefaultAllowCredentialsConfigPath: true,
			cors.DefaultMaxAgeConfigPath:           "1h",
		})
		testutil.NoError(t, err)
		rec, _ := serveCors(m, http.MethodOptions, preflight("https://a.b", "POST", "x-one"))
		testutil.Equal(t, "preflight", rec.Header(), http.Header{
			"Vary":                             preflightVary,
			"Access-Control-Allow-Origin":      {"https://a.b"},
			"Access-Control-Allow-Credentials": {"true"},
			"Access-Control-Allow-Methods":     {"POST"},
			"Access-Control-Allow-Headers":     {"x-one"},
			"Access-Control-Max-Age":           {"3600"},
		})
		rec, _ = serveCors(m, http.MethodGet, http.Header{"Origin": {"https://a.b"}})
		testutil.Equal(t, "expose", rec.Header().Get("Access-Control-Expose-Headers"), "X-Two")

		m, err = createCors(t, container.Bag{
			cors.ConfigFieldAllowedHeaders:   []string{"X-Three"},
			cors.ConfigFieldExposedHeaders:   []string{"*"},
			cors.ConfigFieldAllowCredentials: false,
			cors.ConfigFieldMaxAge:           "2s",
		}, map[string]any{
			cors.DefaultAllowedOriginsConfigPath: []any{"https://c.d"},
			cors.DefaultAllowedMethodsConfigPath: []any{"HEAD"},
		})
		testutil.NoError(t, err)
		rec, _ = serveCors(m, http.MethodOptions, preflight("https://c.d", "HEAD", "X-Three"))
		testutil.Equal(t, "second preflight", rec.Header(), http.Header{
			"Vary":                         preflightVary,
			"Access-Control-Allow-Origin":  {"https://c.d"},
			"Access-Control-Allow-Methods": {"HEAD"},
			"Access-Control-Allow-Headers": {"X-Three"},
			"Access-Control-Max-Age":       {"2"},
		})
		rec, _ = serveCors(m, http.MethodGet, http.Header{"Origin": {"https://c.d"}})
		testutil.Equal(t, "second expose", rec.Header().Get("Access-Control-Expose-Headers"), "*")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		m, err := create(t, bare{cors.NewBundle()}, cors.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		rec, _ := serveCors(m, http.MethodOptions, preflight("https://a.b", "DELETE", "Authorization, accept"))
		testutil.Equal(t, "preflight", rec.Header(), http.Header{
			"Vary":                         preflightVary,
			"Access-Control-Allow-Origin":  {"*"},
			"Access-Control-Allow-Methods": {"GET, HEAD, POST, PUT, PATCH, DELETE"},
			"Access-Control-Allow-Headers": {"Authorization, accept"},
			"Access-Control-Max-Age":       {"600"},
		})
		rec, _ = serveCors(m, http.MethodGet, http.Header{"Origin": {"https://a.b"}})
		testutil.Equal(t, "expose", rec.Header().Get("Access-Control-Expose-Headers"), "X-Request-Id")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{cors.ConfigFieldAllowedOrigins, "*"},
			{cors.ConfigFieldAllowedOrigins, []any{"https://a.b", 1}},
			{cors.ConfigFieldAllowedOrigins, []any{" "}},
			{cors.ConfigFieldAllowedMethods, []string{""}},
			{cors.ConfigFieldAllowedMethods, 1},
			{cors.ConfigFieldAllowedHeaders, []any{true}},
			{cors.ConfigFieldExposedHeaders, "X-Total"},
			{cors.ConfigFieldAllowCredentials, "true"},
			{cors.ConfigFieldMaxAge, -time.Second},
			{cors.ConfigFieldMaxAge, "soon"},
			{cors.ConfigFieldMaxAge, 600},
		} {
			_, err := createCors(t, container.Bag{tc.field: tc.value}, nil)
			testutil.Error(t, errors.Unwrap(err), cors.ErrInvalidField,
				"unable to create cors rest middleware: invalid cors rest middleware field",
				map[string]any{cors.ErrorFieldID: "main", cors.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		for path, field := range map[string]string{
			cors.DefaultAllowedOriginsConfigPath:   cors.ConfigFieldAllowedOrigins,
			cors.DefaultAllowedMethodsConfigPath:   cors.ConfigFieldAllowedMethods,
			cors.DefaultAllowedHeadersConfigPath:   cors.ConfigFieldAllowedHeaders,
			cors.DefaultExposedHeadersConfigPath:   cors.ConfigFieldExposedHeaders,
			cors.DefaultAllowCredentialsConfigPath: cors.ConfigFieldAllowCredentials,
			cors.DefaultMaxAgeConfigPath:           cors.ConfigFieldMaxAge,
		} {
			_, err := createCors(t, container.Bag{}, map[string]any{path: 42})
			testutil.Error(t, errors.Unwrap(err), cors.ErrInvalidField,
				"unable to create cors rest middleware: invalid cors rest middleware field",
				map[string]any{cors.ErrorFieldID: "main", cors.ErrorFieldField: field})
		}
	})

	t.Run("should fail with the credentials along with a wildcard", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields container.Bag
			field  string
		}{
			"default origin": {
				fields: container.Bag{cors.ConfigFieldAllowCredentials: true},
				field:  cors.ConfigFieldAllowedOrigins,
			},
			"listed origin": {
				fields: container.Bag{cors.ConfigFieldAllowCredentials: true, cors.ConfigFieldAllowedOrigins: []any{"https://a.b", " * "}},
				field:  cors.ConfigFieldAllowedOrigins,
			},
			"exposed header": {
				fields: container.Bag{
					cors.ConfigFieldAllowCredentials: true,
					cors.ConfigFieldAllowedOrigins:   []any{"https://a.b"},
					cors.ConfigFieldExposedHeaders:   []any{"*"},
				},
				field: cors.ConfigFieldExposedHeaders,
			},
		} {
			_, err := createCors(t, tc.fields, nil)
			testutil.Error(t, errors.Unwrap(err), cors.ErrWildcardCredentials,
				"unable to create cors rest middleware: cors credentials allowed along with a wildcard",
				map[string]any{cors.ErrorFieldID: "main", cors.ErrorFieldField: tc.field})
			testutil.Equal(t, name+" sentinel", errors.Is(err, cors.ErrWildcardCredentials), true)
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createCors(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
