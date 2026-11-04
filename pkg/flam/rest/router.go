package rest

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewRouter returns the router of a server of the service of the id,
// wrapping the handler of each endpoint in the middlewares, the first one
// being the outermost. It fails with ErrNilMiddleware when one of them is
// nil, and with ErrMiddleware when one returns a nil handler for the
// requests matching no pattern.
//
// The router routes the requests with an [http.ServeMux]. The requests
// matching no pattern are answered, through the middlewares, given an
// endpoint holding the id of the service only, by [WriteEnvelope] with an
// error of status 404 wrapping ErrNotFound, or 405 wrapping
// ErrMethodNotAllowed, along with the Allow header, while the redirections
// of the mux, such as from "/a" to "/a/", are left as they are.
func NewRouter(
	serviceId int,
	middlewares ...flam.IRestMiddleware,
) (*Router, error) {
	for _, middleware := range middlewares {
		if value.IsNil(middleware) {
			return nil, flam.NewWrappedError(ErrNilMiddleware, routerNewMessage)
		}
	}
	r := &Router{
		serviceId:   serviceId,
		middlewares: middlewares,
		mux:         http.NewServeMux(),
	}
	unmatched, err := r.wrap(routerNewMessage, flam.RestEndpoint{ServiceId: serviceId}, http.HandlerFunc(r.serveUnmatched))
	if err != nil {
		return nil, err
	}
	r.unmatched = unmatched
	return r, nil
}

// Router is the [flam.IRestRouter] returned by [NewRouter], serving the
// requests of the endpoints it registered as an [http.Handler]. It is safe
// for concurrent use.
type Router struct {
	serviceId   int
	middlewares []flam.IRestMiddleware
	mux         *http.ServeMux
	unmatched   http.Handler
}

// compile time checks that *Router implements IRestRouter and Handler
var (
	_ flam.IRestRouter = (*Router)(nil)
	_ http.Handler     = (*Router)(nil)
)

// Handle registers the handler of the endpoint, its ServiceId set to the
// id of the service, wrapped in the middlewares, under its pattern. The
// request given to the middlewares and the handler holds the endpoint in
// its context, read by [EndpointOf].
//
// It fails with ErrInvalidEndpoint when the endpoint has no pattern, an
// invalid pattern, or a pattern conflicting with the pattern of another
// endpoint, a status out of the 200 to 599 range, other than 0, or a nil
// handler, and with ErrMiddleware when a middleware returns a nil
// handler, registering nothing.
func (r *Router) Handle(
	endpoint flam.RestEndpoint,
	handler http.Handler,
) error {
	endpoint.ServiceId = r.serviceId
	switch {
	case endpoint.Pattern == "":
		return newErrEndpoint(ErrInvalidEndpoint, routerHandleMessage, endpoint, nil).
			With(ErrorFieldField, "pattern")
	case endpoint.Status != 0 && (endpoint.Status < 200 || endpoint.Status > 599):
		return newErrEndpoint(ErrInvalidEndpoint, routerHandleMessage, endpoint, nil).
			With(ErrorFieldField, "status")
	case value.IsNil(handler):
		return newErrEndpoint(ErrInvalidEndpoint, routerHandleMessage, endpoint, nil).
			With(ErrorFieldField, "handler")
	}

	handler, err := r.wrap(routerHandleMessage, endpoint, handler)
	if err != nil {
		return err
	}
	return r.handle(endpoint, handler)
}

// ServeHTTP serves the request with the handler of the endpoint whose
// pattern matches it, or answers it with an envelope error when none does.
func (r *Router) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	if _, pattern := r.mux.Handler(req); pattern != "" {
		r.mux.ServeHTTP(w, req)
		return
	}
	r.unmatched.ServeHTTP(w, req)
}

// Messages of the errors returned by NewRouter and the router methods.
const (
	routerNewMessage    = "unable to create rest router"
	routerHandleMessage = "unable to handle rest endpoint"
)

// endpointKey is the key of the endpoint in the context of the requests.
type endpointKey struct{}

// wrap returns the handler wrapped in the middlewares, then in the handler
// giving the endpoint to the requests, failing with ErrMiddleware, with
// the message, when a middleware returns a nil handler.
func (r *Router) wrap(
	message string,
	endpoint flam.RestEndpoint,
	handler http.Handler,
) (http.Handler, error) {
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i].Wrap(endpoint, handler)
		if value.IsNil(handler) {
			return nil, newErrEndpoint(ErrMiddleware, message, endpoint, nil).
				With(ErrorFieldID, r.middlewares[i].Id())
		}
	}
	inner := handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := context.WithValue(req.Context(), endpointKey{}, endpoint)
		inner.ServeHTTP(w, req.WithContext(ctx))
	}), nil
}

// serveUnmatched answers the request matching no pattern with an envelope
// error of the status the mux answers it with, along with its Allow
// header.
func (r *Router) serveUnmatched(
	w http.ResponseWriter,
	req *http.Request,
) {
	handler, _ := r.mux.Handler(req)
	unmatched := &statusRecorder{header: http.Header{}}
	handler.ServeHTTP(unmatched, req)
	err := ErrNotFound
	if unmatched.status == http.StatusMethodNotAllowed {
		err = ErrMethodNotAllowed
		w.Header()["Allow"] = unmatched.header["Allow"]
	}
	_ = WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(unmatched.status, err))
}

// statusRecorder is the response writer recording the status and the
// headers of the response of the mux to a request matching no pattern,
// dropping its body.
type statusRecorder struct {
	header http.Header
	status int
}

func (r *statusRecorder) Header() http.Header {
	return r.header
}

func (r *statusRecorder) Write(
	data []byte,
) (int, error) {
	return len(data), nil
}

func (r *statusRecorder) WriteHeader(
	status int,
) {
	r.status = status
}

// handle registers the handler under the pattern of the endpoint, turning
// the panic of the mux, for an invalid or conflicting pattern, into an
// error.
func (r *Router) handle(
	endpoint flam.RestEndpoint,
	handler http.Handler,
) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = newErrEndpoint(ErrInvalidEndpoint, routerHandleMessage, endpoint, fmt.Errorf("%v", recovered)).
				With(ErrorFieldField, "pattern")
		}
	}()
	r.mux.Handle(endpoint.Pattern, handler)
	return nil
}
