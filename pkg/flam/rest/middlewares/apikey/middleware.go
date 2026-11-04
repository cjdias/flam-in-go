package apikey

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the apikey [flam.IRestMiddleware], authenticating the
// requests by the key of its header, or else of its query parameter.
type middleware struct {
	id       string
	header   string
	query    string
	keys     []key
	optional bool
}

// key is a key accepted by a middleware, held as its SHA-256 hash, along
// with the name of its principal, empty for none.
type key struct {
	hash      [sha256.Size]byte
	principal string
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next once their
// key matches one of the middleware, with its principal, if any, in their
// context, or answering them with a 401 envelope otherwise: of
// ErrMissingKey for a request without a key, served without principal
// when the middleware is optional, or of ErrInvalidKey for a key matching
// none.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		presented := m.presented(req)
		if presented == "" {
			if m.optional {
				next.ServeHTTP(w, req)
				return
			}
			middlewareReject(w, req, ErrMissingKey)
			return
		}

		match := m.match(presented)
		if match < 0 {
			middlewareReject(w, req, ErrInvalidKey)
			return
		}
		if principal := m.keys[match].principal; principal != "" {
			req = req.WithContext(rest.WithPrincipal(req.Context(), principal))
		}
		next.ServeHTTP(w, req)
	})
}

// presented returns the key of the header of the request, or else of its
// query parameter, empty when it holds none.
func (m *middleware) presented(
	req *http.Request,
) string {
	if m.header != "" {
		if presented := req.Header.Get(m.header); presented != "" {
			return presented
		}
	}
	if m.query != "" {
		return req.URL.Query().Get(m.query)
	}
	return ""
}

// match returns the index of the key whose hash is the one of the
// presented key, or -1, comparing the hash with all of them in constant
// time, whatever the one matching.
func (m *middleware) match(
	presented string,
) int {
	hash := sha256.Sum256([]byte(presented))
	match := -1
	for i := range m.keys {
		equal := subtle.ConstantTimeCompare(hash[:], m.keys[i].hash[:])
		match = subtle.ConstantTimeSelect(equal, i, match)
	}
	return match
}

// middlewareReject answers the request with a 401 envelope of the error,
// along with the WWW-Authenticate header of the challenge.
func middlewareReject(
	w http.ResponseWriter,
	req *http.Request,
	e error,
) {
	w.Header().Set("WWW-Authenticate", Challenge)
	_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(http.StatusUnauthorized, e))
}
