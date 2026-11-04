package etag

import (
	"net/http"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the etag [flam.IRestMiddleware], tagging the 200
// responses to the GET and HEAD requests whose body fits in its maximum
// size, and answering the conditional ones whose tag matches with a 304.
type middleware struct {
	id      string
	maxSize int
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the GET and HEAD requests through next,
// given a writer buffering their 200 responses to tag them, and the other
// requests through next as they are. It returns next itself for a
// streaming endpoint, whose response is not buffered.
func (m *middleware) Wrap(
	endpoint flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	if endpoint.Streaming {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			next.ServeHTTP(w, req)
			return
		}
		ew := &writer{
			ResponseWriter: w,
			maxSize:        m.maxSize,
			head:           req.Method == http.MethodHead,
			ifNoneMatch:    strings.Join(req.Header.Values("If-None-Match"), ","),
		}
		next.ServeHTTP(rest.NewResponseWriter(ew), req)
		ew.close()
	})
}

// middlewareMatch reports whether the value of the If-None-Match header,
// a list of entity tags or "*", matches the entity tag, by the weak
// comparison of RFC 9110, telling a weak tag from a strong one by their
// opaque tag only. A malformed list matches from its start up to the
// first malformed entry.
func middlewareMatch(
	ifNoneMatch string,
	etag string,
) bool {
	opaque := strings.TrimPrefix(etag, "W/")
	for {
		ifNoneMatch = strings.TrimLeft(ifNoneMatch, " \t,")
		if ifNoneMatch == "" {
			return false
		}
		if ifNoneMatch[0] == '*' {
			return true
		}
		tag, rest, ok := middlewareScan(ifNoneMatch)
		if !ok {
			return false
		}
		if strings.TrimPrefix(tag, "W/") == opaque {
			return true
		}
		ifNoneMatch = rest
	}
}

// middlewareScan returns the entity tag at the start of the text, a quoted
// opaque tag, prefixed by "W/" when weak, the text after it, and whether
// the text starts with one.
func middlewareScan(
	text string,
) (string, string, bool) {
	start := 0
	if strings.HasPrefix(text, "W/") {
		start = 2
	}
	if len(text) <= start || text[start] != '"' {
		return "", "", false
	}
	end := strings.IndexByte(text[start+1:], '"')
	if end < 0 {
		return "", "", false
	}
	end += start + 2
	return text[:end], text[end:], true
}
