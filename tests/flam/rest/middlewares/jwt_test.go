package middlewares

import (
	"context"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/apikey"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/ratelimit"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// jwtSecret is the HS256 secret of the jwt tests, 32 bytes long.
const jwtSecret = "0123456789abcdef0123456789abcdef"

// jwtKeys are the keys of the jwt tests, generated once.
var jwtKeys = sync.OnceValue(func() (keys struct {
	rsa     *rsa.PrivateKey
	rsaWeak *rsa.PrivateKey
	p256    *ecdsa.PrivateKey
	p384    *ecdsa.PrivateKey
	ed      ed25519.PrivateKey
	x25519  *ecdh.PrivateKey
},
) {
	var err error
	if keys.rsa, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		panic(err)
	}
	if keys.rsaWeak, err = rsa.GenerateKey(rand.Reader, 1024); err != nil {
		panic(err)
	}
	if keys.p256, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
		panic(err)
	}
	if keys.p384, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader); err != nil {
		panic(err)
	}
	if _, keys.ed, err = ed25519.GenerateKey(rand.Reader); err != nil {
		panic(err)
	}
	if keys.x25519, err = ecdh.X25519().GenerateKey(rand.Reader); err != nil {
		panic(err)
	}
	return keys
})

// jwtPEM returns the PEM block of the type and the bytes.
func jwtPEM(kind string, data []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: kind, Bytes: data}))
}

// jwtPKIX returns the "PUBLIC KEY" PEM of the public key.
func jwtPKIX(t *testing.T, public any) string {
	t.Helper()
	data, err := x509.MarshalPKIXPublicKey(public)
	testutil.NoError(t, err)
	return jwtPEM("PUBLIC KEY", data)
}

// jwtCertificate returns the "CERTIFICATE" PEM of a certificate of the
// public key, self signed with the private key.
func jwtCertificate(t *testing.T, public any, private any) string {
	t.Helper()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "issuer"},
		NotBefore:    epoch,
		NotAfter:     epoch.Add(time.Hour),
	}
	data, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	testutil.NoError(t, err)
	return jwtPEM("CERTIFICATE", data)
}

// jwtSign returns the token of the claims signed with the method and the
// key.
func jwtSign(t *testing.T, method gojwt.SigningMethod, key any, claims gojwt.MapClaims) string {
	t.Helper()
	token, err := gojwt.NewWithClaims(method, claims).SignedString(key)
	testutil.NoError(t, err)
	return token
}

// jwtClaims returns the claims of the subject, issued by "issuer" for
// "audience", expiring an hour after the epoch, extended by the extra
// claims, a nil subject or extra claim being removed.
func jwtClaims(subject any, extra gojwt.MapClaims) gojwt.MapClaims {
	claims := gojwt.MapClaims{
		"sub": subject,
		"iss": "issuer",
		"aud": "audience",
		"exp": epoch.Add(time.Hour).Unix(),
	}
	if subject == nil {
		delete(claims, "sub")
	}
	for name, value := range extra {
		if value == nil {
			delete(claims, name)
			continue
		}
		claims[name] = value
	}
	return claims
}

// createJwt returns the jwt middleware "main" of the fields, created with
// the jwt bundle, the values and a clock of the epoch.
func createJwt(t *testing.T, fields container.Bag, values map[string]any) (flam.IRestMiddleware, error) {
	t.Helper()
	return create(t, jwt.NewBundle(), jwt.Driver, fields, values, deps{clock: testutil.NewClock(epoch)})
}

// authResult is the outcome of a request served through the
// authentication or rate limit middlewares.
type authResult struct {
	rec       *httptest.ResponseRecorder
	served    bool
	principal string
	hasPrinc  bool
	claims    gojwt.MapClaims
	hasClaims bool
}

