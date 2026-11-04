package compress

import (
	"io"
	"net/http"
)

// writer is the [http.ResponseWriter] given to the handler by a compress
// middleware, through a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], buffering
// the start of the body until it reaches the minimum size of the
// middleware, the response being then compressed with the encoding, or
// until the handler flushes or returns, a response below the minimum size
// being then sent uncompressed. It implements [http.Flusher], and returns
// the writer it wraps from its Unwrap method, for
// [http.ResponseController].
type writer struct {
	http.ResponseWriter

	m        *middleware
	encoding string
	status   int
	buf      []byte
	state    int
	zw       compressor
}

// compressor is the writer of a compressed stream, a [*compress/gzip.Writer]
// or a [*compress/flate.Writer], reused through the pools of the
// middleware.
type compressor interface {
	io.WriteCloser

	Flush() error
	Reset(dst io.Writer)
}

// States of a writer.
const (
	// writerPending is the state of a writer buffering the body until it
	// decides whether to compress it.
	writerPending = iota

	// writerPlain is the state of a writer sending the body as it is.
	writerPlain

	// writerCompressed is the state of a writer compressing the body.
	writerCompressed
)

// compile time checks that *writer implements ResponseWriter and Flusher
var (
	_ http.ResponseWriter = (*writer)(nil)
	_ http.Flusher        = (*writer)(nil)
)

// WriteHeader records the status, unless one was, the response being
// sent as it is from then on when it cannot be compressed, while an
// informational status is written through at once.
func (w *writer) WriteHeader(
	status int,
) {
	if w.status != 0 {
		return
	}
	if status >= 100 && status <= 199 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	w.status = status
	if !w.compressible() {
		_ = w.start(false)
	}
}

// Write writes a part of the body, recording the status 200 when none
// was, buffered until the decision to compress the response is taken, at
// the minimum size.
func (w *writer) Write(
	data []byte,
) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	switch w.state {
	case writerPlain:
		return w.ResponseWriter.Write(data)
	case writerCompressed:
		return w.zw.Write(data)
	}
	w.buf = append(w.buf, data...)
	if len(w.buf) < w.m.minSize {
		return len(data), nil
	}
	if err := w.decide(false); err != nil {
		return 0, err
	}
	return len(data), nil
}

// Flush sends the response buffered, compressed when it can be, whatever
// its size, as the handler streams it, recording the status 200 when none
// was, then flushes the compressor and the writer it wraps, when it can.
func (w *writer) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.state == writerPending {
		_ = w.decide(true)
	}
	if w.state == writerCompressed {
		_ = w.zw.Flush()
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer it wraps, for http.ResponseController.
func (w *writer) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// close sends the response still buffered once the handler returned,
// uncompressed as below the minimum size, then closes the compressor,
// put back in its pool.
func (w *writer) close() {
	if w.state == writerPending && w.status != 0 {
		_ = w.decide(false)
	}
	if w.state == writerCompressed {
		_ = w.zw.Close()
		w.zw.Reset(io.Discard)
		w.m.pools[w.encoding].Put(w.zw)
		w.zw = nil
	}
}

// compressible reports whether the response may be compressed by its
// status and headers: a status allowing a body, no Content-Encoding nor
// Content-Range header, and a Content-Type of the middleware, when set.
func (w *writer) compressible() bool {
	header := w.Header()
	if w.status == http.StatusNoContent || w.status == http.StatusNotModified ||
		header.Get("Content-Encoding") != "" || header.Get("Content-Range") != "" {
		return false
	}
	_, typed := header["Content-Type"]
	return !typed || w.m.allowed(header.Get("Content-Type"))
}

// decide sends the response buffered, compressed when it may be, of a
// Content-Type of the middleware, sniffed from the body buffered when
// unset, as the server would, and when the body buffered reaches the
// minimum size, or the handler flushes it.
func (w *writer) decide(
	flushing bool,
) error {
	header := w.Header()
	if _, typed := header["Content-Type"]; !typed && len(w.buf) > 0 {
		header.Set("Content-Type", http.DetectContentType(w.buf))
	}
	return w.start(w.compressible() && w.m.allowed(header.Get("Content-Type")) &&
		(flushing || len(w.buf) >= w.m.minSize))
}

// start writes the status, along with the headers of the compressed
// response when compressing it, then the body buffered.
func (w *writer) start(
	compress bool,
) error {
	var out io.Writer = w.ResponseWriter
	w.state = writerPlain
	if compress {
		header := w.Header()
		header.Del("Content-Length")
		header.Del("Accept-Ranges")
		header.Set("Content-Encoding", w.encoding)
		w.zw = w.m.pools[w.encoding].Get().(compressor)
		w.zw.Reset(w.ResponseWriter)
		w.state = writerCompressed
		out = w.zw
	}
	w.ResponseWriter.WriteHeader(w.status)
	buf := w.buf
	w.buf = nil
	if len(buf) == 0 {
		return nil
	}
	_, err := out.Write(buf)
	return err
}
