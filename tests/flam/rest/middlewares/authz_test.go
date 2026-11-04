package middlewares

import (
	"errors"
	"net/http"
	"testing"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/apikey"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/authz"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// createAuthz returns the authz middleware "main" of the fields, created
// with the authz bundle and the values.
func createAuthz(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, authz.NewBundle(), authz.Driver, fields, values, deps{})
}

// authzAuth returns the optional jwt and apikey middlewares authenticating
// the requests of the authz tests, the keys "alice-key" and "ci-key"
// being of the principals "alice" and "ci".
func authzAuth(t *testing.T) []flam.IRestMiddleware {
	t.Helper()
	j, err := createJwt(t, container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldOptional: true}, nil)
	testutil.NoError(t, err)
	k, err := create(t, apikey.NewBundle(), apikey.Driver, container.Bag{
		apikey.ConfigFieldKeys:     container.Bag{"alice": "alice-key", "ci": "ci-key"},
		apikey.ConfigFieldOptional: true,
	}, nil, deps{})
	testutil.NoError(t, err)
	return []flam.IRestMiddleware{j, k}
}

// authzToken returns the bearer request of the token of the subject and
// the extra claims.
func authzToken(t *testing.T, subject any, extra gojwt.MapClaims) *http.Request {
	t.Helper()
	token := jwtSign(t, gojwt.SigningMethodHS256, []byte(jwtSecret), jwtClaims(subject, extra))
	return authRequest("/items/1", "Authorization", "Bearer "+token)
}

func Test_Authz_Bundle(t *testing.T) {
	t.Run("should configure the authz middleware", func(t *testing.T) {
		b := authz.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.authz")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + authz.Id: container.Bag{factory.ConfigFieldDriver: authz.Driver},
			authz.DefaultPolicyConfigPath:              "deny",
			authz.DefaultRolesClaimConfigPath:          "roles",
			authz.DefaultChallengeConfigPath:           "Bearer",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", authz.Id, "authz")
		testutil.Equal(t, "Driver", authz.Driver, "flam.rest.middlewares.driver.authz")
		for got, want := range map[string]string{
			authz.DefaultPolicyConfigPath:     "flam.rest.defaults.middlewares.authz.policy",
			authz.DefaultRolesClaimConfigPath: "flam.rest.defaults.middlewares.authz.roles_claim",
			authz.DefaultChallengeConfigPath:  "flam.rest.defaults.middlewares.authz.challenge",
			authz.PolicyDeny:                  "deny",
			authz.PolicyAuthenticated:         "authenticated",
		} {
			testutil.Equal(t, "constant", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should create the authz middleware through the aggregate bundle", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(epoch) }))
		bag := container.Bag{}
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), authz.NewBundle()} {
			if configurable, ok := b.(flam.IConfigurableBundle); ok {
				testutil.NoError(t, configurable.Config(bag))
			}
		}
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), middlewares.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) {
			m, err := f.Get(authz.Id)
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", m.Id(), authz.Id)
		}))
	})
}

