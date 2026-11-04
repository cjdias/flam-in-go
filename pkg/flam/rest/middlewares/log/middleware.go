package log

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the log [flam.IRestMiddleware], signaling the requests it
// receives, and their responses, on its channel of the logger.
type middleware struct {
	id            string
	logger        flam.ILogger
	clock         flam.IClock
	channel       string
	levelRequest  flam.LogLevel
	levelResponse flam.LogLevel
	levelError    flam.LogLevel
	parseJSON     bool
	maxBody       int
	redactHeaders map[string]struct{}
	redactParams  map[string]struct{}
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next, signaling
// each when received, then once served, even when next panics, whose
// panic then goes on. The body of a request expecting a "100-continue" is
// not read, as reading it would have the client send it before the
// handler may reject it. The body of the response is kept, up to the
// maximum body size, through a [flam.IRestResponseWriter] wrapping the one
// of the request, only when its status is not the one of the endpoint, as
// it is then logged.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	expected := endpoint.Status
	if expected == 0 {
		expected = http.StatusOK
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		request := m.request(endpoint, req)
		m.logger.Signal(m.levelRequest, m.channel, middlewareRequestMessage, container.Bag{
			ContextFieldRequest: request,
		})

		rw := rest.NewResponseWriter(w)
		var tee *writer
		if m.maxBody > 0 {
			tee = &writer{IRestResponseWriter: rw, max: m.maxBody, expected: expected}
			rw = tee
		}
		start := m.clock.Now()
		panicked := true
		defer func() {
			m.signal(request, rw, tee, expected, m.clock.Since(start), panicked)
		}()
		next.ServeHTTP(rw, req)
		panicked = false
	})
}

// request returns the bag describing the request served by the endpoint,
// with the id of the request when its context holds one, and its body
// when bodies are logged and the request does not expect a
// "100-continue".
func (m *middleware) request(
	endpoint flam.RestEndpoint,
	req *http.Request,
) container.Bag {
	params := container.Bag{}
	for name, values := range req.URL.Query() {
		if _, ok := m.redactParams[strings.ToLower(name)]; ok {
			params[name] = RedactedValue
			continue
		}
		params[name] = middlewareValues(values)
	}
	request := container.Bag{
		ContextFieldMethod:   req.Method,
		ContextFieldPath:     req.URL.Path,
		ContextFieldPattern:  endpoint.Pattern,
		ContextFieldEndpoint: endpoint.Id,
		ContextFieldService:  endpoint.ServiceId,
		ContextFieldRemote:   req.RemoteAddr,
		ContextFieldHeaders:  m.headers(req.Header),
		ContextFieldParams:   params,
	}
	if id, ok := rest.RequestIdOf(req.Context()); ok {
		request[ContextFieldRequestId] = id
	}
	if m.maxBody > 0 && !strings.EqualFold(req.Header.Get("Expect"), middlewareContinue) {
		body, truncated := m.read(req)
		m.body(request, body, truncated, middlewareJSON(req.Header, middlewareRequestJSONTypes))
	}
	return request
}

// read returns up to the maximum body size bytes of the body of the
// request, and whether it is longer, leaving the body whole for the
// handler: the bytes read are read again before the rest of it, which is
// closed by closing the body. A request without body has an empty one.
func (m *middleware) read(
	req *http.Request,
) ([]byte, bool) {
	if req.Body == nil || req.Body == http.NoBody {
		return nil, false
	}
	// a failing body fails the handler too, as it reads it again: the
	// bytes read before the failure are logged
	read, _ := io.ReadAll(io.LimitReader(req.Body, int64(m.maxBody)+1))
	req.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(read), req.Body), req.Body}
	if len(read) > m.maxBody {
		return read[:m.maxBody], true
	}
	return read, false
}

// body stores the body in the bag, with the truncation flag when it is
// truncated, or else its JSON decoding when JSON bodies are decoded, the
// content type is JSON and it decodes.
func (m *middleware) body(
	bag container.Bag,
	body []byte,
	truncated bool,
	isJSON bool,
) {
	bag[ContextFieldBody] = string(body)
	if truncated {
		bag[ContextFieldBodyTruncated] = true
		return
	}
	if m.parseJSON && isJSON {
		var decoded any
		if json.Unmarshal(body, &decoded) == nil {
			bag[ContextFieldBodyJSON] = decoded
		}
	}
}

// headers returns the bag of the headers, each name holding its value, or
// a copy of its values when it has several, or [RedactedValue] when
// redacted.
func (m *middleware) headers(
	header http.Header,
) container.Bag {
	headers := container.Bag{}
	for name, values := range header {
		if _, ok := m.redactHeaders[strings.ToLower(name)]; ok {
			headers[name] = RedactedValue
			continue
		}
		headers[name] = middlewareValues(values)
	}
	return headers
}

// signal signals the response to the request, at the error level for a
// response of a 5xx status or a handler that panicked, and at the
// response level otherwise, on the channel, with the body kept by the tee
// writer, when not nil, when the status is not the expected one, as the
// tee writer then kept it.
func (m *middleware) signal(
	request container.Bag,
	w flam.IRestResponseWriter,
	tee *writer,
	expected int,
	duration time.Duration,
	panicked bool,
) {
	status := w.Status()
	if panicked && !w.Written() {
		status = 0
	}
	response := container.Bag{
		ContextFieldStatus:  status,
		ContextFieldSize:    w.Size(),
		ContextFieldHeaders: m.headers(w.Header()),
	}
	if tee != nil && status != expected {
		m.body(response, tee.body.Bytes(), tee.truncated, middlewareJSON(w.Header(), middlewareResponseJSONTypes))
	}
	ctx := container.Bag{
		ContextFieldRequest:  request,
		ContextFieldResponse: response,
		ContextFieldDuration: duration,
	}
	level := m.levelResponse
	if panicked {
		ctx[ContextFieldPanicked] = true
	}
	if panicked || status >= http.StatusInternalServerError {
		level = m.levelError
	}
	m.logger.Signal(level, m.channel, middlewareResponseMessage, ctx)
}

// middlewareValues returns the value of a header or a query parameter, or
// a copy of its values when it has several.
func middlewareValues(
	values []string,
) any {
	if len(values) == 1 {
		return values[0]
	}
	return slices.Clone(values)
}

// middlewareJSON reports whether the content type of the header starts
// with one of the media types, whatever its case.
func middlewareJSON(
	header http.Header,
	types []string,
) bool {
	contentType := strings.ToLower(header.Get("Content-Type"))
	return slices.ContainsFunc(types, func(t string) bool {
		return strings.HasPrefix(contentType, t)
	})
}

// Messages of the entries signaled by the middleware.
const (
	middlewareRequestMessage  = "rest request received"
	middlewareResponseMessage = "rest response sent"
)

// middlewareContinue is the value of the Expect header of a request whose
// body is sent once the handler starts reading it.
const middlewareContinue = "100-continue"

// middlewareRequestJSONTypes are the media types of the request bodies
// decoded as JSON.
var middlewareRequestJSONTypes = []string{"application/json"}

// middlewareResponseJSONTypes are the media types of the response bodies
// decoded as JSON.
var middlewareResponseJSONTypes = []string{"application/json", "application/problem+json"}
