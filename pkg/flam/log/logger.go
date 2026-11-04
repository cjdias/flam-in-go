package log

import (
	"context"
	"errors"
	"sync"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// loggerArgs are the dependencies of the logger, taken from the
// container. The group tag repeats ContextExtractorGroup, as a struct tag
// cannot use a constant.
type loggerArgs struct {
	dig.In

	Clock      flam.IClock
	Streams    flam.ILogStreamFactory
	Extractors []flam.ILogContextExtractor `group:"flam.log.context.extractors"`
}

// newLogger returns the logger stamping its entries with the clock,
// adding to the entries given with a context the fields the extractors
// extract from it, and writing them, when flushed, to the streams stored
// in the stream factory. It is a provider registered by the bundle
// returned from [NewBundle], as both the [flam.ILogger] and the
// [flam.IContextLogger] of the container.
func newLogger(
	args loggerArgs,
) *logger {
	return &logger{
		clock:      args.Clock,
		streams:    args.Streams,
		extractors: args.Extractors,
	}
}

// logger is the [flam.IContextLogger] provided by the bundle: it keeps its
// entries until flushed, then writes each of them to every stream stored
// in the stream factory, in stream id order.
type logger struct {
	clock      flam.IClock
	streams    flam.ILogStreamFactory
	extractors []flam.ILogContextExtractor

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

// compile time check that *logger implements IContextLogger
var _ flam.IContextLogger = (*logger)(nil)

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

func (l *logger) SignalContext(
	ctx context.Context,
	level flam.LogLevel,
	channel, message string,
	bags ...flam.IBag,
) {
	l.Signal(level, channel, message, l.extract(ctx, bags)...)
}

func (l *logger) SignalFatalContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogFatal, channel, message, bags...)
}

func (l *logger) SignalErrorContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogError, channel, message, bags...)
}

func (l *logger) SignalWarningContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogWarning, channel, message, bags...)
}

func (l *logger) SignalNoticeContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogNotice, channel, message, bags...)
}

func (l *logger) SignalInfoContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogInfo, channel, message, bags...)
}

func (l *logger) SignalDebugContext(ctx context.Context, channel, message string, bags ...flam.IBag) {
	l.SignalContext(ctx, flam.LogDebug, channel, message, bags...)
}

func (l *logger) BroadcastContext(
	ctx context.Context,
	level flam.LogLevel,
	message string,
	bags ...flam.IBag,
) {
	l.Broadcast(level, message, l.extract(ctx, bags)...)
}

func (l *logger) BroadcastFatalContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogFatal, message, bags...)
}

func (l *logger) BroadcastErrorContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogError, message, bags...)
}

func (l *logger) BroadcastWarningContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogWarning, message, bags...)
}

func (l *logger) BroadcastNoticeContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogNotice, message, bags...)
}

func (l *logger) BroadcastInfoContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogInfo, message, bags...)
}

func (l *logger) BroadcastDebugContext(ctx context.Context, message string, bags ...flam.IBag) {
	l.BroadcastContext(ctx, flam.LogDebug, message, bags...)
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

// extract returns the fields the extractors extract from the context,
// followed by the bags, so that, once merged by add, the bags win over
// the extracted fields. A nil context carries no field.
func (l *logger) extract(
	ctx context.Context,
	bags []flam.IBag,
) []flam.IBag {
	if ctx == nil || len(l.extractors) == 0 {
		return bags
	}
	all := make([]flam.IBag, 0, len(l.extractors)+len(bags))
	for _, extractor := range l.extractors {
		all = append(all, extractor.Extract(ctx))
	}
	return append(all, bags...)
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
