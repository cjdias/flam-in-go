package middlewares

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
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
	"github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/jwt"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// jwksExtraKeys are the keys of the key set tests, beyond the jwt ones,
// generated once.
var jwksExtraKeys = sync.OnceValue(func() (keys struct {
	rsa  *rsa.PrivateKey
	p521 *ecdsa.PrivateKey
},
) {
	var err error
	if keys.rsa, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
		panic(err)
	}
	if keys.p521, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader); err != nil {
		panic(err)
	}
	return keys
})

// jwksAlgorithms are the algorithms of the key set middlewares of the
// tests.
var jwksAlgorithms = []any{"RS256", "PS256", "ES256", "ES384", "ES512", "EdDSA"}

// jwksB64 returns the unpadded base64url encoding of the bytes.
func jwksB64(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

// jwksJWK returns the JSON Web Key of the id and the RSA, ECDSA or Ed25519
// public key, extended by the extra members, a nil one being removed.
func jwksJWK(t *testing.T, kid string, public any, extra map[string]any) map[string]any {
	t.Helper()
	var key map[string]any
	switch k := public.(type) {
	case *rsa.PublicKey:
		key = map[string]any{"kty": "RSA", "n": jwksB64(k.N.Bytes()), "e": jwksB64(big.NewInt(int64(k.E)).Bytes())}
	case *ecdsa.PublicKey:
		point, err := k.Bytes()
		testutil.NoError(t, err)
		size := (len(point) - 1) / 2
		key = map[string]any{"kty": "EC", "crv": k.Curve.Params().Name, "x": jwksB64(point[1 : 1+size]), "y": jwksB64(point[1+size:])}
	case ed25519.PublicKey:
		key = map[string]any{"kty": "OKP", "crv": "Ed25519", "x": jwksB64(k)}
	}
	key["kid"] = kid
	for name, value := range extra {
		if value == nil {
			delete(key, name)
			continue
		}
		key[name] = value
	}
	return key
}

// jwksSign returns the token of the claims signed with the method and the
// key, its "kid" header holding the kid, unless nil.
func jwksSign(t *testing.T, method gojwt.SigningMethod, key any, kid any, claims gojwt.MapClaims) string {
	t.Helper()
	token := gojwt.NewWithClaims(method, claims)
	if kid != nil {
		token.Header["kid"] = kid
	}
	signed, err := token.SignedString(key)
	testutil.NoError(t, err)
	return signed
}

// jwksIdP is a TLS identity provider serving its discovery document under
// "/realm" and its key set at "/jwks", counting the requests of each path.
type jwksIdP struct {
	server *httptest.Server

	mu        sync.Mutex
	keys      []map[string]any
	status    int
	discovery map[string]any
	hits      map[string]int
	gate      chan struct{} // the key set answers wait for, unless nil
}

// newJwksIdP returns a started identity provider of the keys, closed with
// the test.
func newJwksIdP(t *testing.T, keys ...map[string]any) *jwksIdP {
	t.Helper()
	idp := &jwksIdP{keys: keys, status: http.StatusOK, hits: map[string]int{}}
	idp.server = httptest.NewTLSServer(http.HandlerFunc(idp.serve))
	t.Cleanup(idp.server.Close)
	return idp
}

// serve answers the discovery document and key set requests.
func (p *jwksIdP) serve(w http.ResponseWriter, req *http.Request) {
	p.mu.Lock()
	p.hits[req.URL.Path]++
	if gate := p.gate; gate != nil && req.URL.Path == "/jwks" {
		p.mu.Unlock()
		<-gate
		p.mu.Lock()
	}
	defer p.mu.Unlock()
	var body any
	switch req.URL.Path {
	case "/realm" + "/.well-known/openid-configuration":
		body = map[string]any{"issuer": p.server.URL + "/realm", "jwks_uri": p.server.URL + "/jwks"}
		if p.discovery != nil {
			body = p.discovery
		}
	case "/jwks":
		if p.status != http.StatusOK {
			w.WriteHeader(p.status)
			return
		}
		body = map[string]any{"keys": p.keys}
	default:
		http.NotFound(w, req)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// hang makes the key set answers wait, once their request counted,
// until the returned function, which may be called more than once, is
// called.
func (p *jwksIdP) hang() func() {
	p.mu.Lock()
	defer p.mu.Unlock()
	gate := make(chan struct{})
	p.gate = gate
	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			p.gate = nil
			p.mu.Unlock()
			close(gate)
		})
	}
}

