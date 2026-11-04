// Package syslog is the log stream driver of the syslog daemons, through
// the log/syslog package: its streams send each entry they keep,
// serialized, to the local syslog daemon, or to the daemon of a network
// address, with the priority of its level in the facility of the stream,
// and the tag of the stream.
//
// The log/syslog package, and so this driver, is not provided on windows
// and plan9, where this package declares nothing: the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/log/streams] registers it only
// on the other platforms.
//
// The levels map to the syslog severities as follows: fatal to crit,
// error to err, warning to warning, notice to notice, info to info and
// debug to debug.
//
// A stream keeps the entries of its level or more severe, an entry of
// level [github.com/cjdias/flam-in-go/pkg/flam.LogNone] never being kept:
// the broadcast ones, and the signaled ones when it listens to their
// channel, or to "*". The channel of a signaled entry is given to the
// serializer in the context, at
// [github.com/cjdias/flam-in-go/pkg/flam/log.ContextFieldChannel].
//
// A stream connects to its daemon when created, reconnecting once when a
// write fails, and closing a stream closes its connection.
package syslog
