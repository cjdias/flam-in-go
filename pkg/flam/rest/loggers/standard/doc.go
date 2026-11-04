// Package standard is the REST logger adaptor driver writing the lifecycle
// of the servers to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container: each
// report is an entry signaled on the channel of the adaptor, at the level
// of its kind, a start, an error, a shutdown or a done, with the server id
// in its context, at [ContextFieldServer], along with, for a start, the
// address it listens on, at [ContextFieldAddress], and, for an error, the
// error, at [ContextFieldError].
package standard