// authServe serves the request with a router of the service 3 holding the
// endpoint 7 of the pattern "GET /items/{id}", wrapped in the
// middlewares, recording what reached the handler.
func authServe(t *testing.T, req *http.Request, mws ...flam.IRestMiddleware) authResult {
	t.Helper()
	var result authResult
	router, err := rest.NewRouter(3, mws...)
	testutil.NoError(t, err)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: "GET /items/{id}"},
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			result.served = true
			result.principal, result.hasPrinc = rest.PrincipalOf(req.Context())
			result.claims, result.hasClaims = jwt.ClaimsOf(req.Context())
			w.WriteHeader(http.StatusNoContent)
		})))
	result.rec = httptest.NewRecorder()
	router.ServeHTTP(result.rec, req)
	return result
}

// authRequest returns a GET request of the target from the remote address
// "10.0.0.1:4321", with the headers, given as name and value pairs.
func authRequest(target string, headers ...string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = "10.0.0.1:4321"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	return req
}

// assertProblem asserts that the response is the problem of the endpoint
// 7 of the service 3 of the status and the detail, along with the header
// of the name and the value, the handler not being served.
func assertProblem(t *testing.T, result authResult, status int, detail, header, value string) {
	t.Helper()
	testutil.Equal(t, "served", result.served, false)
	testutil.Equal(t, "status", result.rec.Code, status)
	testutil.Equal(t, "content type", result.rec.Header().Get("Content-Type"), "application/problem+json")
	testutil.Equal(t, header, result.rec.Header().Get(header), value)
	var problem map[string]any
	testutil.NoError(t, json.Unmarshal(result.rec.Body.Bytes(), &problem))
	testutil.Equal(t, "detail", problem["detail"], any(detail))
	testutil.Equal(t, "status", problem["status"], any(float64(status)))
	testutil.Equal(t, "id", problem["id"], any("s:3.e:7"))
}

func Test_Jwt_Bundle(t *testing.T) {
	t.Run("should configure the jwt middleware", func(t *testing.T) {
		b := jwt.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.middlewares.jwt")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + jwt.Id: container.Bag{factory.ConfigFieldDriver: jwt.Driver},
			jwt.DefaultAlgorithmsConfigPath:          []string{"HS256"},
			jwt.DefaultIssuerConfigPath:              "",
			jwt.DefaultAudienceConfigPath:            "",
			jwt.DefaultLeewayConfigPath:              "0s",
			jwt.DefaultOptionalConfigPath:            false,
			jwt.DefaultRequireExpirationConfigPath:   true,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		testutil.Equal(t, "Id", jwt.Id, "jwt")
		testutil.Equal(t, "Driver", jwt.Driver, "flam.rest.middlewares.driver.jwt")
		for got, want := range map[string]string{
			jwt.DefaultAlgorithmsConfigPath:        "flam.rest.defaults.middlewares.jwt.algorithms",
			jwt.DefaultIssuerConfigPath:            "flam.rest.defaults.middlewares.jwt.issuer",
			jwt.DefaultAudienceConfigPath:          "flam.rest.defaults.middlewares.jwt.audience",
			jwt.DefaultLeewayConfigPath:            "flam.rest.defaults.middlewares.jwt.leeway",
			jwt.DefaultOptionalConfigPath:          "flam.rest.defaults.middlewares.jwt.optional",
			jwt.DefaultRequireExpirationConfigPath: "flam.rest.defaults.middlewares.jwt.require_expiration",
		} {
			testutil.Equal(t, "path", got, want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})

	t.Run("should give a new default algorithm list each time", func(t *testing.T) {
		a, b := container.Bag{}, container.Bag{}
		testutil.NoError(t, jwt.NewBundle().(flam.IConfigurableBundle).Config(a))
		testutil.NoError(t, jwt.NewBundle().(flam.IConfigurableBundle).Config(b))
		testutil.Equal(t, "shared", testutil.SamePointer(a.Get(jwt.DefaultAlgorithmsConfigPath), b.Get(jwt.DefaultAlgorithmsConfigPath)), false)
	})

	t.Run("should create the authentication and rate limit middlewares through the aggregate bundle", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(epoch) }))
		bag := container.Bag{}
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), jwt.NewBundle(), apikey.NewBundle(), ratelimit.NewBundle()} {
			if configurable, ok := b.(flam.IConfigurableBundle); ok {
				testutil.NoError(t, configurable.Config(bag))
			}
		}
		testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+"."+jwt.Id+"."+jwt.ConfigFieldSecret, jwtSecret))
		testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+"."+apikey.Id+"."+apikey.ConfigFieldKeys, []any{"key"}))
		for _, b := range []flam.IBundle{config.NewBundle(), rest.NewBundle(), middlewares.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(b))
		}
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

		testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) {
			for _, id := range []string{jwt.Id, apikey.Id, ratelimit.Id} {
				m, err := f.Get(id)
				testutil.NoError(t, err)
				testutil.Equal(t, id+" Id()", m.Id(), id)
			}
		}))
	})
}

