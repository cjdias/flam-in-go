package flam

import (
	"context"
	"io"
	"net/http"
)

// ClientCircuitState is the state of an [IClientCircuitBreaker].
type ClientCircuitState int

const (
	// ClientCircuitClosed lets every request through, counting the
	// failures, until enough of them open the circuit.
	ClientCircuitClosed ClientCircuitState = iota

	// ClientCircuitOpen refuses every request, until its open time
	// elapses and the circuit becomes half open.
	ClientCircuitOpen

	// ClientCircuitHalfOpen lets a limited number of trial requests
	// through, to find out whether the server recovered: enough successful
	// ones close the circuit, while a failed one opens it again.
	ClientCircuitHalfOpen
)

// String returns the name of the state, "closed", "open" or "half-open",
// or "unknown" for any other value.
func (s ClientCircuitState) String() string {
	switch s {
	case ClientCircuitClosed:
		return "closed"
	case ClientCircuitOpen:
		return "open"
	case ClientCircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ClientOutcome is how a request let through by an
// [IClientCircuitBreaker] ended, as reported to its Done method.
type ClientOutcome int

const (
	// ClientOutcomeSuccess is a request the server answered, telling that
	// it is healthy.
	ClientOutcomeSuccess ClientOutcome = iota

	// ClientOutcomeFailure is a request the server failed, such as with a
	// 5xx status, or that could not reach it, telling that it is not
	// healthy.
	ClientOutcomeFailure

	// ClientOutcomeIgnored is a request telling nothing of the health of
	// the server, such as one cancelled by its caller: it frees the trial
	// slot it took in a half open circuit, without counting.
	ClientOutcomeIgnored
)

// IClientCircuitBreaker protects a server from the requests of an
// [IClient] while it fails: it lets the requests through while closed,
// refuses them once open, and lets a few trial requests through when half
// open, as described by [ClientCircuitState]. It is held by an
// [IClientCircuitBreakerFactory], and may be shared by several clients
// reaching the same server.
//
// Every request asks Allow first, and, when let through, reports how it
// ended to Done, with the generation Allow returned. As every change of
// state starts a new generation, the outcome of a request let through
// before a change does not count against the state following it.
//
// An IClientCircuitBreaker is safe for concurrent use.
type IClientCircuitBreaker interface {
	IResource

	// State returns the current state of the circuit.
	State() ClientCircuitState

	// Allow reports whether a request may go through, and the generation
	// it goes through under, to be given back to Done.
	Allow() (generation uint64, ok bool)

	// Done records the outcome of a request let through by Allow under the
	// generation, which is dropped once the generation is over.
	Done(generation uint64, outcome ClientOutcome)
}

// IClientCircuitBreakerCreator creates the circuit breakers of a driver,
// for the [IClientCircuitBreakerFactory].
type IClientCircuitBreakerCreator = IResourceCreator[IClientCircuitBreaker]

// IClientCircuitBreakerFactory creates and holds the circuit breakers, one
// per id.
type IClientCircuitBreakerFactory = IFactory[IClientCircuitBreaker]

// IClient is an outgoing HTTP client, such as for the API of another
// service, configured with its base URL, its timeout, its default headers,
// its retries and its circuit breaker. It is held by an
// [IClientFactory], which closes it.
//
// An IClient is safe for concurrent use.
type IClient interface {
	io.Closer
	IResource

	// Do sends the request and returns the response, as [http.Client.Do]
	// does: the response may have any status, and its body must be closed
	// by the caller. The request is not changed: it is sent with its URL
	// resolved against the base URL of the client, when relative, and with
	// the default headers it does not set.
	Do(req *http.Request) (*http.Response, error)

	// Request sends the request of the method, for the URL, resolved
	// against the base URL of the client when relative, with the body,
	// which may be nil, under the context, as Do does.
	Request(ctx context.Context, method string, url string, body io.Reader) (*http.Response, error)
}

// IClientCreator creates the clients of a driver, for the
// [IClientFactory].
type IClientCreator = IResourceCreator[IClient]

// IClientFactory creates and holds the clients, one per id.
type IClientFactory = IFactory[IClient]
