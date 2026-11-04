package compress

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the compress [flam.IRestMiddleware], compressing the
// responses of its content types, once their body reaches its minimum
// size, with the encoding the client prefers among its own, each with its
// pool of compressors.
type middleware struct {
	id        string
	minSize   int
	types     map[string]struct{}
	prefixes  []string
	anyType   bool
	encodings []string
	pools     map[string]*sync.Pool // of compressor, by encoding
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler adding Accept-Encoding to the Vary header of
// the responses, then serving the requests through next, given a writer
// compressing the response when the client accepts one of the encodings,
// unless the request is a HEAD one, whose response has no body. It
// returns next itself for a streaming endpoint, whose response is not
// buffered.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	if endpoint.Streaming {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		middlewareVary(w.Header())
		encoding := m.negotiate(req.Header.Values("Accept-Encoding"))
		if encoding == "" || req.Method == http.MethodHead {
			next.ServeHTTP(w, req)
			return
		}
		cw := &writer{ResponseWriter: w, m: m, encoding: encoding}
		next.ServeHTTP(rest.NewResponseWriter(cw), req)
		cw.close()
	})
}

// negotiate returns the encoding of the middleware of the highest quality
// in the values of the Accept-Encoding header, the first one listed by the
// middleware on a tie, or an empty string when the client accepts none,
// as when it sends no header. An encoding missing from the header takes
// the quality of "*", if listed, and "x-gzip" stands for "gzip".
func (m *middleware) negotiate(
	values []string,
) string {
	qualities := map[string]float64{}
	for _, value := range values {
		for part := range strings.SplitSeq(value, ",") {
			name, params, _ := strings.Cut(part, ";")
			name = strings.ToLower(strings.TrimSpace(name))
			if name == "" {
				continue
			}
			if name == "x-gzip" {
				name = EncodingGzip
			}
			qualities[name] = middlewareQuality(params)
		}
	}

	best, bestQuality := "", 0.0
	for _, encoding := range m.encodings {
		quality, ok := qualities[encoding]
		if !ok {
			quality = qualities["*"]
		}
		if quality > bestQuality {
			best, bestQuality = encoding, quality
		}
	}
	return best
}

// allowed reports whether the media type of the Content-Type header value
// is one of the content types of the middleware, whatever its case and
// parameters.
func (m *middleware) allowed(
	contentType string,
) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" {
		return false
	}
	if _, ok := m.types[mediaType]; ok || m.anyType {
		return true
	}
	for _, prefix := range m.prefixes {
		if strings.HasPrefix(mediaType, prefix) {
			return true
		}
	}
	return false
}

// middlewareQuality returns the quality of the parameters of an
// Accept-Encoding entry, 1 without q parameter, and 0 for an invalid one,
// out of the 0 to 1 range.
func middlewareQuality(
	params string,
) float64 {
	for param := range strings.SplitSeq(params, ";") {
		key, value, _ := strings.Cut(param, "=")
		if !strings.EqualFold(strings.TrimSpace(key), "q") {
			continue
		}
		quality, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || math.IsNaN(quality) || quality < 0 || quality > 1 {
			return 0
		}
		return quality
	}
	return 1
}

// middlewareVary adds Accept-Encoding to the Vary header, unless it lists
// it, whatever its case, or "*".
func middlewareVary(
	header http.Header,
) {
	for _, value := range header.Values("Vary") {
		for name := range strings.SplitSeq(value, ",") {
			if name = strings.TrimSpace(name); name == "*" || strings.EqualFold(name, "Accept-Encoding") {
				return
			}
		}
	}
	header.Add("Vary", "Accept-Encoding")
}
