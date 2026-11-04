package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// jwks is the keyring of a JSON Web Key Set, fetched from its URL, or from
// the URL the discovery document of its issuer gives, when a token first
// needs it, and fetched again once its refresh time elapsed, or when a
// token names a key it does not hold, at most once per minimum refresh
// time, whether the fetch succeeds or not. A failed fetch keeps the keys
// of the last successful one.
//
// A single fetch runs at a time, on a goroutine of its own, never under
// the lock of the keyring: the tokens naming a key it holds are verified
// with it meanwhile, even once its refresh time elapsed, while those
// naming a key it does not hold wait for the fetch under way, bounded by
// the timeout.
type jwks struct {
	clock      flam.IClock
	client     jwksClient
	issuer     string
	refresh    time.Duration
	minRefresh time.Duration
	timeout    time.Duration

	// url is the URL of the key set, once discovered, only read and
	// written by the fetch under way
	url string

	mu        sync.RWMutex // guards the fields below, never held during a fetch
	keys      map[string]jwksKey
	fetched   time.Time
	attempted time.Time
	fetching  chan struct{} // closed once the fetch under way ends, nil without one
}

// jwksClient sends the requests fetching the discovery document and the
// key set, an [*http.Client] or a [flam.IClient].
type jwksClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// jwksKey is a key of a key set, along with the algorithm its "alg"
// member restricts it to, empty for none.
type jwksKey struct {
	value     any
	algorithm string
}

// key returns the key of the set named by the "kid" header of the token,
// starting a fetch of the set when it was never fetched, its refresh time
// elapsed, or it does not hold the key, unless a fetch is under way or was
// attempted within the minimum refresh time. A key the set holds is
// returned at once, while the fetch of a key it does not hold is waited
// for. The key must fit the algorithm of the token, and be restricted to
// it when its "alg" member is set.
func (s *jwks) key(
	ctx context.Context,
	token *gojwt.Token,
) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, errNoKid
	}

	key, found, fetching := s.lookup(ctx, kid)
	if !found && fetching != nil {
		<-fetching // bounded by the timeout
		s.mu.RLock()
		key, found = s.keys[kid]
		s.mu.RUnlock()
	}

	algorithm := token.Method.Alg()
	switch {
	case !found:
		return nil, errUnknownKid
	case key.algorithm != "" && key.algorithm != algorithm:
		return nil, errKeyMismatch
	}
	if err := creatorFits(algorithm, key.value); err != nil {
		return nil, err
	}
	return key.value, nil
}

// lookup returns the key of the set named by the kid, and whether the set
// holds it, along with the channel closed once the fetch under way ends,
// nil without one, starting it when the set was never fetched, its refresh
// time elapsed, or it does not hold the key, unless a fetch is under way
// or was attempted within the minimum refresh time.
func (s *jwks) lookup(
	ctx context.Context,
	kid string,
) (jwksKey, bool, <-chan struct{}) {
	s.mu.RLock()
	now := s.clock.Now()
	key, found := s.keys[kid]
	fresh := s.keys != nil && now.Sub(s.fetched) < s.refresh
	s.mu.RUnlock()
	if found && fresh {
		return key, true, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key, found = s.keys[kid] // a fetch may have ended meanwhile
	stale := s.keys == nil || now.Sub(s.fetched) >= s.refresh
	if s.fetching == nil && (stale || !found) && now.Sub(s.attempted) >= s.minRefresh {
		s.attempted = now
		s.fetching = make(chan struct{})
		go s.update(ctx, now, s.fetching)
	}
	return key, found, s.fetching
}

// update fetches the set, keeping its keys, fetched at the time, when the
// fetch succeeds, then ends the fetch under way, closing its channel.
func (s *jwks) update(
	ctx context.Context,
	now time.Time,
	fetching chan struct{},
) {
	keys, err := s.fetch(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err == nil {
		s.keys, s.fetched = keys, now
	}
	s.fetching = nil
	close(fetching)
}

// fetch returns the usable keys of the set, discovering its URL first
// when not known yet, bounded by the timeout whatever the context of the
// request, which may be canceled, while the keys serve other ones.
func (s *jwks) fetch(
	ctx context.Context,
) (map[string]jwksKey, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
	defer cancel()

	if s.url == "" {
		var document struct {
			Issuer  string `json:"issuer"`
			JWKSURI string `json:"jwks_uri"`
		}
		address := strings.TrimSuffix(s.issuer, "/") + jwksDiscoveryPath
		if err := s.get(ctx, address, &document); err != nil {
			return nil, err
		}
		if document.Issuer != s.issuer {
			return nil, errDiscoveryIssuer
		}
		if !creatorHTTPS(document.JWKSURI) {
			return nil, errNotHTTPS
		}
		s.url = document.JWKSURI
	}

	var set struct {
		Keys []jwksMember `json:"keys"`
	}
	if err := s.get(ctx, s.url, &set); err != nil {
		return nil, err
	}
	keys := jwksKeys(set.Keys)
	if len(keys) == 0 {
		return nil, errNoKeys
	}
	return keys, nil
}

// get decodes into the value the JSON of the 200 answer to the GET request
// of the address, failing when the answer is of another status, comes
// from a URL other than an https one, such as after a redirect, or is
// larger than JWKSMaxSize.
func (s *jwks) get(
	ctx context.Context,
	address string,
	value any,
) error {
	// the address is an absolute https URL, checked before, which
	// NewRequestWithContext parses as url.Parse did, without failing
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode != http.StatusOK:
		return errStatus
	case resp.Request != nil && resp.Request.URL.Scheme != "https":
		return errNotHTTPS
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, JWKSMaxSize+1))
	if err != nil {
		return err
	}
	if len(body) > JWKSMaxSize {
		return errTooLarge
	}
	return json.Unmarshal(body, value)
}