// set replaces the keys and the status of the key set answers.
func (p *jwksIdP) set(status int, keys ...map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status, p.keys = status, keys
}

// count returns the number of requests of the path.
func (p *jwksIdP) count(path string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.hits[path]
}

// jwksClients is a flam.IClientFactory of the http clients of their ids,
// failing with errBoom for the other ids.
type jwksClients struct {
	flam.IClientFactory

	clients map[string]*http.Client
}

func (f *jwksClients) Get(id string) (flam.IClient, error) {
	client, ok := f.clients[id]
	if !ok {
		return nil, errBoom
	}
	return &jwksClient{client: client}, nil
}

// jwksClient is a flam.IClient sending its requests with its http client.
type jwksClient struct {
	flam.IClient

	client *http.Client
}

func (c *jwksClient) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

// createJwks returns the jwt middleware "main" of the fields, created
// with the jwt bundle, the clock and the client factory, unless nil.
func createJwks(t *testing.T, fields container.Bag, clock flam.IClock, clients flam.IClientFactory) (flam.IRestMiddleware, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), jwt.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	fields[factory.ConfigFieldDriver] = jwt.Driver
	testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+".main", fields))

	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	if clients != nil {
		testutil.NoError(t, app.Provide(func() flam.IClientFactory { return clients }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var m flam.IRestMiddleware
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IRestMiddlewareFactory) { m, err = f.Get("main") }))
	return m, err
}

// jwksMiddleware returns the key set middleware of the identity provider,
// fetching its "/jwks" with its "idp" client, the fields overriding the
// defaults, a nil one being removed.
func jwksMiddleware(t *testing.T, idp *jwksIdP, clock flam.IClock, fields container.Bag) flam.IRestMiddleware {
	t.Helper()
	bag := container.Bag{
		jwt.ConfigFieldAlgorithms: jwksAlgorithms,
		jwt.ConfigFieldJWKSURL:    idp.server.URL + "/jwks",
		jwt.ConfigFieldJWKSClient: "idp",
	}
	for name, value := range fields {
		if value == nil {
			delete(bag, name)
			continue
		}
		bag[name] = value
	}
	m, err := createJwks(t, bag, clock, &jwksClients{clients: map[string]*http.Client{"idp": idp.server.Client()}})
	testutil.NoError(t, err)
	return m
}

// jwksBearer returns a request of the endpoint 7 with the bearer token.
func jwksBearer(token string) *http.Request {
	return authRequest("/items/1", "Authorization", "Bearer "+token)
}

// jwksUntil serves the request of the token through the middleware until
// it is served, or rejected when not served, returning the last result,
// failing the test after 5s, as a stale key set is fetched again in the
// background.
func jwksUntil(t *testing.T, m flam.IRestMiddleware, token string, served bool) authResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		result := authServe(t, jwksBearer(token), m)
		if result.served == served {
			return result
		}
		if time.Now().After(deadline) {
			t.Fatalf("served = %v after 5s, want %v", result.served, served)
		}
		time.Sleep(time.Millisecond)
	}
}

