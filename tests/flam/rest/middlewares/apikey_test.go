package middlewares

import (
	"errors"
	"net/http"
	"sync"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/apikey"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createApikey returns the apikey middleware "main" of the fields,
// created with the apikey bundle and the values.
func createApikey(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, apikey.NewBundle(), apikey.Driver, fields, values, deps{})
}

func Test_Apikey_Bundle(t *testing.T) {
	t.Run("should configure the apikey middleware", func(t *testing.T) {
		b := apikey.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.apikey")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + apikey.Id: container.Bag{factory.ConfigFieldDriver: apikey.Driver},
			apikey.DefaultHeaderConfigPath:              "X-Api-Key",
			apikey.DefaultQueryConfigPath:               "",
			apikey.DefaultOptionalConfigPath:            false,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", apikey.Id, "apikey")
		testutil.Equal(t, "Driver", apikey.Driver, "flam.rest.middlewares.driver.apikey")
		testutil.Equal(t, "Challenge", apikey.Challenge, "ApiKey")
		for got, want := range map[string]string{
			apikey.DefaultHeaderConfigPath:   "flam.rest.defaults.middlewares.apikey.header",
			apikey.DefaultQueryConfigPath:    "flam.rest.defaults.middlewares.apikey.query",
			apikey.DefaultOptionalConfigPath: "flam.rest.defaults.middlewares.apikey.optional",
		} {
			testutil.Equal(t, "path", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Apikey_Create(t *testing.T) {
	t.Run("should create the middleware of each key form", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			"strings":         {apikey.ConfigFieldKeys: []string{"a", "b"}},
			"any list":        {apikey.ConfigFieldKeys: []any{"a"}},
			"bag":             {apikey.ConfigFieldKeys: container.Bag{"alice": "a", "bob": []any{"b", "c"}}},
			"map":             {apikey.ConfigFieldKeys: map[string]any{"alice": []string{"a"}}},
			"query only":      {apikey.ConfigFieldKeys: []any{"a"}, apikey.ConfigFieldHeader: "", apikey.ConfigFieldQuery: "api_key"},
			"empty principal": {apikey.ConfigFieldKeys: container.Bag{"alice": "a", "bob": []any{}}},
		} {
			m, err := createApikey(t, fields, nil)
			if err != nil {
				t.Fatalf("%s: error = %v", name, err)
			}
			testutil.Equal(t, name+" Id()", m.Id(), "main")
		}
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		for name, tc := range map[string]struct {
			fields container.Bag
			values map[string]any
			field  string
		}{
			"header":           {container.Bag{apikey.ConfigFieldKeys: []any{"a"}, apikey.ConfigFieldHeader: 1}, nil, apikey.ConfigFieldHeader},
			"query":            {container.Bag{apikey.ConfigFieldKeys: []any{"a"}, apikey.ConfigFieldQuery: 1}, nil, apikey.ConfigFieldQuery},
			"no source":        {container.Bag{apikey.ConfigFieldKeys: []any{"a"}, apikey.ConfigFieldHeader: ""}, nil, apikey.ConfigFieldQuery},
			"no keys":          {container.Bag{}, nil, apikey.ConfigFieldKeys},
			"empty keys":       {container.Bag{apikey.ConfigFieldKeys: []any{}}, nil, apikey.ConfigFieldKeys},
			"empty map":        {container.Bag{apikey.ConfigFieldKeys: container.Bag{}}, nil, apikey.ConfigFieldKeys},
			"key string":       {container.Bag{apikey.ConfigFieldKeys: "a"}, nil, apikey.ConfigFieldKeys},
			"key number":       {container.Bag{apikey.ConfigFieldKeys: []any{"a", 1}}, nil, apikey.ConfigFieldKeys},
			"empty key":        {container.Bag{apikey.ConfigFieldKeys: []any{"a", ""}}, nil, apikey.ConfigFieldKeys},
			"duplicate key":    {container.Bag{apikey.ConfigFieldKeys: []any{"a", "a"}}, nil, apikey.ConfigFieldKeys},
			"duplicate shared": {container.Bag{apikey.ConfigFieldKeys: container.Bag{"alice": "a", "bob": "a"}}, nil, apikey.ConfigFieldKeys},
			"principal value":  {container.Bag{apikey.ConfigFieldKeys: container.Bag{"alice": 1}}, nil, apikey.ConfigFieldKeys},
			"empty map key":    {container.Bag{apikey.ConfigFieldKeys: map[string]any{"alice": ""}}, nil, apikey.ConfigFieldKeys},
			"optional":         {container.Bag{apikey.ConfigFieldKeys: []any{"a"}}, map[string]any{apikey.DefaultOptionalConfigPath: "no"}, apikey.ConfigFieldOptional},
		} {
			_, err := createApikey(t, tc.fields, tc.values)
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			testutil.Error(t, errors.Unwrap(err), apikey.ErrInvalidField,
				"unable to create apikey rest middleware: invalid apikey rest middleware field",
				map[string]any{apikey.ErrorFieldID: "main", apikey.ErrorFieldField: tc.field})
		}
	})
}

func Test_Apikey(t *testing.T) {
	keys := container.Bag{"alice": "alice-key", "bob": []any{"bob-old", "bob-new"}}

	t.Run("should serve a request of a known key with its principal", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys, apikey.ConfigFieldQuery: "api_key"}, nil)
		testutil.NoError(t, err)
		for name, tc := range map[string]struct {
			req       *http.Request
			principal string
		}{
			"header":           {authRequest("/items/1", "X-Api-Key", "alice-key"), "alice"},
			"header case":      {authRequest("/items/1", "x-api-key", "bob-old"), "bob"},
			"rotated":          {authRequest("/items/1", "X-Api-Key", "bob-new"), "bob"},
			"query":            {authRequest("/items/1?api_key=alice-key"), "alice"},
			"header first":     {authRequest("/items/1?api_key=wrong", "X-Api-Key", "bob-new"), "bob"},
			"empty header":     {authRequest("/items/1?api_key=alice-key", "X-Api-Key", ""), "alice"},
			"first of headers": {authRequest("/items/1", "X-Api-Key", "alice-key", "X-Api-Key", "bob-new"), "alice"},
		} {
			result := authServe(t, tc.req, m)
			testutil.Equal(t, name+" status", result.rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" served", result.served, true)
			testutil.Equal(t, name+" principal", result.principal, tc.principal)
			testutil.Equal(t, name+" has principal", result.hasPrinc, true)
		}
	})

	t.Run("should serve a key of a list without principal", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: []any{"k1", "k2"}}, nil)
		testutil.NoError(t, err)
		result := authServe(t, authRequest("/items/1", "X-Api-Key", "k2"), m)
		testutil.Equal(t, "served", result.served, true)
		testutil.Equal(t, "has principal", result.hasPrinc, false)
	})

	t.Run("should read the configured header only", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys}, map[string]any{apikey.DefaultHeaderConfigPath: "X-Token"})
		testutil.NoError(t, err)
		testutil.Equal(t, "served", authServe(t, authRequest("/items/1", "X-Token", "alice-key"), m).served, true)
		assertProblem(t, authServe(t, authRequest("/items/1", "X-Api-Key", "alice-key"), m), http.StatusUnauthorized, "missing api key", "WWW-Authenticate", apikey.Challenge)
		assertProblem(t, authServe(t, authRequest("/items/1?api_key=alice-key"), m), http.StatusUnauthorized, "missing api key", "WWW-Authenticate", apikey.Challenge)
	})

	t.Run("should read the query only when the header is unset", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys, apikey.ConfigFieldHeader: "", apikey.ConfigFieldQuery: "key"}, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "served", authServe(t, authRequest("/items/1?key=alice-key"), m).served, true)
		assertProblem(t, authServe(t, authRequest("/items/1", "X-Api-Key", "alice-key"), m), http.StatusUnauthorized, "missing api key", "WWW-Authenticate", apikey.Challenge)
	})

	t.Run("should answer a request without key with a 401", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys, apikey.ConfigFieldQuery: "api_key"}, nil)
		testutil.NoError(t, err)
		for name, req := range map[string]*http.Request{
			"none":         authRequest("/items/1"),
			"empty header": authRequest("/items/1", "X-Api-Key", ""),
			"empty query":  authRequest("/items/1?api_key="),
		} {
			t.Run(name, func(t *testing.T) {
				assertProblem(t, authServe(t, req, m), http.StatusUnauthorized, "missing api key", "WWW-Authenticate", apikey.Challenge)
			})
		}
	})

	t.Run("should answer an unknown key with a 401", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys, apikey.ConfigFieldQuery: "api_key"}, nil)
		testutil.NoError(t, err)
		for name, req := range map[string]*http.Request{
			"unknown":    authRequest("/items/1", "X-Api-Key", "mallory"),
			"prefix":     authRequest("/items/1", "X-Api-Key", "alice-ke"),
			"longer":     authRequest("/items/1", "X-Api-Key", "alice-key "),
			"case":       authRequest("/items/1", "X-Api-Key", "ALICE-KEY"),
			"name":       authRequest("/items/1", "X-Api-Key", "alice"),
			"query":      authRequest("/items/1?api_key=mallory"),
			"bad header": authRequest("/items/1?api_key=alice-key", "X-Api-Key", "mallory"),
		} {
			t.Run(name, func(t *testing.T) {
				assertProblem(t, authServe(t, req, m), http.StatusUnauthorized, "invalid api key", "WWW-Authenticate", apikey.Challenge)
			})
		}
	})

	t.Run("should serve a request without key when optional, but not an unknown key", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys, apikey.ConfigFieldOptional: true}, nil)
		testutil.NoError(t, err)
		result := authServe(t, authRequest("/items/1"), m)
		testutil.Equal(t, "served", result.served, true)
		testutil.Equal(t, "has principal", result.hasPrinc, false)
		assertProblem(t, authServe(t, authRequest("/items/1", "X-Api-Key", "mallory"), m), http.StatusUnauthorized, "invalid api key", "WWW-Authenticate", apikey.Challenge)
	})

	t.Run("should be used concurrently", func(t *testing.T) {
		m, err := createApikey(t, container.Bag{apikey.ConfigFieldKeys: keys}, nil)
		testutil.NoError(t, err)
		var wg sync.WaitGroup
		for _, key := range []string{"alice-key", "bob-old", "bob-new", "alice-key"} {
			wg.Go(func() {
				testutil.Equal(t, key+" served", authServe(t, authRequest("/items/1", "X-Api-Key", key), m).served, true)
			})
		}
		wg.Wait()
	})
}
