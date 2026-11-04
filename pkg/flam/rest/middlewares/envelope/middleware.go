package envelope

import (
	"errors"
	"net/http"
	"runtime/debug"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	flamenvelope "github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the envelope [flam.IRestMiddleware], reporting the
// unexpected panics it answers on its channel of the logger, when not nil,
// and sending the message of their error values when exposing the errors.
type middleware struct {
	id           string
	logger       flam.ILogger
	channel      string
	level        flam.LogLevel
	exposeErrors bool
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next, answering
// its panics with the envelope they stand for, or panicking again with an
// [http.ErrAbortHandler] panic, or once the response was written.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := rest.NewResponseWriter(w)
		defer func() {
			if recovered := recover(); recovered != nil {
				m.handlePanic(endpoint, rw, req, recovered)
			}
		}()
		next.ServeHTTP(rw, req)
	})
}

// handlePanic handles the value a handler of the endpoint panicked with:
// panicking again with an [http.ErrAbortHandler] panic, or, after
// reporting it, when the response was written, or answering with the
// envelope it stands for otherwise.
func (m *middleware) handlePanic(
	endpoint flam.RestEndpoint,
	w flam.IRestResponseWriter,
	req *http.Request,
	recovered any,
) {
	if e, ok := recovered.(error); ok && errors.Is(e, http.ErrAbortHandler) {
		panic(recovered)
	}
	if w.Written() {
		m.signal(endpoint, recovered)
		panic(recovered)
	}
	_ = rest.WriteEnvelope(w, req, m.envelope(endpoint, recovered))
}

// envelope returns the envelope answering the value a handler of the
// endpoint panicked with: the value itself when an envelope, an envelope
// of the value when an envelope error, or else, after reporting the
// panic, a 500 envelope of the value when an error and the errors are
// exposed, or of [ErrInternal] otherwise.
func (m *middleware) envelope(
	endpoint flam.RestEndpoint,
	recovered any,
) flamenvelope.Envelope {
	switch value := recovered.(type) {
	case flamenvelope.Envelope:
		return value
	case flamenvelope.Error:
		return flamenvelope.NewEnvelope().WithError(value)
	case error:
		m.signal(endpoint, recovered)
		if m.exposeErrors {
			return flamenvelope.NewEnvelope().WithNewError(http.StatusInternalServerError, value)
		}
	default:
		m.signal(endpoint, recovered)
	}
	return flamenvelope.NewEnvelope().WithNewError(http.StatusInternalServerError, ErrInternal)
}

// signal signals the panic of a handler of the endpoint at the level on
// the channel, with the endpoint id and pattern, the value of the panic
// and the stack in the context, when the logger is not nil.
func (m *middleware) signal(
	endpoint flam.RestEndpoint,
	recovered any,
) {
	if m.logger == nil {
		return
	}
	m.logger.Signal(m.level, m.channel, "rest handler panicked", container.Bag{
		ContextFieldEndpoint: endpoint.Id,
		ContextFieldPattern:  endpoint.Pattern,
		ContextFieldPanic:    recovered,
		ContextFieldStack:    string(debug.Stack()),
	})
}
