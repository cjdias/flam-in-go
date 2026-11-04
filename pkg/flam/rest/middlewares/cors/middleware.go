package cors

import (
	"net/http"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// middleware is the CORS [flam.IRestMiddleware], allowing the requests of
// its origins, answering their preflight requests for its methods and
// headers, and exposing its headers.
type middleware struct {
	id               string
	origins          map[string]struct{}
	anyOrigin        bool
	methods          map[string]struct{}
	allowMethods     string
	headers          map[string]struct{}
	anyHeader        bool
	exposeHeaders    string
	allowCredentials bool
	maxAge           string
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler answering the preflight requests itself, and
// serving the other requests through next, with the CORS headers allowing
// their origin, when allowed.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodOptions && req.Header.Get(middlewareRequestMethod) != "" {
			m.preflight(w, req)
			return
		}
		if !m.anyOrigin {
			w.Header().Add("Vary", "Origin")
		}
		if origin := req.Header.Get("Origin"); origin != "" && m.allowed(origin) {
			m.allow(w.Header(), origin)
			if m.exposeHeaders != "" {
				w.Header().Set("Access-Control-Expose-Headers", m.exposeHeaders)
			}
		}
		next.ServeHTTP(w, req)
	})
}

// preflight answers the preflight request with a 204 response, holding
// the headers allowing the request when its origin, its requested method
// and its requested headers are allowed, and none of them otherwise.
func (m *middleware) preflight(
	w http.ResponseWriter,
	req *http.Request,
) {
	header := w.Header()
	header.Add("Vary", "Origin")
	header.Add("Vary", middlewareRequestMethod)
	header.Add("Vary", middlewareRequestHeaders)

	origin := req.Header.Get("Origin")
	requested := middlewareHeaders(req.Header.Values(middlewareRequestHeaders))
	if origin != "" && m.allowed(origin) && m.allowedMethod(req.Header.Get(middlewareRequestMethod)) && m.allowedHeaders(requested) {
		m.allow(header, origin)
		header.Set("Access-Control-Allow-Methods", m.allowMethods)
		if len(requested) > 0 {
			header.Set("Access-Control-Allow-Headers", strings.Join(requested, ", "))
		}
		if m.maxAge != "" {
			header.Set("Access-Control-Max-Age", m.maxAge)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// allow sets the headers allowing the origin, and the credentials when
// allowed, on the header of the response, the "*" wildcard standing for
// the origin when any is allowed, as the credentials then never are.
func (m *middleware) allow(
	header http.Header,
	origin string,
) {
	if m.anyOrigin {
		header.Set("Access-Control-Allow-Origin", Wildcard)
		return
	}
	header.Set("Access-Control-Allow-Origin", origin)
	if m.allowCredentials {
		header.Set("Access-Control-Allow-Credentials", "true")
	}
}

// allowed reports whether the origin is allowed, any origin being allowed
// with the "*" wildcard, and the others whatever their case.
func (m *middleware) allowed(
	origin string,
) bool {
	if m.anyOrigin {
		return true
	}
	_, ok := m.origins[strings.ToLower(origin)]
	return ok
}

// allowedMethod reports whether the method requested by a preflight
// request is allowed, matched exactly, as the methods are case-sensitive.
func (m *middleware) allowedMethod(
	method string,
) bool {
	_, ok := m.methods[method]
	return ok
}

// allowedHeaders reports whether every header requested by a preflight
// request is allowed, any header being allowed with the "*" wildcard, and
// the others whatever their case.
func (m *middleware) allowedHeaders(
	requested []string,
) bool {
	if m.anyHeader {
		return true
	}
	for _, name := range requested {
		if _, ok := m.headers[strings.ToLower(name)]; !ok {
			return false
		}
	}
	return true
}

// middlewareHeaders returns the names of the headers listed by the values
// of the Access-Control-Request-Headers header, each a list of names
// separated by commas, trimmed, the empty ones left out.
func middlewareHeaders(
	values []string,
) []string {
	var names []string
	for _, value := range values {
		for name := range strings.SplitSeq(value, ",") {
			if name = strings.TrimSpace(name); name != "" {
				names = append(names, name)
			}
		}
	}
	return names
}

// Headers of the preflight requests.
const (
	middlewareRequestMethod  = "Access-Control-Request-Method"
	middlewareRequestHeaders = "Access-Control-Request-Headers"
)