// jwksWaitCount waits until the identity provider was requested n times
// for the path, failing the test after 5s.
func jwksWaitCount(t *testing.T, idp *jwksIdP, path string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for idp.count(path) != n {
		if time.Now().After(deadline) {
			t.Fatalf("%s requested %d times, want %d", path, idp.count(path), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// jwksRejected asserts that the result is the 401 of an invalid token.
func jwksRejected(t *testing.T, result authResult) {
	t.Helper()
	assertProblem(t, result, http.StatusUnauthorized, "invalid bearer token", "WWW-Authenticate", jwt.ChallengeInvalid)
}

func Test_Jwks_Create(t *testing.T) {
	clients := &jwksClients{clients: map[string]*http.Client{"idp": http.DefaultClient}}

	t.Run("should create the middleware of a key set or an issuer", func(t *testing.T) {
		for name, fields := range map[string]container.Bag{
			"jwks url":         {jwt.ConfigFieldAlgorithms: jwksAlgorithms, jwt.ConfigFieldJWKSURL: "https://idp.example.com/jwks"},
			"issuer url":       {jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldIssuerURL: "https://idp.example.com/realm"},
			"client":           {jwt.ConfigFieldAlgorithms: []any{"EdDSA"}, jwt.ConfigFieldJWKSURL: "https://idp.example.com:8443/jwks", jwt.ConfigFieldJWKSClient: "idp"},
			"durations":        {jwt.ConfigFieldAlgorithms: []any{"ES256"}, jwt.ConfigFieldJWKSURL: "https://idp.example.com/jwks", jwt.ConfigFieldJWKSRefresh: 5 * time.Minute, jwt.ConfigFieldJWKSMinRefresh: "1s", jwt.ConfigFieldJWKSTimeout: "1s"},
			"empty other keys": {jwt.ConfigFieldAlgorithms: []any{"RS256"}, jwt.ConfigFieldJWKSURL: "https://idp.example.com/jwks", jwt.ConfigFieldSecret: "", jwt.ConfigFieldIssuerURL: ""},
		} {
			m, err := createJwks(t, fields, testutil.NewClock(epoch), clients)
			if err != nil {
				t.Fatalf("%s: error = %v", name, err)
			}
			testutil.Equal(t, name+" Id()", m.Id(), "main")
		}
	})

	t.Run("should not fetch the key set when created", func(t *testing.T) {
		idp := newJwksIdP(t)
		jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{})
		testutil.Equal(t, "fetches", idp.count("/jwks"), 0)
	})

	t.Run("should fail on an invalid field", func(t *testing.T) {
		url := "https://idp.example.com/jwks"
		rs256 := []any{"RS256"}
		for name, tc := range map[string]struct {
			fields  container.Bag
			clients flam.IClientFactory
			field   string
			cause   string
		}{
			"jwks url type":       {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: 1}, clients, jwt.ConfigFieldJWKSURL, "key field not a string"},
			"issuer url type":     {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldIssuerURL: []any{}}, clients, jwt.ConfigFieldIssuerURL, "key field not a string"},
			"secret and jwks":     {container.Bag{jwt.ConfigFieldSecret: jwtSecret, jwt.ConfigFieldJWKSURL: url}, clients, jwt.ConfigFieldJWKSURL, "only one of secret, public_key, public_key_file, jwks_url or issuer_url may be set"},
			"jwks and issuer":     {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldIssuerURL: url}, clients, jwt.ConfigFieldIssuerURL, "only one of secret, public_key, public_key_file, jwks_url or issuer_url may be set"},
			"http url":            {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: "http://idp.example.com/jwks"}, clients, jwt.ConfigFieldJWKSURL, "url not an absolute https url"},
			"relative url":        {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: "/jwks"}, clients, jwt.ConfigFieldJWKSURL, "url not an absolute https url"},
			"no host":             {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldIssuerURL: "https:///realm"}, clients, jwt.ConfigFieldIssuerURL, "url not an absolute https url"},
			"user info":           {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: "https://user:pass@idp.example.com/jwks"}, clients, jwt.ConfigFieldJWKSURL, "url not an absolute https url"},
			"query":               {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldIssuerURL: "https://idp.example.com/realm?x=1"}, clients, jwt.ConfigFieldIssuerURL, "url not an absolute https url"},
			"empty query":         {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldIssuerURL: "https://idp.example.com/realm?"}, clients, jwt.ConfigFieldIssuerURL, "url not an absolute https url"},
			"fragment":            {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url + "#x"}, clients, jwt.ConfigFieldJWKSURL, "url not an absolute https url"},
			"unparsable url":      {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: "https://idp.example.com/%zz"}, clients, jwt.ConfigFieldJWKSURL, "url not an absolute https url"},
			"hmac algorithm":      {container.Bag{jwt.ConfigFieldJWKSURL: url}, clients, jwt.ConfigFieldAlgorithms, "algorithm not fitting the key"},
			"none algorithm":      {container.Bag{jwt.ConfigFieldAlgorithms: []any{"RS256", "none"}, jwt.ConfigFieldJWKSURL: url}, clients, jwt.ConfigFieldAlgorithms, "unknown or unsigned algorithm"},
			"refresh zero":        {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSRefresh: "0s"}, clients, jwt.ConfigFieldJWKSRefresh, ""},
			"refresh type":        {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSRefresh: 3600}, clients, jwt.ConfigFieldJWKSRefresh, ""},
			"min refresh zero":    {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSMinRefresh: time.Duration(0)}, clients, jwt.ConfigFieldJWKSMinRefresh, ""},
			"min refresh invalid": {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSMinRefresh: "soon"}, clients, jwt.ConfigFieldJWKSMinRefresh, ""},
			"timeout negative":    {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSTimeout: "-1s"}, clients, jwt.ConfigFieldJWKSTimeout, ""},
			"client type":         {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSClient: 1}, clients, jwt.ConfigFieldJWKSClient, ""},
			"no client factory":   {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldJWKSURL: url, jwt.ConfigFieldJWKSClient: "idp"}, nil, jwt.ConfigFieldJWKSClient, "no client factory, register the client bundle"},
			"unknown client":      {container.Bag{jwt.ConfigFieldAlgorithms: rs256, jwt.ConfigFieldIssuerURL: url, jwt.ConfigFieldJWKSClient: "other"}, clients, jwt.ConfigFieldJWKSClient, "boom"},
		} {
			_, err := createJwks(t, tc.fields, testutil.NewClock(epoch), tc.clients)
			if err == nil {
				t.Fatalf("%s: error = nil", name)
			}
			text := "unable to create jwt rest middleware: invalid jwt rest middleware field"
			if tc.cause != "" {
				text += ": " + tc.cause
			}
			testutil.Error(t, errors.Unwrap(err), jwt.ErrInvalidField, text,
				map[string]any{jwt.ErrorFieldID: "main", jwt.ErrorFieldField: tc.field})
		}
	})
}

