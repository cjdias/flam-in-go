package ratelimit

import (
	"container/list"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// middleware is the ratelimit [flam.IRestMiddleware], holding a bucket
// per client, from the most recently seen to the least, up to its
// maximum number of buckets.
type middleware struct {
	id      string
	clock   flam.IClock
	limit   rate.Limit
	burst   int
	refill  time.Duration
	maxKeys int
	header  string
	prefix  int

	mu      sync.Mutex
	buckets map[string]*list.Element // of *bucket, by key
	recent  *list.List               // of *bucket, the most recent first
}

// bucket is the token bucket of the client of the key, along with the
// time its client was last seen.
type bucket struct {
	key     string
	limiter *rate.Limiter
	seen    time.Time
}

// compile time check that *middleware implements IRestMiddleware
var _ flam.IRestMiddleware = (*middleware)(nil)

func (m *middleware) Id() string {
	return m.id
}

// Wrap returns the handler serving the requests through next when the
// bucket of their client holds a token, which they take, or answering
// them with a 429 envelope of ErrRateLimited, along with the Retry-After
// header, of 1 second to a year, otherwise.
func (m *middleware) Wrap(
	_ flam.RestEndpoint,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		allowed, wait := m.allow(m.key(req), m.clock.Now())
		if allowed {
			next.ServeHTTP(w, req)
			return
		}
		seconds := min(max(math.Ceil(wait), 1), middlewareMaxRetryAfter)
		w.Header().Set("Retry-After", strconv.FormatInt(int64(seconds), 10))
		_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().WithNewError(http.StatusTooManyRequests, ErrRateLimited))
	})
}

// key returns the key of the client of the request: its principal when
// it holds one, or else its IP.
func (m *middleware) key(
	req *http.Request,
) string {
	if principal, ok := rest.PrincipalOf(req.Context()); ok {
		return middlewarePrincipalKey + principal
	}
	return middlewareIPKey + m.ip(req)
}

// ip returns the IP of the client of the request: the last one of the
// trusted proxy header, when set and valid, or else the one of its remote
// address, an IPv6 one reduced to its prefix, or the remote address as it
// is when it holds no IP.
func (m *middleware) ip(
	req *http.Request,
) string {
	if m.header != "" {
		if values := req.Header.Values(m.header); len(values) > 0 {
			last := values[len(values)-1]
			if addr, ok := middlewareAddr(last[strings.LastIndex(last, ",")+1:]); ok {
				return m.masked(addr)
			}
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		host = req.RemoteAddr
	}
	if addr, ok := middlewareAddr(host); ok {
		return m.masked(addr)
	}
	return req.RemoteAddr
}

// masked returns the IP of the address, unmapped from IPv6 when an IPv4
// one, and reduced to its prefix when an IPv6 one.
func (m *middleware) masked(
	addr netip.Addr,
) string {
	addr = addr.Unmap().WithZone("")
	if addr.Is6() {
		// the length is from 0 to 128, so that it never fails
		prefix, _ := addr.Prefix(m.prefix)
		return prefix.String()
	}
	return addr.String()
}

// allow takes a token from the bucket of the key at the time, made full
// when missing, reporting whether there was one, or else the seconds until
// there is, which may be too many for a time.Duration. It drops beforehand the buckets full again, and the least
// recently seen one when the maximum number of buckets is reached.
func (m *middleware) allow(
	key string,
	now time.Time,
) (bool, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for oldest := m.recent.Back(); oldest != nil; oldest = m.recent.Back() {
		if now.Sub(oldest.Value.(*bucket).seen) < m.refill {
			break
		}
		m.drop(oldest)
	}

	element, ok := m.buckets[key]
	if ok {
		m.recent.MoveToFront(element)
	} else {
		if m.recent.Len() >= m.maxKeys {
			m.drop(m.recent.Back())
		}
		element = m.recent.PushFront(&bucket{key: key, limiter: rate.NewLimiter(m.limit, m.burst)})
		m.buckets[key] = element
	}
	b := element.Value.(*bucket)
	b.seen = now

	if b.limiter.AllowN(now, 1) {
		return true, 0
	}
	return false, (1 - b.limiter.TokensAt(now)) / float64(m.limit)
}

// drop drops the bucket of the element.
func (m *middleware) drop(
	element *list.Element,
) {
	m.recent.Remove(element)
	delete(m.buckets, element.Value.(*bucket).key)
}

// middlewareAddr returns the IP of the text, an IP or an IP and a port,
// spaces trimmed, and whether it is one.
func middlewareAddr(
	text string,
) (netip.Addr, bool) {
	text = strings.TrimSpace(text)
	if addr, err := netip.ParseAddr(text); err == nil {
		return addr, true
	}
	addrPort, err := netip.ParseAddrPort(text)
	return addrPort.Addr(), err == nil
}

// middlewareMaxRetryAfter is the maximum of the seconds of the
// Retry-After header, a year.
const middlewareMaxRetryAfter = 365 * 24 * 60 * 60

// Prefixes of the keys of the buckets, telling a principal from an IP.
const (
	middlewarePrincipalKey = "principal:"
	middlewareIPKey        = "ip:"
)
