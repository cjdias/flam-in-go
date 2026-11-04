package idempotency

import (
	"bytes"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// writer is the [flam.IRestResponseWriter] of an idempotency middleware,
// writing through the one it embeds while keeping up to max bytes of the
// body written, to store the response, and recording whether the body
// was longer. It implements [http.Flusher], and returns the writer it
// embeds from its Unwrap method, for [http.ResponseController].
type writer struct {
	flam.IRestResponseWriter

	max       int
	body      bytes.Buffer
	truncated bool
}

// compile time checks that *writer implements IRestResponseWriter and
// Flusher
var (
	_ flam.IRestResponseWriter = (*writer)(nil)
	_ http.Flusher             = (*writer)(nil)
)

// Write writes a part of the body, keeping it unless the body kept would
// then be longer than the maximum, which records the truncation, after
// which nothing is kept, as the response is not stored.
func (w *writer) Write(
	data []byte,
) (int, error) {
	n, err := w.IRestResponseWriter.Write(data)
	if w.truncated {
		return n, err
	}
	if w.body.Len()+n > w.max {
		w.truncated = true
		w.body = bytes.Buffer{}
		return n, err
	}
	w.body.Write(data[:n])
	return n, err
}

// Flush flushes the response through the writer it embeds, when it can.
func (w *writer) Flush() {
	_ = http.NewResponseController(w.IRestResponseWriter).Flush()
}

// Unwrap returns the writer it embeds, for http.ResponseController.
func (w *writer) Unwrap() http.ResponseWriter {
	return w.IRestResponseWriter
}
