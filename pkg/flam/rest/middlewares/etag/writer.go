package etag

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
)

// writer is the [http.ResponseWriter] given to the handler of a GET or
// HEAD request by an etag middleware, through a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], buffering
// the body of a 200 response, up to the maximum size, until the handler
// returns, to tag it, or answer with a 304 when the If-None-Match header
// of the request matches its tag. It implements [http.Flusher], and
// returns the writer it wraps from its Unwrap method, for
// [http.ResponseController].
type writer struct {
	http.ResponseWriter

	maxSize     int
	strong      bool
	head        bool
	ifNoneMatch string
	status      int
	buf         []byte
	state       int
}

// States of a writer, the zero one being the state of a writer given no
// status yet.
const (
	// writerBuffered is the state of a writer buffering the body of a 200
	// response.
	writerBuffered = iota + 1

	// writerPlain is the state of a writer sending the body as it is.
	writerPlain

	// writerDropped is the state of a writer having answered with a 304,
	// dropping the body.
	writerDropped
)

// compile time checks that *writer implements ResponseWriter and Flusher
var (
	_ http.ResponseWriter = (*writer)(nil)
	_ http.Flusher        = (*writer)(nil)
)

// WriteHeader records the status, unless one was. A 200 response is
// buffered, unless the handler set its entity tag, in which case it is
// answered at once, with a 304 when the tag matches. Any other status is
// written through, an informational one along with the status to come.
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
	switch etag := w.Header().Get("ETag"); {
	case status != http.StatusOK:
		w.state = writerPlain
		w.ResponseWriter.WriteHeader(status)
	case etag != "":
		w.send(etag)
	default:
		w.state = writerBuffered
	}
}

// Write writes a part of the body, recording the status 200 when none
// was, buffered up to the maximum size, past which the response buffered
// is sent as it is, dropped once answered with a 304.
func (w *writer) Write(
	data []byte,
) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	switch w.state {
	case writerPlain:
		return w.ResponseWriter.Write(data)
	case writerDropped:
		return len(data), nil
	}
	if len(w.buf)+len(data) <= w.maxSize {
		w.buf = append(w.buf, data...)
		return len(data), nil
	}
	if err := w.spill(); err != nil {
		return 0, err
	}
	return w.ResponseWriter.Write(data)
}

// Flush sends the response buffered as it is, untagged, as the handler
// streams it, recording the status 200 when none was, then flushes the
// writer it wraps, when it can.
func (w *writer) Flush() {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	if w.state == writerBuffered {
		_ = w.spill()
	}
	_ = http.NewResponseController(w.ResponseWriter).Flush()
}

// Unwrap returns the writer it wraps, for http.ResponseController.
func (w *writer) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// close tags the response buffered once the handler returned, with a weak
// tag unless strong, unless the handler set its tag after its status, then
// sends it, or answers with a 304 when the tag matches. The response to a HEAD request whose handler
// wrote no body is sent untagged, as its tag would not be the one of the
// response to the GET request.
func (w *writer) close() {
	if w.state != writerBuffered {
		return
	}
	if w.head && len(w.buf) == 0 {
		_ = w.spill()
		return
	}
	etag := w.Header().Get("ETag")
	if etag == "" {
		sum := sha256.Sum256(w.buf)
		etag = `"` + hex.EncodeToString(sum[:TagLength]) + `"`
		if !w.strong {
			etag = "W/" + etag
		}
		w.Header().Set("ETag", etag)
	}
	w.send(etag)
	if w.state == writerPlain && len(w.buf) > 0 {
		_, _ = w.ResponseWriter.Write(w.buf)
	}
	w.buf = nil
}

// send writes the status, or a 304 when the If-None-Match header of the
// request matches the entity tag, without the headers describing the body,
// as [http.ServeContent] does, the body being dropped from then on.
func (w *writer) send(
	etag string,
) {
	if middlewareMatch(w.ifNoneMatch, etag) {
		header := w.Header()
		header.Del("Content-Type")
		header.Del("Content-Length")
		header.Del("Content-Encoding")
		header.Del("Last-Modified")
		w.state = writerDropped
		w.ResponseWriter.WriteHeader(http.StatusNotModified)
		return
	}
	w.state = writerPlain
	w.ResponseWriter.WriteHeader(w.status)
}

// spill sends the response buffered as it is, untagged.
func (w *writer) spill() error {
	w.state = writerPlain
	w.ResponseWriter.WriteHeader(w.status)
	buf := w.buf
	w.buf = nil
	if len(buf) == 0 {
		return nil
	}
	_, err := w.ResponseWriter.Write(buf)
	return err
}