func Test_Jwks(t *testing.T) {
	keys, extra := jwtKeys(), jwksExtraKeys()

	t.Run("should verify the tokens of each key type by their key id", func(t *testing.T) {
		idp := newJwksIdP(t,
			jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil),
			jwksJWK(t, "p256", &keys.p256.PublicKey, map[string]any{"use": "sig", "alg": "ES256"}),
			jwksJWK(t, "p384", &keys.p384.PublicKey, nil),
			jwksJWK(t, "p521", &extra.p521.PublicKey, nil),
			jwksJWK(t, "ed", keys.ed.Public(), map[string]any{"key_ops": []any{"verify"}, "x5c": []any{"ignored"}}),
		)
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{})
		for name, tc := range map[string]struct {
			method  gojwt.SigningMethod
			private any
			kid     string
		}{
			"rs256": {gojwt.SigningMethodRS256, keys.rsa, "rsa"},
			"ps256": {gojwt.SigningMethodPS256, keys.rsa, "rsa"},
			"es256": {gojwt.SigningMethodES256, keys.p256, "p256"},
			"es384": {gojwt.SigningMethodES384, keys.p384, "p384"},
			"es512": {gojwt.SigningMethodES512, extra.p521, "p521"},
			"eddsa": {gojwt.SigningMethodEdDSA, keys.ed, "ed"},
		} {
			result := authServe(t, jwksBearer(jwksSign(t, tc.method, tc.private, tc.kid, jwtClaims("alice", nil))), m)
			testutil.Equal(t, name+" status", result.rec.Code, http.StatusNoContent)
			testutil.Equal(t, name+" principal", result.principal, "alice")
			testutil.Equal(t, name+" has claims", result.hasClaims, true)
		}
		testutil.Equal(t, "fetches", idp.count("/jwks"), 1)
	})

	t.Run("should reject a token not verified by the key it names", func(t *testing.T) {
		idp := newJwksIdP(t,
			jwksJWK(t, "rsa", &keys.rsa.PublicKey, map[string]any{"alg": "RS512"}),
			jwksJWK(t, "p256", &keys.p256.PublicKey, nil),
			jwksJWK(t, "weak", &keys.rsaWeak.PublicKey, nil),
			jwksJWK(t, "ed", keys.ed.Public(), nil),
		)
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{})
		claims := jwtClaims("alice", nil)
		public := jwtPKIX(t, &keys.rsa.PublicKey)
		for name, token := range map[string]string{
			"no kid":               jwksSign(t, gojwt.SigningMethodES256, keys.p256, nil, claims),
			"empty kid":            jwksSign(t, gojwt.SigningMethodES256, keys.p256, "", claims),
			"number kid":           jwksSign(t, gojwt.SigningMethodES256, keys.p256, 7, claims),
			"other algorithm":      jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", claims),
			"other curve":          jwksSign(t, gojwt.SigningMethodES384, keys.p384, "p256", claims),
			"other family":         jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "p256", claims),
			"other key":            jwksSign(t, gojwt.SigningMethodEdDSA, ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)), "ed", claims),
			"weak rsa key":         jwksSign(t, gojwt.SigningMethodRS256, keys.rsaWeak, "weak", claims),
			"hmac with public key": jwksSign(t, gojwt.SigningMethodHS256, []byte(public), "rsa", claims),
			"unsigned":             jwksSign(t, gojwt.SigningMethodNone, gojwt.UnsafeAllowNoneSignatureType, "ed", claims),
			"expired":              jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "ed", jwtClaims("alice", gojwt.MapClaims{"exp": epoch.Add(-time.Second).Unix()})),
		} {
			t.Run(name, func(t *testing.T) {
				jwksRejected(t, authServe(t, jwksBearer(token), m))
			})
		}
	})

	t.Run("should keep only the usable keys of the set", func(t *testing.T) {
		rsaKey := jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil)
		huge := new(big.Int).Lsh(big.NewInt(1), 8200)
		idp := newJwksIdP(t,
			jwksJWK(t, "enc", keys.ed.Public(), map[string]any{"use": "enc"}),
			jwksJWK(t, "", keys.ed.Public(), nil),
			jwksJWK(t, "kty", keys.ed.Public(), map[string]any{"kty": "oct"}),
			jwksJWK(t, "x25519", keys.ed.Public(), map[string]any{"crv": "X25519"}),
			jwksJWK(t, "ed short", keys.ed.Public(), map[string]any{"x": jwksB64([]byte("short"))}),
			jwksJWK(t, "ed padded", keys.ed.Public(), map[string]any{"x": base64.URLEncoding.EncodeToString(keys.ed.Public().(ed25519.PublicKey))}),
			jwksJWK(t, "rsa n", &keys.rsa.PublicKey, map[string]any{"n": "!"}),
			jwksJWK(t, "rsa e", &keys.rsa.PublicKey, map[string]any{"e": "!"}),
			jwksJWK(t, "rsa no n", &keys.rsa.PublicKey, map[string]any{"n": nil}),
			jwksJWK(t, "rsa no e", &keys.rsa.PublicKey, map[string]any{"e": nil}),
			jwksJWK(t, "rsa long e", &keys.rsa.PublicKey, map[string]any{"e": jwksB64([]byte{1, 0, 0, 0, 1})}),
			jwksJWK(t, "rsa huge", &keys.rsa.PublicKey, map[string]any{"n": jwksB64(huge.Bytes())}),
			jwksJWK(t, "ec crv", &keys.p256.PublicKey, map[string]any{"crv": "P-224"}),
			jwksJWK(t, "ec x", &keys.p256.PublicKey, map[string]any{"x": "!"}),
			jwksJWK(t, "ec y", &keys.p256.PublicKey, map[string]any{"y": "!"}),
			jwksJWK(t, "ec short", &keys.p256.PublicKey, map[string]any{"x": jwksB64([]byte{1})}),
			jwksJWK(t, "ec off curve", &keys.p256.PublicKey, map[string]any{"y": jwksB64(make([]byte, 32))}),
			jwksJWK(t, "dup", &keys.p256.PublicKey, nil),
			jwksJWK(t, "dup", &keys.p256.PublicKey, nil),
			jwksJWK(t, "dup", &keys.p256.PublicKey, nil),
			rsaKey,
		)
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{})
		claims := jwtClaims("alice", nil)
		for name, token := range map[string]string{
			"enc":          jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "enc", claims),
			"kty":          jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "kty", claims),
			"x25519":       jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "x25519", claims),
			"ed short":     jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "ed short", claims),
			"ed padded":    jwksSign(t, gojwt.SigningMethodEdDSA, keys.ed, "ed padded", claims),
			"rsa n":        jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa n", claims),
			"rsa e":        jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa e", claims),
			"rsa no n":     jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa no n", claims),
			"rsa no e":     jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa no e", claims),
			"rsa long e":   jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa long e", claims),
			"rsa huge":     jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa huge", claims),
			"ec crv":       jwksSign(t, gojwt.SigningMethodES256, keys.p256, "ec crv", claims),
			"ec x":         jwksSign(t, gojwt.SigningMethodES256, keys.p256, "ec x", claims),
			"ec y":         jwksSign(t, gojwt.SigningMethodES256, keys.p256, "ec y", claims),
			"ec short":     jwksSign(t, gojwt.SigningMethodES256, keys.p256, "ec short", claims),
			"ec off curve": jwksSign(t, gojwt.SigningMethodES256, keys.p256, "ec off curve", claims),
			"dup":          jwksSign(t, gojwt.SigningMethodES256, keys.p256, "dup", claims),
		} {
			t.Run(name, func(t *testing.T) {
				jwksRejected(t, authServe(t, jwksBearer(token), m))
			})
		}
		result := authServe(t, jwksBearer(jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", claims)), m)
		testutil.Equal(t, "usable key served", result.served, true)
		testutil.Equal(t, "fetches", idp.count("/jwks"), 1)
	})

	t.Run("should fetch the set again on an unknown key id at most once per minimum refresh", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "old", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{})
		claims := jwtClaims("alice", nil)
		old := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "old", claims)
		rotated := jwksSign(t, gojwt.SigningMethodRS256, extra.rsa, "new", claims)

		testutil.Equal(t, "old served", authServe(t, jwksBearer(old), m).served, true)
		testutil.Equal(t, "first fetch", idp.count("/jwks"), 1)

		idp.set(http.StatusOK, jwksJWK(t, "old", &keys.rsa.PublicKey, nil), jwksJWK(t, "new", &extra.rsa.PublicKey, nil))
		clock.Advance(time.Minute)
		testutil.Equal(t, "rotated served", authServe(t, jwksBearer(rotated), m).served, true)
		testutil.Equal(t, "refetch", idp.count("/jwks"), 2)

		unknown := jwksSign(t, gojwt.SigningMethodRS256, extra.rsa, "unknown", claims)
		for range 5 {
			jwksRejected(t, authServe(t, jwksBearer(unknown), m))
		}
		testutil.Equal(t, "rate limited", idp.count("/jwks"), 2)
		testutil.Equal(t, "old still served", authServe(t, jwksBearer(old), m).served, true)

		clock.Advance(59 * time.Second)
		jwksRejected(t, authServe(t, jwksBearer(unknown), m))
		testutil.Equal(t, "still rate limited", idp.count("/jwks"), 2)

		clock.Advance(time.Second)
		jwksRejected(t, authServe(t, jwksBearer(unknown), m))
		testutil.Equal(t, "refetch after minimum refresh", idp.count("/jwks"), 3)
	})

	t.Run("should fetch the set again once the refresh time elapsed", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "old", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldJWKSRefresh: "10m", jwt.ConfigFieldLeeway: "24h"})
		old := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "old", jwtClaims("alice", nil))

		testutil.Equal(t, "served", authServe(t, jwksBearer(old), m).served, true)
		clock.Advance(10*time.Minute - time.Nanosecond)
		testutil.Equal(t, "cached", authServe(t, jwksBearer(old), m).served, true)
		testutil.Equal(t, "fetches", idp.count("/jwks"), 1)

		idp.set(http.StatusOK, jwksJWK(t, "new", &extra.rsa.PublicKey, nil))
		clock.Advance(time.Nanosecond)
		jwksRejected(t, jwksUntil(t, m, old, false)) // served until refreshed
		testutil.Equal(t, "refreshed", idp.count("/jwks"), 2)
		rotated := jwksSign(t, gojwt.SigningMethodRS256, extra.rsa, "new", jwtClaims("alice", nil))
		testutil.Equal(t, "new served", authServe(t, jwksBearer(rotated), m).served, true)
		testutil.Equal(t, "no refetch", idp.count("/jwks"), 2)
	})

	t.Run("should keep the keys of the last successful fetch", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "old", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldLeeway: "24h"})
		old := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "old", jwtClaims("alice", nil))
		testutil.Equal(t, "served", authServe(t, jwksBearer(old), m).served, true)

		idp.set(http.StatusInternalServerError)
		clock.Advance(time.Hour)
		testutil.Equal(t, "stale served", authServe(t, jwksBearer(old), m).served, true)
		jwksWaitCount(t, idp, "/jwks", 2) // the failed fetch
		for range 5 {
			testutil.Equal(t, "stale served again", authServe(t, jwksBearer(old), m).served, true)
		}
		testutil.Equal(t, "rate limited", idp.count("/jwks"), 2)

		idp.set(http.StatusOK, jwksJWK(t, "new", &extra.rsa.PublicKey, nil))
		clock.Advance(time.Minute)
		jwksRejected(t, jwksUntil(t, m, old, false))
		testutil.Equal(t, "recovered", idp.count("/jwks"), 3)
	})

	t.Run("should rate limit the failed first fetches", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t)
		idp.set(http.StatusServiceUnavailable)
		m := jwksMiddleware(t, idp, clock, container.Bag{})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		jwksRejected(t, authServe(t, jwksBearer(token), m))
		jwksRejected(t, authServe(t, jwksBearer(token), m))
		testutil.Equal(t, "fetches", idp.count("/jwks"), 1)

		idp.set(http.StatusOK, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		clock.Advance(time.Minute)
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)
		testutil.Equal(t, "fetches after minimum refresh", idp.count("/jwks"), 2)
	})

	t.Run("should fail the fetch of an invalid answer", func(t *testing.T) {
		valid := func(t *testing.T) []byte {
			data, err := json.Marshal(map[string]any{"keys": []any{jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil)}})
			testutil.NoError(t, err)
			return data
		}(t)
		plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(valid)
		}))
		t.Cleanup(plain.Close)
		for name, handler := range map[string]http.HandlerFunc{
			"status": func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write(valid)
			},
			"too large": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(append(valid, strings.Repeat(" ", jwt.JWKSMaxSize+1-len(valid))...))
			},
			"truncated": func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", strconv.Itoa(len(valid)+10))
				_, _ = w.Write(valid)
			},
			"not json": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("<html>"))
			},
			"keys not a list": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"keys":{}}`))
			},
			"no keys": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{}`))
			},
			"no usable key": func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"keys":[{"kty":"oct","kid":"rsa","k":"c2VjcmV0"}]}`))
			},
			"redirect to http": func(w http.ResponseWriter, req *http.Request) {
				http.Redirect(w, req, plain.URL+"/jwks", http.StatusFound)
			},
			"timeout": func(_ http.ResponseWriter, req *http.Request) {
				<-req.Context().Done()
			},
		} {
			t.Run(name, func(t *testing.T) {
				var hits int
				var mu sync.Mutex
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					mu.Lock()
					hits++
					mu.Unlock()
					handler(w, req)
				}))
				t.Cleanup(server.Close)
				m, err := createJwks(t, container.Bag{
					jwt.ConfigFieldAlgorithms:  []any{"RS256"},
					jwt.ConfigFieldJWKSURL:     server.URL + "/jwks",
					jwt.ConfigFieldJWKSClient:  "idp",
					jwt.ConfigFieldJWKSTimeout: "50ms",
				}, testutil.NewClock(epoch), &jwksClients{clients: map[string]*http.Client{"idp": server.Client()}})
				testutil.NoError(t, err)
				token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
				jwksRejected(t, authServe(t, jwksBearer(token), m))
				mu.Lock()
				defer mu.Unlock()
				testutil.Equal(t, "hits", hits, 1)
			})
		}
	})

	t.Run("should fail the fetch of a server not trusted by the plain client", func(t *testing.T) {
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{jwt.ConfigFieldJWKSClient: nil})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		jwksRejected(t, authServe(t, jwksBearer(token), m))
		testutil.Equal(t, "fetches", idp.count("/jwks"), 0)
	})

	t.Run("should fetch the set whatever the cancellation of the request", func(t *testing.T) {
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		req := jwksBearer(jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))).WithContext(ctx)
		testutil.Equal(t, "served", authServe(t, req, m).served, true)
		testutil.Equal(t, "fetches", idp.count("/jwks"), 1)
	})

	t.Run("should serve the cached keys while a refresh hangs", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldLeeway: "24h", jwt.ConfigFieldJWKSTimeout: "1h"})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)

		release := idp.hang()
		defer release()
		clock.Advance(time.Hour)
		done := make(chan bool, 8)
		for range 8 {
			go func() { done <- authServe(t, jwksBearer(token), m).served }()
		}
		for range 8 {
			select {
			case served := <-done:
				testutil.Equal(t, "served while refreshing", served, true)
			case <-time.After(5 * time.Second):
				t.Fatal("a cached key waited for the refresh")
			}
		}
		jwksWaitCount(t, idp, "/jwks", 2)
		release()
	})

	t.Run("should share the fetch under way among the unknown key ids", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldLeeway: "24h", jwt.ConfigFieldJWKSTimeout: "1h"})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)

		release := idp.hang()
		defer release()
		idp.set(http.StatusOK, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil), jwksJWK(t, "new", &extra.rsa.PublicKey, nil))
		clock.Advance(time.Minute)
		rotated := jwksSign(t, gojwt.SigningMethodRS256, extra.rsa, "new", jwtClaims("alice", nil))
		done := make(chan bool, 8)
		for range 8 {
			go func() { done <- authServe(t, jwksBearer(rotated), m).served }()
		}
		jwksWaitCount(t, idp, "/jwks", 2)
		select {
		case <-done:
			t.Fatal("an unknown key id did not wait for the fetch under way")
		case <-time.After(50 * time.Millisecond):
		}
		cached := make(chan bool, 1)
		go func() { cached <- authServe(t, jwksBearer(token), m).served }()
		select {
		case served := <-cached:
			testutil.Equal(t, "cached served", served, true)
		case <-time.After(5 * time.Second):
			t.Fatal("a cached key waited for the fetch")
		}
		release()
		for range 8 {
			select {
			case served := <-done:
				testutil.Equal(t, "rotated served", served, true)
			case <-time.After(5 * time.Second):
				t.Fatal("an unknown key id still waits once fetched")
			}
		}
		testutil.Equal(t, "fetches", idp.count("/jwks"), 2)
	})

	t.Run("should be read concurrently", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldLeeway: "24h"})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		unknown := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "unknown", jwtClaims("alice", nil))
		var wg sync.WaitGroup
		for i := range 16 {
			wg.Go(func() {
				if i%4 == 0 {
					clock.Advance(time.Minute)
					authServe(t, jwksBearer(unknown), m)
					return
				}
				testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)
			})
		}
		wg.Wait()
	})
}

func Test_Jwks_Discovery(t *testing.T) {
	keys := jwtKeys()

	t.Run("should fetch the set of the discovery document once", func(t *testing.T) {
		clock := testutil.NewClock(epoch)
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		issuer := idp.server.URL + "/realm"
		m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldJWKSURL: nil, jwt.ConfigFieldIssuerURL: issuer, jwt.ConfigFieldLeeway: "24h"})

		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", gojwt.MapClaims{"iss": issuer}))
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)
		clock.Advance(time.Hour)
		testutil.Equal(t, "served after refresh", authServe(t, jwksBearer(token), m).served, true)
		jwksWaitCount(t, idp, "/jwks", 2)
		testutil.Equal(t, "discoveries", idp.count("/realm/.well-known/openid-configuration"), 1)

		for name, iss := range map[string]any{
			"other issuer": "issuer",
			"no issuer":    nil,
		} {
			other := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", gojwt.MapClaims{"iss": iss}))
			t.Run(name, func(t *testing.T) {
				jwksRejected(t, authServe(t, jwksBearer(other), m))
			})
		}
	})

	t.Run("should read the document under an issuer url of a trailing slash", func(t *testing.T) {
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		issuer := idp.server.URL + "/realm/"
		idp.discovery = map[string]any{"issuer": issuer, "jwks_uri": idp.server.URL + "/jwks"}
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{jwt.ConfigFieldJWKSURL: nil, jwt.ConfigFieldIssuerURL: issuer})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", gojwt.MapClaims{"iss": issuer}))
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)
	})

	t.Run("should check the configured issuer rather than the issuer url", func(t *testing.T) {
		idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
		m := jwksMiddleware(t, idp, testutil.NewClock(epoch), container.Bag{
			jwt.ConfigFieldJWKSURL:   nil,
			jwt.ConfigFieldIssuerURL: idp.server.URL + "/realm",
			jwt.ConfigFieldIssuer:    "issuer",
		})
		token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", nil))
		testutil.Equal(t, "served", authServe(t, jwksBearer(token), m).served, true)
	})

	t.Run("should fail the fetch of an invalid discovery document", func(t *testing.T) {
		for name, tc := range map[string]struct {
			path      string
			discovery func(url string) map[string]any
		}{
			"not found": {"/other", nil},
			"other issuer": {"/realm", func(url string) map[string]any {
				return map[string]any{"issuer": url + "/other", "jwks_uri": url + "/jwks"}
			}},
			"http jwks uri": {"/realm", func(url string) map[string]any {
				return map[string]any{"issuer": url + "/realm", "jwks_uri": "http://idp.example.com/jwks"}
			}},
			"no jwks uri": {"/realm", func(url string) map[string]any { return map[string]any{"issuer": url + "/realm"} }},
			"wrong jwks uri": {"/realm", func(url string) map[string]any {
				return map[string]any{"issuer": url + "/realm", "jwks_uri": url + "/missing"}
			}},
		} {
			t.Run(name, func(t *testing.T) {
				clock := testutil.NewClock(epoch)
				idp := newJwksIdP(t, jwksJWK(t, "rsa", &keys.rsa.PublicKey, nil))
				if tc.discovery != nil {
					idp.discovery = tc.discovery(idp.server.URL)
				}
				issuer := idp.server.URL + tc.path
				m := jwksMiddleware(t, idp, clock, container.Bag{jwt.ConfigFieldJWKSURL: nil, jwt.ConfigFieldIssuerURL: issuer})
				token := jwksSign(t, gojwt.SigningMethodRS256, keys.rsa, "rsa", jwtClaims("alice", gojwt.MapClaims{"iss": issuer}))
				jwksRejected(t, authServe(t, jwksBearer(token), m))
				testutil.Equal(t, "fetches", idp.count("/jwks"), 0)

				idp.discovery = nil
				clock.Advance(time.Minute)
				result := authServe(t, jwksBearer(token), m)
				testutil.Equal(t, "served once valid", result.served, tc.path == "/realm" && name != "wrong jwks uri")
			})
		}
	})
}
