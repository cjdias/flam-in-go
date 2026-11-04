package bodylimit

import (
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the body limit [flam.IRestMiddleware], limiting the bodies
// of the requests to its maximum number of bytes.
type middleware struct {
	id       string
	maxBytes int64
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler answering the requests announcing a body longer
// than the maximum with a 413 envelope, and serving the others through
// next, their body, if any, limited to the maximum.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.ContentLength > m.maxBytes {
			_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(http.StatusRequestEntityTooLarge, ErrTooLarge))
			return
		}
		if req.Body != nil && req.Body != http.NoBody {
			req.Body = http.MaxBytesReader(w, req.Body, m.maxBytes)
		}
		next.ServeHTTP(w, req)
	})
}
