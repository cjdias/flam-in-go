// Package rotatingfile is the log stream driver of the daily files of a
// disk: its streams append each entry they keep, serialized, on a line of
// the file of the day of the entry timestamp, in its location, created
// when missing, but not its directory. The path of the files holds
// [DatePlaceholder], replaced by the date, such as "logs/app-%s.log"
// giving "logs/app-2026-11-02.log", so that an entry logged before
// midnight but written after lands in the file of its day.
//
// The stream opens the file of the day of the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container when
// created, then the file of the day of an entry when it is not the file of
// the previous entry written, closing the previous one. The files of the
// previous days are kept, and appended to by an entry of their day.
//
// A stream keeps the entries of its level or more severe, an entry of
// level [github.com/cjdias/flam-in-go/pkg/flam.LogNone] never being kept:
// the broadcast ones, and the signaled ones when it listens to their
// channel, or to "*". The channel of a signaled entry is given to the
// serializer in the context, at
// [github.com/cjdias/flam-in-go/pkg/flam/log.ContextFieldChannel].
//
// Closing a stream closes its file.
package rotatingfile
