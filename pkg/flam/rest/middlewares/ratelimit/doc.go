// Package ratelimit is the REST middleware driver limiting the rate of the
// requests of each client with a token bucket of
// [golang.org/x/time/rate]: a bucket of [ConfigFieldBurst] tokens, refilled
// at [ConfigFieldRate] tokens per second, each request taking one. A
// request finding the bucket of its client empty is answered with a 429
// envelope of [ErrRateLimited], along with a Retry-After header holding
// the seconds until a token is back. The buckets live in the middleware,
// shared by all the endpoints of the servers it wraps, the requests
// matching none included.
//
// A client is told by its principal, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf], when an
// authentication middleware set one before this one, or else by the IP of
// the remote address of the request, an IPv6 address being reduced to its
// prefix of [ConfigFieldIPv6Prefix] bits, as a single client is commonly
// given a whole /64. Behind a proxy, set [ConfigFieldTrustedProxyHeader],
// such as to "X-Forwarded-For": the IP is then the last one of the header,
// the one added by the proxy, as the ones before it are sent by the client
// and may be forged, and the remote address is used when the header holds
// no valid IP. Leave it unset otherwise, the default, as any client can
// send the header to pose as many.
//
// The memory is bounded: a bucket is dropped once full again, its client
// idle for the time the bucket takes to refill, which loses nothing as a
// new bucket is full too, and the least recently seen bucket is dropped
// when [ConfigFieldMaxKeys] buckets are held, which a client may then
// see as a refill. The time is read from the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, no
// goroutine running in the background.
package ratelimit
