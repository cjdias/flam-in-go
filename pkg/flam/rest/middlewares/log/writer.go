package log

import (
	"bytes"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// writer is the [flam.IRestResponseWriter] of a log middleware, writing
// through the one it embeds while keeping up to max bytes of the body
// written, for the entry of the response, when the status is not the
// expected one, as the body is only logged then. It implements
// [http.Flusher], and returns the writer it embeds from its Unwrap method,
// for [http.ResponseController].
type writer struct {
	flam.IRestResponseWriter

	max       int
	expected  int
	body      bytes.Buffer
	truncated bool
}

// compile time checks that *writer implements IRestResponseWriter and
// Flusher
var (
	_ flam.IRestResponseWriter = (*writer)(nil)
	_ http.Flusher             = (*writer)(nil)
)

// Write writes a part of the body, keeping what fits in the bytes kept,
// and recording the truncation of the rest, when the status written is not
// the expected one, so that the responses of the expected status allocate
// nothing.
func (w *writer) Write(
	data []byte,
) (int, error) {
	n, err := w.IRestResponseWriter.Write(data)
	if w.IRestResponseWriter.Status() == w.expected {
		return n, err
	}
	written := data[:n]
	if room := w.max - w.body.Len(); len(written) > room {
		written = written[:room]
		w.truncated = true
	}
	w.body.Write(written)
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
