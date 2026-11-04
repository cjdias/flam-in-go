package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the idempotency [flam.IRestMiddleware], storing the
// responses to the requests of its methods holding an idempotency key in
// its cache adaptor, to replay them, along with the keys of the requests
// in flight in the process.
type middleware struct {
	id          string
	cache       flam.ICacheAdaptor
	header      string
	methods     map[string]struct{}
	required    bool
	ttl         time.Duration
	lockTTL     time.Duration
	maxBodySize int
	headers     []string

	mu       sync.Mutex
	inFlight map[string]struct{} // of the record ids in flight
}

// record is the response stored under the id of an idempotency key, or
// the marker of the request in flight under it, along with the hash of the
// body of its request. Its fields are exported for the serializer of the
// cache adaptor, its Id keying it for the default key generator.
type record struct {
	Id      string
	Hash    string
	Pending bool
	Status  int
	Header  http.Header
	Body    []byte
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests of the methods holding an
// idempotency key once only, replaying the response stored to the
// repeated ones, answering with an envelope the ones whose key is missing,
// when required, invalid, in flight, or reused for another body, and
// serving the other requests through next as they are. It returns next
// itself for a streaming endpoint, whose response cannot be stored.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	if endpoint.Streaming {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if _, ok := m.methods[req.Method]; !ok {
			next.ServeHTTP(w, req)
			return
		}
		key := req.Header.Get(m.header)
		switch {
		case key == "" && !m.required:
			next.ServeHTTP(w, req)
			return
		case key == "":
			middlewareFail(w, req, http.StatusBadRequest, ErrMissingKey)
			return
		case len(key) > MaxKeyLength:
			middlewareFail(w, req, http.StatusBadRequest, ErrInvalidKey)
			return
		}

		hash, ok := middlewareHash(w, req)
		if !ok {
			return
		}
		id := m.recordId(req, key)
		if !m.acquire(id) {
			middlewareFail(w, req, http.StatusConflict, ErrInFlight)
			return
		}
		defer m.release(id)
		m.serve(w, req, next, id, hash)
	})
}

// serve replays the response stored under the id, or answers with an
// envelope when the request stored is still in flight, or of another
// body, or else serves the request through next, under the marker of the
// requests in flight, storing its response unless of a 5xx status or
// longer than the maximum body size, the marker being removed then, as
// when next panics, so that the request can be made again.
func (m *middleware) serve(
	w http.ResponseWriter,
	req *http.Request,
	next http.Handler,
	id string,
	hash string,
) {
	ctx := req.Context()
	stored := &record{Id: id}
	found, err := m.cache.Get(ctx, stored)
	if err != nil {
		middlewareFail(w, req, http.StatusServiceUnavailable, ErrCache)
		return
	}
	if found[0] != nil {
		switch {
		case stored.Hash != hash:
			middlewareFail(w, req, http.StatusUnprocessableEntity, ErrKeyReused)
		case stored.Pending:
			middlewareFail(w, req, http.StatusConflict, ErrInFlight)
		default:
			middlewareReplay(w, stored)
		}
		return
	}
	if err := m.cache.SetEphemeral(ctx, m.lockTTL, &record{Id: id, Hash: hash, Pending: true}); err != nil {
		middlewareFail(w, req, http.StatusServiceUnavailable, ErrCache)
		return
	}

	// the cache is updated even once the client went away
	ctx = context.WithoutCancel(ctx)
	kept := false
	defer func() {
		if !kept {
			_ = m.cache.Del(ctx, &record{Id: id})
		}
	}()
	rw := &writer{IRestResponseWriter: rest.NewResponseWriter(w), max: m.maxBodySize}
	next.ServeHTTP(rw, req)
	if status := rw.Status(); status < 500 && !rw.truncated {
		kept = m.cache.SetEphemeral(ctx, m.ttl, &record{
			Id:     id,
			Hash:   hash,
			Status: status,
			Header: m.selected(rw.Header()),
			Body:   rw.body.Bytes(),
		}) == nil
	}
}

// recordId returns the id of the record of the idempotency key of the
// request, scoped by the middleware, the principal of the request, its
// method and its path, hashed so that it is of a fixed length whatever
// they hold.
func (m *middleware) recordId(
	req *http.Request,
	key string,
) string {
	principal, _ := rest.PrincipalOf(req.Context())
	h := sha256.New()
	for _, part := range []string{principal, req.Method, req.URL.Path, key} {
		// each part prefixed by its length, so that no two lists of parts
		// hash alike
		_, _ = io.WriteString(h, strconv.Itoa(len(part))+":"+part)
	}
	return m.id + ":" + hex.EncodeToString(h.Sum(nil))
}

// acquire marks the record id as in flight in the process, reporting
// whether it was not already.
func (m *middleware) acquire(
	id string,
) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.inFlight[id]; ok {
		return false
	}
	m.inFlight[id] = struct{}{}
	return true
}

// release unmarks the record id as in flight in the process.
func (m *middleware) release(
	id string,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.inFlight, id)
}

// selected returns a copy of the headers of the middleware held by the
// header of a response.
func (m *middleware) selected(
	header http.Header,
) http.Header {
	selected := http.Header{}
	for _, name := range m.headers {
		if values := header.Values(name); len(values) > 0 {
			selected[name] = slices.Clone(values)
		}
	}
	return selected
}

// middlewareHash returns the hexadecimal SHA-256 hash of the body of the
// request, read whole, then given back to the request, or answers the
// request with an envelope of ErrReadBody when it cannot be read,
// reporting whether it could.
func middlewareHash(
	w http.ResponseWriter,
	req *http.Request,
) (string, bool) {
	var body []byte
	if req.Body != nil {
		var err error
		if body, err = io.ReadAll(req.Body); err != nil {
			status := http.StatusBadRequest
			if tooLarge := (*http.MaxBytesError)(nil); errors.As(err, &tooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			middlewareFail(w, req, status, ErrReadBody)
			return "", false
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), true
}

// middlewareReplay writes the response stored, along with the
// ReplayedHeader header.
func middlewareReplay(
	w http.ResponseWriter,
	stored *record,
) {
	header := w.Header()
	maps.Copy(header, stored.Header)
	header.Set(ReplayedHeader, "true")
	w.WriteHeader(stored.Status)
	if len(stored.Body) > 0 {
		_, _ = w.Write(stored.Body)
	}
}

// middlewareFail answers the request with an envelope of the status and
// the error.
func middlewareFail(
	w http.ResponseWriter,
	req *http.Request,
	status int,
	err error,
) {
	_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(status, err))
}