func Test_Authz_Create(t *testing.T) {
	t.Run("should create the middleware of valid fields", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			"defaults":      {},
			"no rules":      {authz.ConfigFieldRules: nil},
			"empty rules":   {authz.ConfigFieldRules: []any{}},
			"authenticated": {authz.ConfigFieldPolicy: authz.PolicyAuthenticated},
			"rules": {authz.ConfigFieldRules: []any{
				container.Bag{authz.RuleFieldEndpoints: []int{1, 2}, authz.RuleFieldScopes: []string{"a"}},
				map[string]any{authz.RuleFieldEndpoints: []any{3}, authz.RuleFieldPatterns: []any{"GET /x/*"}, authz.RuleFieldRoles: []any{"r"}},
				container.Bag{authz.RuleFieldPatterns: []string{"GET /health"}, authz.RuleFieldPublic: true},
				container.Bag{authz.RuleFieldPatterns: []string{"GET /y"}, authz.RuleFieldPublic: false},
			}},
			"roles claim":     {authz.ConfigFieldRolesClaim: "realm_access.roles"},
			"no roles claim":  {authz.ConfigFieldRolesClaim: ""},
			"principals bag":  {authz.ConfigFieldPrincipals: container.Bag{"alice": "admin", "ci": []any{"deployer", "reader"}}},
			"principals map":  {authz.ConfigFieldPrincipals: map[string]any{"ci": []string{"deployer"}}},
			"no principals":   {authz.ConfigFieldPrincipals: nil},
			"empty challenge": {authz.ConfigFieldChallenge: ""},
		} {
			m, err := createAuthz(t, fields, nil)
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
			"policy unknown":      {container.Bag{authz.ConfigFieldPolicy: "allow"}, nil, authz.ConfigFieldPolicy},
			"policy type":         {container.Bag{authz.ConfigFieldPolicy: true}, nil, authz.ConfigFieldPolicy},
			"policy default":      {container.Bag{}, map[string]any{authz.DefaultPolicyConfigPath: "open"}, authz.ConfigFieldPolicy},
			"rules type":          {container.Bag{authz.ConfigFieldRules: container.Bag{"a": 1}}, nil, authz.ConfigFieldRules},
			"roles claim type":    {container.Bag{authz.ConfigFieldRolesClaim: 1}, nil, authz.ConfigFieldRolesClaim},
			"roles claim segment": {container.Bag{authz.ConfigFieldRolesClaim: "realm_access..roles"}, nil, authz.ConfigFieldRolesClaim},
			"roles claim dot":     {container.Bag{authz.ConfigFieldRolesClaim: "roles."}, nil, authz.ConfigFieldRolesClaim},
			"principals type":     {container.Bag{authz.ConfigFieldPrincipals: []any{"alice"}}, nil, authz.ConfigFieldPrincipals},
			"principal role":      {container.Bag{authz.ConfigFieldPrincipals: container.Bag{"alice": 1}}, nil, authz.ConfigFieldPrincipals},
			"principal empty":     {container.Bag{authz.ConfigFieldPrincipals: container.Bag{"alice": ""}}, nil, authz.ConfigFieldPrincipals},
			"principal list":      {container.Bag{authz.ConfigFieldPrincipals: container.Bag{"alice": []any{"admin", ""}}}, nil, authz.ConfigFieldPrincipals},
			"challenge type":      {container.Bag{authz.ConfigFieldChallenge: 1}, nil, authz.ConfigFieldChallenge},
		} {
			_, err := createAuthz(t, tc.fields, tc.values)
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			testutil.Error(t, errors.Unwrap(err), authz.ErrInvalidField,
				"unable to create authz rest middleware: invalid authz rest middleware field",
				map[string]any{authz.ErrorFieldID: "main", authz.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail on an invalid rule", func(t *testing.T) {
		valid := container.Bag{authz.RuleFieldEndpoints: []any{7}}
		for name, tc := range map[string]struct {
			rule  any
			field string
		}{
			"not a map":          {"rule", authz.ConfigFieldRules},
			"endpoints type":     {container.Bag{authz.RuleFieldEndpoints: "7"}, authz.RuleFieldEndpoints},
			"endpoint type":      {container.Bag{authz.RuleFieldEndpoints: []any{7, "8"}}, authz.RuleFieldEndpoints},
			"no endpoint":        {container.Bag{authz.RuleFieldScopes: []any{"a"}}, authz.RuleFieldEndpoints},
			"empty endpoints":    {container.Bag{authz.RuleFieldEndpoints: []any{}, authz.RuleFieldPatterns: []any{}}, authz.RuleFieldEndpoints},
			"patterns type":      {container.Bag{authz.RuleFieldPatterns: "GET /x"}, authz.RuleFieldPatterns},
			"empty pattern":      {container.Bag{authz.RuleFieldPatterns: []any{""}}, authz.RuleFieldPatterns},
			"malformed pattern":  {container.Bag{authz.RuleFieldPatterns: []any{"GET /x", "GET /[a"}}, authz.RuleFieldPatterns},
			"scopes type":        {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldScopes: "a b"}, authz.RuleFieldScopes},
			"empty scope":        {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldScopes: []any{""}}, authz.RuleFieldScopes},
			"roles type":         {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldRoles: []any{1}}, authz.RuleFieldRoles},
			"empty role":         {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldRoles: []any{""}}, authz.RuleFieldRoles},
			"public type":        {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldPublic: "yes"}, authz.RuleFieldPublic},
			"public with scopes": {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldPublic: true, authz.RuleFieldScopes: []any{"a"}}, authz.RuleFieldPublic},
			"public with roles":  {container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldPublic: true, authz.RuleFieldRoles: []any{"r"}}, authz.RuleFieldPublic},
		} {
			_, err := createAuthz(t, container.Bag{authz.ConfigFieldRules: []any{valid, tc.rule}}, nil)
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			testutil.Error(t, errors.Unwrap(err), authz.ErrInvalidField,
				"unable to create authz rest middleware: invalid authz rest middleware field",
				map[string]any{authz.ErrorFieldID: "main", authz.ErrorFieldField: tc.field, authz.ErrorFieldRule: 1})
		}
	})
}

