package timeout

import (
	"bytes"
	"maps"
	"net/http"
	"sync"
)

// writer is the [http.ResponseWriter] given to the handler by a timeout
// middleware, through a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], buffering
// the response until the handler returns, then sent, or the context of
// the request is done, after which its writes fail with
// [http.ErrHandlerTimeout]. Its header starts as a copy of the one of the
// response, so that the handler sees the headers set by the middlewares
// listed before. It implements neither [http.Flusher] nor an Unwrap
// method, as the response is only sent once the handler returns.
type writer struct {
	mu      sync.Mutex
	header  http.Header
	body    bytes.Buffer
	status  int
	expired bool
}

// compile time check that *writer implements ResponseWriter
var _ http.ResponseWriter = (*writer)(nil)

// Header returns the header buffered, sent once the handler returns.
func (w *writer) Header() http.Header {
	return w.header
}

// WriteHeader buffers the status, unless one was, the context is done, or
// it is informational, as an informational status cannot be sent before
// the handler returns.
func (w *writer) WriteHeader(
	status int,
) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.expired || w.status != 0 || (status >= 100 && status <= 199) {
		return
	}
	w.status = status
}

// Write buffers a part of the body, along with the status 200 when none
// was, failing with [http.ErrHandlerTimeout] once the context is done.
func (w *writer) Write(
	data []byte,
) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.expired {
		return 0, http.ErrHandlerTimeout
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(data)
}

// expire records that the context is done, failing the writes from then
// on.
func (w *writer) expire() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.expired = true
}

// send writes the response buffered to dst, its header in place of the one
// of dst, and its status, 200 when none was buffered, as the server then
// answers with it.
func (w *writer) send(
	dst http.ResponseWriter,
) {
	w.mu.Lock()
	defer w.mu.Unlock()
	header := dst.Header()
	clear(header)
	maps.Copy(header, w.header)
	status := w.status
	if status == 0 {
		status = http.StatusOK
	}
	dst.WriteHeader(status)
	if w.body.Len() > 0 {
		_, _ = dst.Write(w.body.Bytes())
	}
}
