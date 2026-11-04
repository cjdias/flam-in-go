package timeout

import (
	"context"
	"net/http"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the timeout [flam.IRestMiddleware], bounding the time
// taken by the handlers to its timeout.
type middleware struct {
	id      string
	timeout time.Duration
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next, run in its
// own goroutine with a request whose context expires after the timeout,
// writing the response buffered once next returns, or a 503 envelope when
// the context is done before, and panicking with the value next panicked
// with before.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), m.timeout)
		defer cancel()
		req = req.WithContext(ctx)

		tw := &writer{header: w.Header().Clone()}
		done := make(chan struct{})
		panicked := make(chan any, 1)
		go func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					panicked <- recovered
				}
			}()
			next.ServeHTTP(rest.NewResponseWriter(tw), req)
			close(done)
		}()

		select {
		case recovered := <-panicked:
			panic(recovered)
		case <-done:
			tw.send(w)
		case <-ctx.Done():
			tw.expire()
			_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(http.StatusServiceUnavailable, ErrTimeout))
		}
	})
}