// jwksMember is a JSON Web Key of a key set, as RFC 7517 and RFC 7518
// define it, of its members read by the middleware: the private ones, and
// the certificates, are ignored.
type jwksMember struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// jwksKeys returns the usable keys of the members, by their id: the
// signature keys, of a "use" member of "sig" or unset, with an id, of a
// supported type and curve, and well encoded. The members sharing an id
// are all dropped, as which one a token names is unknown.
func jwksKeys(
	members []jwksMember,
) map[string]jwksKey {
	keys := map[string]jwksKey{}
	shared := map[string]bool{}
	for _, member := range members {
		if member.Kid == "" || (member.Use != "" && member.Use != "sig") {
			continue
		}
		value, ok := member.public()
		if !ok {
			continue
		}
		if _, dup := keys[member.Kid]; dup || shared[member.Kid] {
			delete(keys, member.Kid)
			shared[member.Kid] = true
			continue
		}
		keys[member.Kid] = jwksKey{value: value, algorithm: member.Alg}
	}
	return keys
}

// public returns the public key of the member, an RSA one, of 8192 bits
// at most to bound the time of its verifications, an ECDSA P-256, P-384 or
// P-521 one, or an Ed25519 one, and whether it is one of them, well
// encoded.
func (k jwksMember) public() (any, bool) {
	switch k.Kty {
	case "RSA":
		n, nErr := jwksDecode(k.N)
		e, eErr := jwksDecode(k.E)
		if nErr != nil || eErr != nil || len(n) == 0 || len(n) > jwksMaxRSABytes || len(e) == 0 || len(e) > 4 {
			return nil, false
		}
		exponent := 0
		for _, b := range e {
			exponent = exponent<<8 | int(b)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exponent}, true
	case "EC":
		curve, ok := jwksCurves[k.Crv]
		x, xErr := jwksDecode(k.X)
		y, yErr := jwksDecode(k.Y)
		size := (curve.bits + 7) / 8
		if !ok || xErr != nil || yErr != nil || len(x) != size || len(y) != size {
			return nil, false
		}
		point := append(append([]byte{4}, x...), y...)
		key, err := ecdsa.ParseUncompressedPublicKey(curve.curve(), point)
		return key, err == nil
	case "OKP":
		x, err := jwksDecode(k.X)
		if k.Crv != "Ed25519" || err != nil || len(x) != ed25519.PublicKeySize {
			return nil, false
		}
		return ed25519.PublicKey(x), true
	}
	return nil, false
}

// jwksDecode returns the bytes of the unpadded base64url value.
func jwksDecode(
	value string,
) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(value)
}

// jwksCurves are the curves of the ECDSA keys, by their "crv" name, along
// with their size in bits.
var jwksCurves = map[string]struct {
	curve func() elliptic.Curve
	bits  int
}{
	"P-256": {elliptic.P256, 256},
	"P-384": {elliptic.P384, 384},
	"P-521": {elliptic.P521, 521},
}

// jwksMaxRSABytes is the maximum size, in bytes, of the modulus of an RSA
// key of a key set, 8192 bits.
const jwksMaxRSABytes = 1024

// jwksDiscoveryPath is the path of the OpenID Connect discovery document,
// under the issuer URL.
const jwksDiscoveryPath = "/.well-known/openid-configuration"

// Causes of the failures to give a key of a key set, never sent to the
// clients.
var (
	errNoKid           = errors.New("token without key id")
	errUnknownKid      = errors.New("key id not in the key set")
	errDiscoveryIssuer = errors.New("discovery document of another issuer")
	errNoKeys          = errors.New("key set without usable key")
	errStatus          = errors.New("unexpected answer status")
	errTooLarge        = errors.New("answer larger than the maximum size")
)
