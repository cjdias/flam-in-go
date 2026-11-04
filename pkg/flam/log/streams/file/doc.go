// Package file is the log stream driver of the files of a disk: its
// streams append each entry they keep, serialized, on a line of their
// file, created when missing, but not its directory.
//
// A stream keeps the entries of its level or more severe, an entry of
// level [github.com/cjdias/flam-in-go/pkg/flam.LogNone] never being kept:
// the broadcast ones, and the signaled ones when it listens to their
// channel, or to "*". The channel of a signaled entry is given to the
// serializer in the context, at
// [github.com/cjdias/flam-in-go/pkg/flam/log.ContextFieldChannel].
//
// Closing a stream closes its file.
package file
