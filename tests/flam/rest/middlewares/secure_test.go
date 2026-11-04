package middlewares

import (
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/secure"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createSecure returns the secure middleware "main" of the fields, created
// with the secure bundle and the values.
func createSecure(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, secure.NewBundle(), secure.Driver, fields, values, deps{})
}

// serveSecure serves a GET request through the middleware, over TLS when
// asked, with a handler answering 200 "ok" after calling change, if not
// nil, with its header, returning the recorded response.
func serveSecure(m flam.IRestMiddleware, overTLS bool, change func(http.Header)) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	if overTLS {
		req.TLS = &tls.ConnectionState{}
	}
	rec := httptest.NewRecorder()
	m.Wrap(logEndpoint(0), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if change != nil {
			change(w.Header())
		}
		_, _ = w.Write([]byte("ok"))
	})).ServeHTTP(rec, req)
	return rec
}

// secureDefaults are the headers the default secure middleware sets on
// the responses to the requests received without TLS.
func secureDefaults() http.Header {
	return http.Header{
		"X-Content-Type-Options": {"nosniff"},
		"X-Frame-Options":        {"DENY"},
		"Referrer-Policy":        {"no-referrer"},
		"Content-Type":           {"text/plain; charset=utf-8"},
	}
}

func Test_Secure_Bundle(t *testing.T) {
	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := secure.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.secure")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + secure.Id:       container.Bag{factory.ConfigFieldDriver: secure.Driver},
			secure.DefaultContentTypeOptionsConfigPath:        "nosniff",
			secure.DefaultFrameOptionsConfigPath:              "DENY",
			secure.DefaultReferrerPolicyConfigPath:            "no-referrer",
			secure.DefaultHSTSMaxAgeConfigPath:                "8760h0m0s",
			secure.DefaultHSTSIncludeSubdomainsConfigPath:     true,
			secure.DefaultHSTSPreloadConfigPath:               false,
			secure.DefaultForceHSTSConfigPath:                 false,
			secure.DefaultContentSecurityPolicyConfigPath:     "",
			secure.DefaultPermissionsPolicyConfigPath:         "",
			secure.DefaultCrossOriginOpenerPolicyConfigPath:   "",
			secure.DefaultCrossOriginEmbedderPolicyConfigPath: "",
			secure.DefaultCrossOriginResourcePolicyConfigPath: "",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		for got, want := range map[string]string{
			secure.Driver: "flam.rest.middlewares.driver.secure",
			secure.DefaultContentTypeOptionsConfigPath:        "flam.rest.defaults.middlewares.secure.content_type_options",
			secure.DefaultFrameOptionsConfigPath:              "flam.rest.defaults.middlewares.secure.frame_options",
			secure.DefaultReferrerPolicyConfigPath:            "flam.rest.defaults.middlewares.secure.referrer_policy",
			secure.DefaultHSTSMaxAgeConfigPath:                "flam.rest.defaults.middlewares.secure.hsts_max_age",
			secure.DefaultHSTSIncludeSubdomainsConfigPath:     "flam.rest.defaults.middlewares.secure.hsts_include_subdomains",
			secure.DefaultHSTSPreloadConfigPath:               "flam.rest.defaults.middlewares.secure.hsts_preload",
			secure.DefaultForceHSTSConfigPath:                 "flam.rest.defaults.middlewares.secure.force_hsts",
			secure.DefaultContentSecurityPolicyConfigPath:     "flam.rest.defaults.middlewares.secure.content_security_policy",
			secure.DefaultPermissionsPolicyConfigPath:         "flam.rest.defaults.middlewares.secure.permissions_policy",
			secure.DefaultCrossOriginOpenerPolicyConfigPath:   "flam.rest.defaults.middlewares.secure.cross_origin_opener_policy",
			secure.DefaultCrossOriginEmbedderPolicyConfigPath: "flam.rest.defaults.middlewares.secure.cross_origin_embedder_policy",
			secure.DefaultCrossOriginResourcePolicyConfigPath: "flam.rest.defaults.middlewares.secure.cross_origin_resource_policy",
		} {
			testutil.Equal(t, "const", got, want)
		}
		testutil.Equal(t, "Id", secure.Id, "secure")
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should register the creator", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(secure.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.rest.middlewares.secure", "provider/1"})
	})
}

