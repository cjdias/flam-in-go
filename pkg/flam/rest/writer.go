package rest

import (
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewResponseWriter returns the [flam.IRestResponseWriter] writing through
// w, or w when already one, so that the middlewares of an endpoint share it. It
// implements [http.Flusher], and returns w from its Unwrap method, for
// [http.ResponseController].
func NewResponseWriter(
	w http.ResponseWriter,
) flam.IRestResponseWriter {
	if rw, ok := w.(flam.IRestResponseWriter); ok {
		return rw
	}
	return &responseWriter{ResponseWriter: w}
}

// responseWriter is the [flam.IRestResponseWriter] returned by
// [NewResponseWriter].
type responseWriter struct {
	http.ResponseWriter

	status int
	size   int
}

// compile time check that *responseWriter implements Flusher
var _ http.Flusher = (*responseWriter)(nil)

func (w *responseWriter) Status() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *responseWriter) Size() int {
	return w.size
}

func (w *responseWriter) Written() bool {
	return w.status != 0
}

// WriteHeader writes the status, recorded unless informational, as an
// informational one may be followed by another.
func (w *responseWriter) WriteHeader(
	status int,
) {
	if w.status == 0 && (status < 100 || status > 199) {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write writes a part of the body, recording the status 200 when none was
// written.
func (w *responseWriter) Write(
	data []byte,
) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(data)
	w.size += n
	return n, err
}

// Flush flushes the response through the writer it wraps, when it can,
// recording the status 200 when none was written, as flushing writes it.
func (w *responseWriter) Flush() {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer it wraps, for http.ResponseController.
func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