func Test_Authz(t *testing.T) {
	auth := authzAuth(t)
	authorize := func(t *testing.T, fields container.Bag, req *http.Request) authResult {
		t.Helper()
		m, err := createAuthz(t, fields, nil)
		testutil.NoError(t, err)
		return authServe(t, req, append(append([]flam.IRestMiddleware(nil), auth...), m)...)
	}
	apiKey := func(key string) *http.Request {
		return authRequest("/items/1", "X-Api-Key", key)
	}
	anonymous := func() *http.Request {
		return authRequest("/items/1")
	}
	rules := func(rule container.Bag) container.Bag {
		return container.Bag{authz.ConfigFieldRules: []any{rule}}
	}

	t.Run("should deny the endpoints no rule matches by default", func(t *testing.T) {
		for name, req := range map[string]*http.Request{
			"anonymous": anonymous(),
			"token":     authzToken(t, "alice", nil),
			"api key":   apiKey("alice-key"),
		} {
			t.Run(name, func(t *testing.T) {
				result := authorize(t, rules(container.Bag{authz.RuleFieldEndpoints: []any{8}}), req)
				assertProblem(t, result, http.StatusForbidden, "insufficient permissions", "WWW-Authenticate", "")
			})
		}
	})

	t.Run("should serve the authenticated requests of the endpoints no rule matches when the policy allows them", func(t *testing.T) {
		fields := container.Bag{authz.ConfigFieldPolicy: authz.PolicyAuthenticated}
		for name, req := range map[string]*http.Request{
			"token":            authzToken(t, "alice", nil),
			"token no subject": authzToken(t, nil, nil),
			"api key":          apiKey("ci-key"),
		} {
			result := authorize(t, fields, req)
			testutil.Equal(t, name+" status", result.rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" served", result.served, true)
		}
		assertProblem(t, authorize(t, fields, anonymous()), http.StatusUnauthorized, "authentication required", "WWW-Authenticate", "Bearer")
	})

	t.Run("should answer an unauthenticated request with the challenge", func(t *testing.T) {
		rule := container.Bag{authz.RuleFieldEndpoints: []any{7}}
		for name, tc := range map[string]struct {
			challenge any
			want      string
		}{
			"default": {nil, "Bearer"},
			"custom":  {"ApiKey", "ApiKey"},
			"none":    {"", ""},
		} {
			fields := rules(rule)
			if tc.challenge != nil {
				fields[authz.ConfigFieldChallenge] = tc.challenge
			}
			t.Run(name, func(t *testing.T) {
				result := authorize(t, fields, anonymous())
				assertProblem(t, result, http.StatusUnauthorized, "authentication required", "WWW-Authenticate", tc.want)
				_, has := result.rec.Header()["Www-Authenticate"]
				testutil.Equal(t, "has header", has, tc.want != "")
			})
		}
	})

	t.Run("should serve every request of a public rule", func(t *testing.T) {
		fields := rules(container.Bag{authz.RuleFieldPatterns: []any{"GET /items/*"}, authz.RuleFieldPublic: true})
		for name, req := range map[string]*http.Request{
			"anonymous": anonymous(),
			"token":     authzToken(t, "alice", nil),
		} {
			testutil.Equal(t, name+" served", authorize(t, fields, req).served, true)
		}
	})

	t.Run("should apply the first rule matching the endpoint", func(t *testing.T) {
		for name, tc := range map[string]struct {
			rules  []any
			status int
		}{
			"id then public": {[]any{
				container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldRoles: []any{"admin"}},
				container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldPublic: true},
			}, http.StatusForbidden},
			"pattern then id": {[]any{
				container.Bag{authz.RuleFieldPatterns: []any{"POST /items/*", "GET /items/*"}, authz.RuleFieldPublic: true},
				container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldRoles: []any{"admin"}},
			}, http.StatusNoContent},
			"other id and pattern": {[]any{
				container.Bag{authz.RuleFieldEndpoints: []any{8}, authz.RuleFieldPatterns: []any{"GET /items", "GET /*"}, authz.RuleFieldPublic: true},
				container.Bag{authz.RuleFieldEndpoints: []int{6, 7}, authz.RuleFieldRoles: []any{"admin"}},
			}, http.StatusForbidden},
		} {
			result := authorize(t, container.Bag{authz.ConfigFieldRules: tc.rules}, authzToken(t, "alice", nil))
			testutil.Equal(t, name+" status", result.rec.Code, tc.status)
		}
	})

	t.Run("should require every scope of the rule", func(t *testing.T) {
		fields := rules(container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldScopes: []any{"items:read", "items:list"}})
		for name, tc := range map[string]struct {
			claims gojwt.MapClaims
			served bool
		}{
			"scope":         {gojwt.MapClaims{"scope": "openid items:read  items:list"}, true},
			"scp list":      {gojwt.MapClaims{"scp": []any{"items:list", 1, "items:read"}}, true},
			"scp string":    {gojwt.MapClaims{"scp": "items:read items:list"}, true},
			"scope and scp": {gojwt.MapClaims{"scope": "items:read", "scp": []any{"items:list"}}, true},
			"missing one":   {gojwt.MapClaims{"scope": "items:read"}, false},
			"prefix":        {gojwt.MapClaims{"scope": "items:read items:lis"}, false},
			"scope list":    {gojwt.MapClaims{"scope": map[string]any{"items:read": true, "items:list": true}}, false},
			"none":          {nil, false},
		} {
			t.Run(name, func(t *testing.T) {
				result := authorize(t, fields, authzToken(t, "alice", tc.claims))
				if tc.served {
					testutil.Equal(t, "served", result.served, true)
					return
				}
				assertProblem(t, result, http.StatusForbidden, "insufficient permissions", "WWW-Authenticate", "")
			})
		}
		assertProblem(t, authorize(t, fields, apiKey("alice-key")), http.StatusForbidden, "insufficient permissions", "WWW-Authenticate", "")
	})

	t.Run("should require one of the roles of the rule", func(t *testing.T) {
		rule := container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldRoles: []any{"admin", "editor"}}
		principals := container.Bag{"ci": []any{"deployer", "editor"}, "alice": "viewer"}
		for name, tc := range map[string]struct {
			claim  any
			req    *http.Request
			served bool
		}{
			"roles list":           {nil, authzToken(t, "bob", gojwt.MapClaims{"roles": []any{"viewer", "editor"}}), true},
			"roles string":         {nil, authzToken(t, "bob", gojwt.MapClaims{"roles": "viewer admin"}), true},
			"roles other":          {nil, authzToken(t, "bob", gojwt.MapClaims{"roles": []any{"viewer", true}}), false},
			"roles missing":        {nil, authzToken(t, "bob", nil), false},
			"nested":               {"realm_access.roles", authzToken(t, "bob", gojwt.MapClaims{"realm_access": map[string]any{"roles": []any{"admin"}}}), true},
			"nested not an object": {"realm_access.roles", authzToken(t, "bob", gojwt.MapClaims{"realm_access": []any{"admin"}}), false},
			"nested top level":     {"realm_access.roles", authzToken(t, "bob", gojwt.MapClaims{"roles": []any{"admin"}}), false},
			"no claim":             {"", authzToken(t, "bob", gojwt.MapClaims{"roles": []any{"admin"}}), false},
			"principal of token":   {"", authzToken(t, "ci", nil), true},
			"principal of key":     {nil, apiKey("ci-key"), true},
			"other principal":      {nil, apiKey("alice-key"), false},
			"token and principal":  {nil, authzToken(t, "alice", gojwt.MapClaims{"roles": []any{"admin"}}), true},
		} {
			fields := rules(rule)
			fields[authz.ConfigFieldPrincipals] = principals
			if tc.claim != nil {
				fields[authz.ConfigFieldRolesClaim] = tc.claim
			}
			t.Run(name, func(t *testing.T) {
				result := authorize(t, fields, tc.req)
				if tc.served {
					testutil.Equal(t, "served", result.served, true)
					return
				}
				assertProblem(t, result, http.StatusForbidden, "insufficient permissions", "WWW-Authenticate", "")
			})
		}
		assertProblem(t, authorize(t, rules(rule), anonymous()), http.StatusUnauthorized, "authentication required", "WWW-Authenticate", "Bearer")
	})

	t.Run("should require both the scopes and a role of the rule", func(t *testing.T) {
		fields := rules(container.Bag{authz.RuleFieldEndpoints: []any{7}, authz.RuleFieldScopes: []any{"items:write"}, authz.RuleFieldRoles: []any{"admin"}})
		for name, tc := range map[string]struct {
			claims gojwt.MapClaims
			status int
		}{
			"both":       {gojwt.MapClaims{"scope": "items:write", "roles": []any{"admin"}}, http.StatusNoContent},
			"scope only": {gojwt.MapClaims{"scope": "items:write"}, http.StatusForbidden},
			"role only":  {gojwt.MapClaims{"roles": []any{"admin"}}, http.StatusForbidden},
		} {
			testutil.Equal(t, name+" status", authorize(t, fields, authzToken(t, "alice", tc.claims)).rec.Code, tc.status)
		}
	})
}
