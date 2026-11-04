package requestid

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the request id [flam.IRestMiddleware], reading the id of
// the requests from its header, or generating it, and setting it on the
// header of the responses.
type middleware struct {
	id     string
	header string
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next, with the id
// of the request, read from the header when valid, or generated otherwise,
// stored in the context of the request and set on the header of the
// response.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		id := req.Header.Get(m.header)
		if !middlewareValid(id) {
			id = middlewareGenerate()
		}
		w.Header().Set(m.header, id)
		next.ServeHTTP(w, req.WithContext(rest.WithRequestId(req.Context(), id)))
	})
}

// middlewareValid reports whether the id read from a request is valid: not
// empty, of at most [MaxLength] bytes, each an ASCII letter, a digit or
// one of "-_.:+/=".
func middlewareValid(
	id string,
) bool {
	if id == "" || len(id) > MaxLength {
		return false
	}
	for i := range len(id) {
		switch c := id[i]; {
		case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		case c == '-', c == '_', c == '.', c == ':', c == '+', c == '/', c == '=':
		default:
			return false
		}
	}
	return true
}

// middlewareGenerate returns a new id of 128 random bits, as 32 lower case
// hexadecimal digits. [rand.Read] never fails, crashing the program
// irrecoverably instead when the system provides no randomness.
func middlewareGenerate() string {
	var id [16]byte
	_, _ = rand.Read(id[:])
	return hex.EncodeToString(id[:])
}
