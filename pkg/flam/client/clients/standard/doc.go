// Package standard is the client driver sending the requests with a
// [net/http] client of its own: each client resolves the relative URLs of
// its requests against its base URL, adds its default headers to them,
// and sends them within its timeout, under the protection of its circuit
// breaker, taken from the
// [github.com/cjdias/flam-in-go/pkg/flam.IClientCircuitBreakerFactory],
// which fails them fast with [ErrOpen] while its circuit is open.
//
// The requests the server fails, answering with a 5xx status, and those
// failing to reach it count as failures for the circuit breaker, while the
// requests cancelled by their caller do not count. Those of an idempotent
// method, GET, HEAD, OPTIONS, TRACE, PUT or DELETE, whose body can be sent
// again, are sent again on such a failure, up to the number of retries of
// the client, after a delay doubled at each retry, up to its maximum,
// waited with the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
// container.
package standard
