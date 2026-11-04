//go:build !windows && !plan9

package syslog

import (
	"log/syslog"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// writer is the base.Writer of a stream, sending its lines to the daemon
// with the priority of the level of their entry, which the leveled
// serializer of the stream records just before: the stream serializes an
// entry and writes its line holding its lock, so that the level recorded
// is the one of the line written.
type writer struct {
	conn  *syslog.Writer
	level flam.LogLevel // of the entry being written
}

// WriteLine sends the line with the severity of the level of its entry.
func (w *writer) WriteLine(
	_ time.Time,
	line string,
) error {
	switch w.level {
	case flam.LogFatal:
		return w.conn.Crit(line)
	case flam.LogError:
		return w.conn.Err(line)
	case flam.LogWarning:
		return w.conn.Warning(line)
	case flam.LogNotice:
		return w.conn.Notice(line)
	case flam.LogInfo:
		return w.conn.Info(line)
	default: // flam.LogDebug, as the stream keeps no other level
		return w.conn.Debug(line)
	}
}

// Close closes the connection to the daemon.
func (w *writer) Close() error {
	return w.conn.Close()
}

// leveled is the serializer of a stream, recording the level of each
// entry in the writer before serializing it with the serializer of the
// stream configuration.
type leveled struct {
	flam.ILogSerializer
	w *writer
}

func (l leveled) Serialize(
	timestamp time.Time,
	level flam.LogLevel,
	message string,
	ctx flam.IBag,
) string {
	l.w.level = level
	return l.ILogSerializer.Serialize(timestamp, level, message, ctx)
}
