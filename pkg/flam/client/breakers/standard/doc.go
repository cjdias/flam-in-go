// Package standard is the circuit breaker driver counting the consecutive
// failures of the requests: the circuit opens once their number reaches
// the failure threshold of the breaker, refusing every request during its
// open timeout, measured with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, then
// becomes half open, letting its half open requests through as trials:
// the circuit closes once they all succeed, and opens again as soon as one
// fails. A success while closed resets the count of failures.
//
// The state is computed when the breaker is consulted, so that no timer
// runs: an open circuit becomes half open on the first call made once its
// open timeout elapsed.
package standard
