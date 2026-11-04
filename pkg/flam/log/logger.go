package log

import (
	"errors"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// newLogger returns the logger stamping its entries with the clock and
// writing them, when flushed, to the streams stored in the stream factory.
// It is a provider registered by the bundle returned from [NewBundle].
func newLogger(
	clock flam.IClock,
	streams flam.ILogStreamFactory,
) flam.ILogger {
	return &logger{
		clock:   clock,
		streams: streams,
	}
}

// logger is the [flam.ILogger] provided by the bundle: it keeps its entries
// until flushed, then writes each of them to every stream stored in the
// stream factory, in stream id order.
type logger struct {
	clock   flam.IClock
	streams flam.ILogStreamFactory

	mu      sync.Mutex // guards entries
	entries []logEntry

	flushMu sync.Mutex // serializes the flushes, keeping the entries in order
}

// logEntry is an entry kept by the logger until flushed. A broadcast entry
// has no channel.
type logEntry struct {
	timestamp time.Time
	level     flam.LogLevel
	broadcast bool
	channel   string
	message   string
	ctx       []flam.IBag
}

// compile time check that *logger implements ILogger
var _ flam.ILogger = (*logger)(nil)

func (l *logger) Signal(
	level flam.LogLevel,
	channel, message string,
	ctx ...flam.IBag,
) {
	l.add(logEntry{level: level, channel: channel, message: message, ctx: ctx})
}

func (l *logger) SignalFatal(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogFatal, channel, message, ctx...)
}

func (l *logger) SignalError(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogError, channel, message, ctx...)
}

func (l *logger) SignalWarning(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogWarning, channel, message, ctx...)
}

func (l *logger) SignalNotice(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogNotice, channel, message, ctx...)
}

func (l *logger) SignalInfo(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogInfo, channel, message, ctx...)
}

func (l *logger) SignalDebug(channel, message string, ctx ...flam.IBag) {
	l.Signal(flam.LogDebug, channel, message, ctx...)
}

func (l *logger) Broadcast(
	level flam.LogLevel,
	message string,
	ctx ...flam.IBag,
) {
	l.add(logEntry{level: level, broadcast: true, message: message, ctx: ctx})
}

func (l *logger) BroadcastFatal(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogFatal, message, ctx...)
}

func (l *logger) BroadcastError(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogError, message, ctx...)
}

func (l *logger) BroadcastWarning(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogWarning, message, ctx...)
}

func (l *logger) BroadcastNotice(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogNotice, message, ctx...)
}

func (l *logger) BroadcastInfo(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogInfo, message, ctx...)
}

func (l *logger) BroadcastDebug(message string, ctx ...flam.IBag) {
	l.Broadcast(flam.LogDebug, message, ctx...)
}

func (l *logger) Flush() error {
	l.flushMu.Lock()
	defer l.flushMu.Unlock()

	l.mu.Lock()
	entries := l.entries
	l.entries = nil
	l.mu.Unlock()
	if len(entries) == 0 {
		return nil
	}

	// the streams are resolved first, without creating those removed since
	// listed
	var streams []flam.ILogStream
	for _, id := range l.streams.Stored() {
		if stream, ok := l.streams.Lookup(id); ok {
			streams = append(streams, stream)
		}
	}

	// a stream failing to write an entry is not given the next ones, so
	// that it reports a single error
	var errs []error
	for _, stream := range streams {
		for _, entry := range entries {
			if err := entry.write(stream); err != nil {
				errs = append(errs, newErrStream(loggerFlushMessage, stream.Id(), err))
				break
			}
		}
	}
	return errors.Join(errs...)
}

// loggerFlushMessage is the message of the errors returned by Flush.
const loggerFlushMessage = "unable to flush log entries to stream"

// add stamps the entry with the time of the clock and keeps it until the
// next flush, along with a copy of its non-nil contexts merged, the later
// ones winning, once for every stream, so that changing them afterwards
// does not change the entry. It stamps the entry while holding the lock,
// so that the entries are kept, and so flushed, in time order.
func (l *logger) add(
	entry logEntry,
) {
	merged := container.Bag{}
	for _, c := range entry.ctx {
		if !value.IsNil(c) {
			for key, val := range c.All() {
				merged[key] = val
			}
		}
	}
	entry.ctx = nil
	if len(merged) > 0 {
		entry.ctx = []flam.IBag{merged.Clone()}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	entry.timestamp = l.clock.Now()
	l.entries = append(l.entries, entry)
}

// write writes the entry to the stream, as a broadcast or on its channel.
func (e logEntry) write(
	stream flam.ILogStream,
) error {
	if e.broadcast {
		return stream.Broadcast(e.timestamp, e.level, e.message, e.ctx...)
	}
	return stream.Signal(e.timestamp, e.level, e.channel, e.message, e.ctx...)
}