func Test_Jwt_Create(t *testing.T) {
	keys := jwtKeys()
	rsaPKIX := jwtPKIX(t, &keys.rsa.PublicKey)

	t.Run("should create the middleware of each key form", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "key.pem")
		testutil.NoError(t, os.WriteFile(file, []byte(rsaPKIX), 0o600))
		for name, fields := range map[string]container.Bag{
			"secret":          {jwt.ConfigFieldSecret: jwtSecret},
			"hs512":           {jwt.ConfigFieldAlgorithms: []any{"HS512"}, jwt.ConfigFieldSecret: strings.Repeat("s", 64)},
			"rsa pkix":        {jwt.ConfigFieldAlgorithms: []any{"RS256", "PS512"}, jwt.ConfigFieldPublicKey: rsaPKIX},
			"rsa pkcs1":       {jwt.ConfigFieldAlgorithms: []string{"RS256"}, jwt.ConfigFieldPublicKey: jwtPEM("RSA PUBLIC KEY", x509.MarshalPKCS1PublicKey(&keys.rsa.PublicKey))},
			"rsa certificate": {jwt.ConfigFieldAlgorithms: []string{"RS256"}, jwt.ConfigFieldPublicKey: jwtCertificate(t, &keys.rsa.PublicKey, keys.rsa)},
			"rsa file":        {jwt.ConfigFieldAlgorithms: []string{"RS256"}, jwt.ConfigFieldPublicKeyFile: file},
			"ecdsa":           {jwt.ConfigFieldAlgorithms: []string{"ES384"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, &keys.p384.PublicKey)},
			"ed25519":         {jwt.ConfigFieldAlgorithms: []string{"EdDSA"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, keys.ed.Public())},
			"empty others":    {jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldPublicKey: "", jwt.ConfigFieldPublicKeyFile: ""},
		} {
			m, err := createJwt(t, fields, nil)
			if err != nil {
				t.Fatalf("%s: error = %v", name, err)
			}
			testutil.Equal(t, name+" Id()", m.Id(), "main")
		}
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		missingFile := filepath.Join(t.TempDir(), "missing.pem")
		for name, tc := range map[string]struct {
			fields container.Bag
			values map[string]any
			field  string
			cause  string
		}{
			"algorithms type":    {container.Bag{jwt.ConfigFieldAlgorithms: "HS256", jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, ""},
			"algorithms empty":   {container.Bag{jwt.ConfigFieldAlgorithms: []any{}, jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, ""},
			"algorithms default": {container.Bag{jwt.ConfigFieldSecret: jwtSecret}, map[string]any{jwt.DefaultAlgorithmsConfigPath: 1}, jwt.ConfigFieldAlgorithms, ""},
			"algorithm none":     {container.Bag{jwt.ConfigFieldAlgorithms: []any{"HS256", "none"}, jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, "unknown or unsigned algorithm"},
			"algorithm unknown":  {container.Bag{jwt.ConfigFieldAlgorithms: []any{"XX256"}, jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, "unknown or unsigned algorithm"},
			"no key":             {container.Bag{}, nil, jwt.ConfigFieldSecret, "one of secret, public_key or public_key_file must be set"},
			"secret type":        {container.Bag{jwt.ConfigFieldSecret: 1}, nil, jwt.ConfigFieldSecret, "key field not a string"},
			"public key type":    {container.Bag{jwt.ConfigFieldPublicKey: true}, nil, jwt.ConfigFieldPublicKey, "key field not a string"},
			"two keys":           {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldPublicKey: rsaPKIX}, nil, jwt.ConfigFieldPublicKey, "only one of secret, public_key or public_key_file may be set"},
			"key and file":       {container.Bag{jwt.ConfigFieldPublicKey: rsaPKIX, jwt.ConfigFieldPublicKeyFile: "key.pem"}, nil, jwt.ConfigFieldPublicKeyFile, "only one of secret, public_key or public_key_file may be set"},
			"no pem":             {container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldPublicKey: "not a pem"}, nil, jwt.ConfigFieldPublicKey, "no PEM block found"},
			"x25519 key":         {container.Bag{jwt.ConfigFieldAlgorithms: []any{"EdDSA"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, keys.x25519.PublicKey())}, nil, jwt.ConfigFieldPublicKey, "public key not of RSA, ECDSA or Ed25519 type"},
			"short secret":       {container.Bag{jwt.ConfigFieldSecret: jwtSecret[:31]}, nil, jwt.ConfigFieldAlgorithms, "secret shorter than the hash of the algorithm"},
			"short hs512 secret": {container.Bag{jwt.ConfigFieldAlgorithms: []any{"HS256", "HS512"}, jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, "secret shorter than the hash of the algorithm"},
			"hmac with rsa":      {container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256", "HS256"}, jwt.ConfigFieldPublicKey: rsaPKIX}, nil, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"rsa with secret":    {container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldSecret: jwtSecret}, nil, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"rsa with ecdsa":     {container.Bag{jwt.ConfigFieldAlgorithms: []any{"PS256"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, &keys.p256.PublicKey)}, nil, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"weak rsa":           {container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, &keys.rsaWeak.PublicKey)}, nil, jwt.ConfigFieldAlgorithms, "rsa key shorter than 2048 bits"},
			"ecdsa with rsa":     {container.Bag{jwt.ConfigFieldAlgorithms: []any{"ES256"}, jwt.ConfigFieldPublicKey: rsaPKIX}, nil, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"ecdsa curve":        {container.Bag{jwt.ConfigFieldAlgorithms: []any{"ES256"}, jwt.ConfigFieldPublicKey: jwtPKIX(t, &keys.p384.PublicKey)}, nil, jwt.ConfigFieldAlgorithms, "ecdsa key not of the curve of the algorithm"},
			"eddsa with rsa":     {container.Bag{jwt.ConfigFieldAlgorithms: []any{"EdDSA"}, jwt.ConfigFieldPublicKey: rsaPKIX}, nil, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"issuer":             {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldIssuer: 1}, nil, jwt.ConfigFieldIssuer, ""},
			"audience":           {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldAudience: []any{"a"}}, nil, jwt.ConfigFieldAudience, ""},
			"leeway":             {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldLeeway: "-1s"}, nil, jwt.ConfigFieldLeeway, ""},
			"optional":           {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldOptional: "yes"}, nil, jwt.ConfigFieldOptional, ""},
			"require expiration": {container.Bag{jwt.ConfigFieldSecret: jwtSecret}, map[string]any{jwt.DefaultRequireExpirationConfigPath: 1}, jwt.ConfigFieldRequireExpiration, ""},
			"missing file":       {container.Bag{jwt.ConfigFieldPublicKeyFile: missingFile}, nil, jwt.ConfigFieldPublicKeyFile, "open " + missingFile + ": no such file or directory"},
		} {
			_, err := createJwt(t, tc.fields, tc.values)
			text := "unable to create jwt rest middleware: invalid jwt rest middleware field"
			if tc.cause != "" {
				text += ": " + tc.cause
			}
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			testutil.Error(t, errors.Unwrap(err), jwt.ErrInvalidField, text,
				map[string]any{jwt.ErrorFieldID: "main", jwt.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail on an undecodable key", func(t *testing.T) {
		badFile := filepath.Join(t.TempDir(), "bad.pem")
		testutil.NoError(t, os.WriteFile(badFile, []byte("garbage"), 0o600))
		for name, tc := range map[string]struct {
			fields container.Bag
			field  string
		}{
			"pkix":        {container.Bag{jwt.ConfigFieldPublicKey: jwtPEM("PUBLIC KEY", []byte("x"))}, jwt.ConfigFieldPublicKey},
			"pkcs1":       {container.Bag{jwt.ConfigFieldPublicKey: jwtPEM("RSA PUBLIC KEY", []byte("x"))}, jwt.ConfigFieldPublicKey},
			"certificate": {container.Bag{jwt.ConfigFieldPublicKey: jwtPEM("CERTIFICATE", []byte("x"))}, jwt.ConfigFieldPublicKey},
			"file":        {container.Bag{jwt.ConfigFieldPublicKeyFile: badFile}, jwt.ConfigFieldPublicKeyFile},
		} {
			_, err := createJwt(t, tc.fields, nil)
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			if !errors.Is(err, jwt.ErrInvalidField) {
				t.Fatalf("%s: error = %v, want %v", name, err, jwt.ErrInvalidField)
			}
			var e *flam.Error
			if !errors.As(errors.Unwrap(err), &e) {
				t.Fatalf("%s: error = %T", name, err)
			}
			testutil.Context(t, e, map[string]any{jwt.ErrorFieldID: "main", jwt.ErrorFieldField: tc.field})
		}
	})
}

func Test_Jwt(t *testing.T) {
	keys := jwtKeys()
	hs256 := func(claims gojwt.MapClaims) string {
		return jwtSign(t, gojwt.SigningMethodHS256, []byte(jwtSecret), claims)
	}
	bearer := func(token string) *http.Request {
		return authRequest("/items/1", "Authorization", "Bearer "+token)
	}
	hsMiddleware := func(t *testing.T, fields container.Bag) flam.IRestMiddleware {
		t.Helper()
		fields[jwt.ConfigFieldSecret] = jwtSecret
		fields[jwt.ConfigFieldIssuer] = "issuer"
		fields[jwt.ConfigFieldAudience] = "audience"
		m, err := createJwt(t, fields, nil)
		testutil.NoError(t, err)
		return m
	}

	t.Run("should serve a valid token with its claims and subject", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{})
		for name, req := range map[string]*http.Request{
			"bearer":       bearer(hs256(jwtClaims("alice", gojwt.MapClaims{"role": "admin"}))),
			"lower scheme": authRequest("/items/1", "Authorization", "bearer "+hs256(jwtClaims("alice", gojwt.MapClaims{"role": "admin"}))),
			"extra spaces": authRequest("/items/1", "Authorization", "Bearer  "+hs256(jwtClaims("alice", gojwt.MapClaims{"role": "admin"}))+" "),
		} {
			result := authServe(t, req, m)
			testutil.Equal(t, name+" status", result.rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" served", result.served, true)
			testutil.Equal(t, name+" principal", result.principal, "alice")
			testutil.Equal(t, name+" has principal", result.hasPrinc, true)
			testutil.Equal(t, name+" has claims", result.hasClaims, true)
			testutil.Equal(t, name+" role", result.claims["role"], any("admin"))
			testutil.Equal(t, name+" sub", result.claims["sub"], any("alice"))
		}
	})

	t.Run("should serve a valid token without subject without principal", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{})
		for name, claims := range map[string]gojwt.MapClaims{
			"no subject":    jwtClaims(nil, nil),
			"empty subject": jwtClaims("", nil),
		} {
			result := authServe(t, bearer(hs256(claims)), m)
			testutil.Equal(t, name+" served", result.served, true)
			testutil.Equal(t, name+" has principal", result.hasPrinc, false)
			testutil.Equal(t, name+" has claims", result.hasClaims, true)
		}
	})

	t.Run("should answer a request without bearer token with a 401", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{})
		for name, req := range map[string]*http.Request{
			"no header":   authRequest("/items/1"),
			"empty":       authRequest("/items/1", "Authorization", ""),
			"basic":       authRequest("/items/1", "Authorization", "Basic YTpi"),
			"bearer-like": authRequest("/items/1", "Authorization", "BearerX abc"),
		} {
			t.Run(name, func(t *testing.T) {
				assertProblem(t, authServe(t, req, m), http.StatusUnauthorized, "missing bearer token", "WWW-Authenticate", jwt.ChallengeMissing)
			})
		}
	})

	t.Run("should serve a request without bearer token when optional", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{jwt.ConfigFieldOptional: true})
		result := authServe(t, authRequest("/items/1", "Authorization", "Basic YTpi"), m)
		testutil.Equal(t, "served", result.served, true)
		testutil.Equal(t, "has principal", result.hasPrinc, false)
		testutil.Equal(t, "has claims", result.hasClaims, false)

		assertProblem(t, authServe(t, bearer("garbage"), m), http.StatusUnauthorized, "invalid bearer token", "WWW-Authenticate", jwt.ChallengeInvalid)
	})

	t.Run("should answer an invalid token with a 401 without detail", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{jwt.ConfigFieldLeeway: "30s"})
		unsigned, err := gojwt.NewWithClaims(gojwt.SigningMethodNone, jwtClaims("alice", nil)).SignedString(gojwt.UnsafeAllowNoneSignatureType)
		testutil.NoError(t, err)
		valid := hs256(jwtClaims("alice", nil))
		for name, token := range map[string]string{
			"empty":            "",
			"garbage":          "garbage",
			"unsigned":         unsigned,
			"tampered":         valid[:len(valid)-2] + "AA",
			"other secret":     jwtSign(t, gojwt.SigningMethodHS256, []byte(strings.Repeat("x", 32)), jwtClaims("alice", nil)),
			"other algorithm":  jwtSign(t, gojwt.SigningMethodHS384, []byte(jwtSecret+jwtSecret), jwtClaims("alice", nil)),
			"expired":          hs256(jwtClaims("alice", gojwt.MapClaims{"exp": epoch.Add(-31 * time.Second).Unix()})),
			"not yet valid":    hs256(jwtClaims("alice", gojwt.MapClaims{"nbf": epoch.Add(31 * time.Second).Unix()})),
			"no expiration":    hs256(jwtClaims("alice", gojwt.MapClaims{"exp": nil})),
			"other issuer":     hs256(jwtClaims("alice", gojwt.MapClaims{"iss": "other"})),
			"no issuer":        hs256(jwtClaims("alice", gojwt.MapClaims{"iss": nil})),
			"other audience":   hs256(jwtClaims("alice", gojwt.MapClaims{"aud": "other"})),
			"no audience":      hs256(jwtClaims("alice", gojwt.MapClaims{"aud": nil})),
			"subject number":   hs256(jwtClaims(7, nil)),
			"subject null":     hs256(gojwt.MapClaims{"sub": nil, "iss": "issuer", "aud": "audience", "exp": epoch.Add(time.Hour).Unix()}),
			"rsa key as hmac":  jwtSign(t, gojwt.SigningMethodHS256, []byte(jwtPKIX(t, &keys.rsa.PublicKey)), jwtClaims("alice", nil)),
			"rsa signed token": jwtSign(t, gojwt.SigningMethodRS256, keys.rsa, jwtClaims("alice", nil)),
		} {
			t.Run(name, func(t *testing.T) {
				result := authServe(t, bearer(token), m)
				assertProblem(t, result, http.StatusUnauthorized, "invalid bearer token", "WWW-Authenticate", jwt.ChallengeInvalid)
				testutil.Equal(t, "body", strings.Contains(result.rec.Body.String(), "expired"), false)
			})
		}
	})

	t.Run("should accept the time claims within the leeway", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{jwt.ConfigFieldLeeway: 30 * time.Second})
		for name, claims := range map[string]gojwt.MapClaims{
			"expired":       jwtClaims("alice", gojwt.MapClaims{"exp": epoch.Add(-29 * time.Second).Unix()}),
			"not yet valid": jwtClaims("alice", gojwt.MapClaims{"nbf": epoch.Add(29 * time.Second).Unix()}),
		} {
			testutil.Equal(t, name+" served", authServe(t, bearer(hs256(claims)), m).served, true)
		}
	})

	t.Run("should not check the unset issuer, audience and expiration", func(t *testing.T) {
		m, err := createJwt(t, container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldRequireExpiration: false}, nil)
		testutil.NoError(t, err)
		token := hs256(gojwt.MapClaims{"sub": "alice", "iss": "any", "aud": "any"})
		result := authServe(t, bearer(token), m)
		testutil.Equal(t, "served", result.served, true)
		testutil.Equal(t, "principal", result.principal, "alice")

		expired := hs256(gojwt.MapClaims{"sub": "alice", "exp": epoch.Add(-time.Second).Unix()})
		assertProblem(t, authServe(t, bearer(expired), m), http.StatusUnauthorized, "invalid bearer token", "WWW-Authenticate", jwt.ChallengeInvalid)
	})

	t.Run("should verify the tokens of the public key algorithms", func(t *testing.T) {
		for name, tc := range map[string]struct {
			algorithms []any
			public     any
			method     gojwt.SigningMethod
			private    any
		}{
			"rs256": {[]any{"RS256"}, &keys.rsa.PublicKey, gojwt.SigningMethodRS256, keys.rsa},
			"ps384": {[]any{"RS256", "PS384"}, &keys.rsa.PublicKey, gojwt.SigningMethodPS384, keys.rsa},
			"es256": {[]any{"ES256"}, &keys.p256.PublicKey, gojwt.SigningMethodES256, keys.p256},
			"eddsa": {[]any{"EdDSA"}, keys.ed.Public(), gojwt.SigningMethodEdDSA, keys.ed},
		} {
			m, err := createJwt(t, container.Bag{jwt.ConfigFieldAlgorithms: tc.algorithms, jwt.ConfigFieldPublicKey: jwtPKIX(t, tc.public)}, nil)
			testutil.NoError(t, err)
			result := authServe(t, bearer(jwtSign(t, tc.method, tc.private, jwtClaims("alice", nil))), m)
			testutil.Equal(t, name+" served", result.served, true)
			testutil.Equal(t, name+" principal", result.principal, "alice")
		}
	})

	t.Run("should reject the algorithm confusion of a public key used as an hmac secret", func(t *testing.T) {
		public := jwtPKIX(t, &keys.rsa.PublicKey)
		m, err := createJwt(t, container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldPublicKey: public}, nil)
		testutil.NoError(t, err)
		forged := jwtSign(t, gojwt.SigningMethodHS256, []byte(public), jwtClaims("alice", nil))
		assertProblem(t, authServe(t, bearer(forged), m), http.StatusUnauthorized, "invalid bearer token", "WWW-Authenticate", jwt.ChallengeInvalid)
	})

	t.Run("should be read concurrently", func(t *testing.T) {
		m := hsMiddleware(t, container.Bag{})
		token := hs256(jwtClaims("alice", nil))
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				result := authServe(t, bearer(token), m)
				testutil.Equal(t, "served", result.served, true)
			})
		}
		wg.Wait()
	})
}

func Test_Jwt_ClaimsOf(t *testing.T) {
	var nilCtx context.Context
	for name, ctx := range map[string]context.Context{
		"nil":      nilCtx,
		"no value": context.Background(),
	} {
		claims, ok := jwt.ClaimsOf(ctx)
		testutil.Equal(t, name+" claims", claims, gojwt.MapClaims(nil))
		testutil.Equal(t, name+" ok", ok, false)
	}
}