func Test_Secure(t *testing.T) {
	t.Run("should set the default headers, HSTS only over TLS", func(t *testing.T) {
		m, err := createSecure(t, container.Bag{}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", m.Id(), "main")

		rec := serveSecure(m, false, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "body", rec.Body.String(), "ok")
		testutil.Equal(t, "header", rec.Header(), secureDefaults())

		rec = serveSecure(m, true, nil)
		want := secureDefaults()
		want.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		testutil.Equal(t, "tls header", rec.Header(), want)
	})

	t.Run("should let the handler change the headers", func(t *testing.T) {
		m, err := createSecure(t, container.Bag{}, nil)
		testutil.NoError(t, err)
		rec := serveSecure(m, true, func(h http.Header) {
			h.Set("X-Frame-Options", "SAMEORIGIN")
			h.Del("Strict-Transport-Security")
		})
		want := secureDefaults()
		want.Set("X-Frame-Options", "SAMEORIGIN")
		testutil.Equal(t, "header", rec.Header(), want)
	})

	t.Run("should set every configured header, forcing HSTS", func(t *testing.T) {
		m, err := createSecure(t, container.Bag{
			secure.ConfigFieldContentTypeOptions:        "",
			secure.ConfigFieldFrameOptions:              "SAMEORIGIN",
			secure.ConfigFieldReferrerPolicy:            "same-origin",
			secure.ConfigFieldHSTSMaxAge:                90*time.Second + 500*time.Millisecond,
			secure.ConfigFieldHSTSIncludeSubdomains:     false,
			secure.ConfigFieldHSTSPreload:               true,
			secure.ConfigFieldForceHSTS:                 true,
			secure.ConfigFieldContentSecurityPolicy:     "default-src 'none'",
			secure.ConfigFieldPermissionsPolicy:         "camera=()",
			secure.ConfigFieldCrossOriginOpenerPolicy:   "same-origin",
			secure.ConfigFieldCrossOriginEmbedderPolicy: "require-corp",
			secure.ConfigFieldCrossOriginResourcePolicy: "same-site",
		}, nil)
		testutil.NoError(t, err)
		rec := serveSecure(m, false, nil)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"X-Frame-Options":              {"SAMEORIGIN"},
			"Referrer-Policy":              {"same-origin"},
			"Strict-Transport-Security":    {"max-age=90; preload"},
			"Content-Security-Policy":      {"default-src 'none'"},
			"Permissions-Policy":           {"camera=()"},
			"Cross-Origin-Opener-Policy":   {"same-origin"},
			"Cross-Origin-Embedder-Policy": {"require-corp"},
			"Cross-Origin-Resource-Policy": {"same-site"},
			"Content-Type":                 {"text/plain; charset=utf-8"},
		})
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		m, err := createSecure(t, container.Bag{}, map[string]any{
			secure.DefaultContentTypeOptionsConfigPath:        "",
			secure.DefaultFrameOptionsConfigPath:              "",
			secure.DefaultReferrerPolicyConfigPath:            "",
			secure.DefaultHSTSMaxAgeConfigPath:                "1h",
			secure.DefaultHSTSIncludeSubdomainsConfigPath:     true,
			secure.DefaultHSTSPreloadConfigPath:               true,
			secure.DefaultForceHSTSConfigPath:                 true,
			secure.DefaultContentSecurityPolicyConfigPath:     "a",
			secure.DefaultPermissionsPolicyConfigPath:         "b",
			secure.DefaultCrossOriginOpenerPolicyConfigPath:   "c",
			secure.DefaultCrossOriginEmbedderPolicyConfigPath: "d",
			secure.DefaultCrossOriginResourcePolicyConfigPath: "e",
		})
		testutil.NoError(t, err)
		rec := serveSecure(m, false, nil)
		testutil.Equal(t, "header", rec.Header(), http.Header{
			"Strict-Transport-Security":    {"max-age=3600; includeSubDomains; preload"},
			"Content-Security-Policy":      {"a"},
			"Permissions-Policy":           {"b"},
			"Cross-Origin-Opener-Policy":   {"c"},
			"Cross-Origin-Embedder-Policy": {"d"},
			"Cross-Origin-Resource-Policy": {"e"},
			"Content-Type":                 {"text/plain; charset=utf-8"},
		})
	})

	t.Run("should leave HSTS out with a max age under a second", func(t *testing.T) {
		for _, maxAge := range []any{"0s", 999 * time.Millisecond} {
			m, err := createSecure(t, container.Bag{secure.ConfigFieldHSTSMaxAge: maxAge, secure.ConfigFieldForceHSTS: true}, nil)
			testutil.NoError(t, err)
			testutil.Equal(t, "header", serveSecure(m, true, nil).Header(), secureDefaults())
		}
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		m, err := create(t, bare{secure.NewBundle()}, secure.Driver, container.Bag{}, nil, deps{})
		testutil.NoError(t, err)
		want := secureDefaults()
		want.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		testutil.Equal(t, "header", serveSecure(m, true, nil).Header(), want)
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{secure.ConfigFieldContentTypeOptions, 1},
			{secure.ConfigFieldFrameOptions, "DENY\r\nX-Injected: 1"},
			{secure.ConfigFieldReferrerPolicy, "a\nb"},
			{secure.ConfigFieldContentSecurityPolicy, "a\x00"},
			{secure.ConfigFieldPermissionsPolicy, []string{"a"}},
			{secure.ConfigFieldCrossOriginOpenerPolicy, true},
			{secure.ConfigFieldCrossOriginEmbedderPolicy, 1.5},
			{secure.ConfigFieldCrossOriginResourcePolicy, nil},
			{secure.ConfigFieldHSTSMaxAge, -time.Second},
			{secure.ConfigFieldHSTSMaxAge, "soon"},
			{secure.ConfigFieldHSTSMaxAge, 3600},
			{secure.ConfigFieldHSTSIncludeSubdomains, "true"},
			{secure.ConfigFieldHSTSPreload, 1},
			{secure.ConfigFieldForceHSTS, "yes"},
		} {
			_, err := createSecure(t, container.Bag{tc.field: tc.value}, nil)
			testutil.Error(t, errors.Unwrap(err), secure.ErrInvalidField,
				"unable to create secure rest middleware: invalid secure rest middleware field",
				map[string]any{secure.ErrorFieldID: "main", secure.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail with an invalid default", func(t *testing.T) {
		for path, field := range map[string]string{
			secure.DefaultContentTypeOptionsConfigPath:        secure.ConfigFieldContentTypeOptions,
			secure.DefaultFrameOptionsConfigPath:              secure.ConfigFieldFrameOptions,
			secure.DefaultReferrerPolicyConfigPath:            secure.ConfigFieldReferrerPolicy,
			secure.DefaultHSTSMaxAgeConfigPath:                secure.ConfigFieldHSTSMaxAge,
			secure.DefaultHSTSIncludeSubdomainsConfigPath:     secure.ConfigFieldHSTSIncludeSubdomains,
			secure.DefaultHSTSPreloadConfigPath:               secure.ConfigFieldHSTSPreload,
			secure.DefaultForceHSTSConfigPath:                 secure.ConfigFieldForceHSTS,
			secure.DefaultContentSecurityPolicyConfigPath:     secure.ConfigFieldContentSecurityPolicy,
			secure.DefaultPermissionsPolicyConfigPath:         secure.ConfigFieldPermissionsPolicy,
			secure.DefaultCrossOriginOpenerPolicyConfigPath:   secure.ConfigFieldCrossOriginOpenerPolicy,
			secure.DefaultCrossOriginEmbedderPolicyConfigPath: secure.ConfigFieldCrossOriginEmbedderPolicy,
			secure.DefaultCrossOriginResourcePolicyConfigPath: secure.ConfigFieldCrossOriginResourcePolicy,
		} {
			_, err := createSecure(t, container.Bag{}, map[string]any{path: 42})
			testutil.Error(t, errors.Unwrap(err), secure.ErrInvalidField,
				"unable to create secure rest middleware: invalid secure rest middleware field",
				map[string]any{secure.ErrorFieldID: "main", secure.ErrorFieldField: field})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := createSecure(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
