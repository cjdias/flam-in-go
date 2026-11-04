package middleware

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the metrics [flam.IRestMiddleware], measuring the
// requests it serves on its metrics.
type middleware struct {
	id        string
	clock     flam.IClock
	codeClass bool
	requests  *prometheus.CounterVec
	duration  *prometheus.HistogramVec
	inFlight  *prometheus.GaugeVec
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next, counting
// each as in flight while served, then counting it and observing the time
// taken to serve it, even when next panics, whose panic then goes on. The
// status is read from a [flam.IRestResponseWriter] wrapping the one of the
// request. The in flight gauge of the endpoint is created at once, so
// that it is exposed, at 0, before its first request.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	service := strconv.Itoa(endpoint.ServiceId)
	id := strconv.Itoa(endpoint.Id)
	pattern := endpoint.Pattern
	inFlight := m.inFlight.WithLabelValues(service, id, pattern)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		rw := rest.NewResponseWriter(w)
		inFlight.Inc()
		start := m.clock.Now()
		panicked := true
		defer func() {
			duration := m.clock.Since(start)
			inFlight.Dec()
			method := middlewareMethod(req.Method)
			code := m.code(rw, panicked)
			m.requests.WithLabelValues(service, id, pattern, method, code).Inc()
			m.duration.WithLabelValues(service, id, pattern, method, code).Observe(duration.Seconds())
		}()
		next.ServeHTTP(rw, req)
		panicked = false
	})
}

// code returns the value of the code label of the response: CodePanic
// when the handler panicked before writing it, or else its status, or the
// class of its status when the middleware labels the classes.
func (m *middleware) code(
	w flam.IRestResponseWriter,
	panicked bool,
) string {
	if panicked && !w.Written() {
		return CodePanic
	}
	status := w.Status()
	if m.codeClass {
		return strconv.Itoa(status/100) + "xx"
	}
	return strconv.Itoa(status)
}

// middlewareMethod returns the value of the method label of the request
// method: the method when standard, or else MethodOther.
func middlewareMethod(
	method string,
) string {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch,
		http.MethodDelete, http.MethodConnect, http.MethodOptions, http.MethodTrace:
		return method
	}
	return MethodOther
}
