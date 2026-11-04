// Package console is the log stream driver of the standard output and
// error: its streams write each entry they keep, serialized, on a line of
// the output.
//
// A stream keeps the entries of its level or more severe, an entry of
// level [github.com/cjdias/flam-in-go/pkg/flam.LogNone] never being kept:
// the broadcast ones, and the signaled ones when it listens to their
// channel, or to "*". The channel of a signaled entry is given to the
// serializer in the context, at
// [github.com/cjdias/flam-in-go/pkg/flam/log.ContextFieldChannel].
//
// Closing a stream leaves the output open.
package console
