// Package standard is the watchdog logger adaptor driver writing the
// lifecycle of the processes to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container: each
// report is an entry signaled on the channel of the adaptor, at the level
// of its kind, a start, an error or a done, with the process id and, for
// an error, the error in its context, at [ContextFieldProcess] and
// [ContextFieldError], and, for a panic, its value and stack, at
// [ContextFieldPanic] and [ContextFieldStack].
package standard
