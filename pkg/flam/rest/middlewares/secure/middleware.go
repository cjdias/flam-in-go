package secure

import (
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// middleware is the secure [flam.IRestMiddleware], setting its headers on
// the responses, along with its Strict-Transport-Security header on the
// responses to the requests received over TLS, or to every request when
// forced.
type middleware struct {
	id        string
	headers   []header
	hsts      string
	forceHSTS bool
}

// header is a header set by a middleware.
type header struct {
	name  string
	value string
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler setting the headers of the middleware on the
// response, then serving the requests through next.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h := w.Header()
		for _, header := range m.headers {
			h.Set(header.name, header.value)
		}
		if m.hsts != "" && (m.forceHSTS || req.TLS != nil) {
			h.Set("Strict-Transport-Security", m.hsts)
		}
		next.ServeHTTP(w, req)
	})
}
