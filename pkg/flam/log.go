package flam

import (
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message.
type LogLevel int

// LogNone and the following constants enumerate the supported log severity levels.
const (
	LogNone LogLevel = iota
	LogFatal
	LogError
	LogWarning
	LogNotice
	LogInfo
	LogDebug
)

const (
	levelLogNone    = "none"
	levelLogFatal   = "fatal"
	levelLogError   = "error"
	levelLogWarning = "warning"
	levelLogNotice  = "notice"
	levelLogInfo    = "info"
	levelLogDebug   = "debug"
)

// String returns the level's name, such as "error" or "debug".
func (level LogLevel) String() string {
	switch level {
	case LogFatal:
		return levelLogFatal
	case LogError:
		return levelLogError
	case LogWarning:
		return levelLogWarning
	case LogNotice:
		return levelLogNotice
	case LogInfo:
		return levelLogInfo
	case LogDebug:
		return levelLogDebug
	default:
		return levelLogNone
	}
}

// From resolves a LogLevel from the given value: a number of any integer
// or float kind, a LogLevel included, holding a whole value between
// LogNone and LogDebug, or a string holding a level name, whatever its
// case, or such a number. It returns the first default for any other
// value, or LogNone without default.
func (LogLevel) From(
	val any,
	def ...LogLevel,
) LogLevel {
	if level, ok := logLevelOf(val); ok {
		return level
	}
	if len(def) > 0 {
		return def[0]
	}
	return LogNone
}

// ILogSerializer serializes a log entry into its string representation.
// As an [IResource], its Id method returns the id it was created with by
// its [ILogSerializerFactory].
type ILogSerializer interface {
	io.Closer
	IResource

	// Serialize returns the representation of the entry.
	Serialize(timestamp time.Time, level LogLevel, message string, ctx IBag) string
}

// ILogSerializerCreator creates the serializers of an
// [ILogSerializerFactory] from their configuration.
type ILogSerializerCreator = IResourceCreator[ILogSerializer]

// ILogSerializerFactory creates and holds the serializers configured for
// the log streams.
type ILogSerializerFactory = IFactory[ILogSerializer]

// ILogStream represents a destination that log entries are written to,
// keeping the entries of its level or more severe: a signaled entry when
// it listens to its channel, and a broadcast entry whatever its channels.
// As an [IResource], its Id method returns the id it was created with by
// its [ILogStreamFactory].
type ILogStream interface {
	io.Closer
	IResource

	// GetLevel returns the least severe level the stream keeps.
	GetLevel() LogLevel

	// SetLevel sets the least severe level the stream keeps.
	SetLevel(level LogLevel) error

	// HasChannel reports whether the stream listens to the channel.
	HasChannel(channel string) bool

	// ListChannels returns the channels the stream listens to.
	ListChannels() []string

	// AddChannel makes the stream listen to the channel.
	AddChannel(channel string) error

	// RemoveChannel stops the stream listening to the channel.
	RemoveChannel(channel string) error

	// RemoveAllChannels stops the stream listening to any channel.
	RemoveAllChannels() error

	// Signal writes the entry when the stream keeps its level and listens
	// to its channel.
	Signal(timestamp time.Time, level LogLevel, channel, message string, ctx ...IBag) error

	// Broadcast writes the entry when the stream keeps its level.
	Broadcast(timestamp time.Time, level LogLevel, message string, ctx ...IBag) error
}

// ILogStreamCreator creates the streams of an [ILogStreamFactory] from
// their configuration.
type ILogStreamCreator = IResourceCreator[ILogStream]

// ILogStreamFactory creates and holds the log streams: those it stores
// are the streams an [ILogger] writes to.
type ILogStreamFactory = IFactory[ILogStream]

// ILogger keeps the log entries of an application, stamped with the time
// they are given at, until flushed: Flush then writes them, in order, to
// the streams, each keeping those of its level and channels. Signal gives
// an entry on a channel, and Broadcast an entry for every channel, and
// both have a method per level. The contexts, which may be left out, are
// copied as by IBag Clone, so that changing their bags, maps and slices
// afterwards does not change the entry, while the other values they hold,
// such as pointers, are shared with the entry until it is flushed; a nil
// context is skipped.
//
// The fatal level is a severity like the others: logging at that level
// neither flushes the logger nor stops the program.
//
// An ILogger is safe for concurrent use.
type ILogger interface {
	Signal(level LogLevel, channel, message string, ctx ...IBag)
	SignalFatal(channel, message string, ctx ...IBag)
	SignalError(channel, message string, ctx ...IBag)
	SignalWarning(channel, message string, ctx ...IBag)
	SignalNotice(channel, message string, ctx ...IBag)
	SignalInfo(channel, message string, ctx ...IBag)
	SignalDebug(channel, message string, ctx ...IBag)
	Broadcast(level LogLevel, message string, ctx ...IBag)
	BroadcastFatal(message string, ctx ...IBag)
	BroadcastError(message string, ctx ...IBag)
	BroadcastWarning(message string, ctx ...IBag)
	BroadcastNotice(message string, ctx ...IBag)
	BroadcastInfo(message string, ctx ...IBag)
	BroadcastDebug(message string, ctx ...IBag)

	// Flush writes the kept entries to every stream, then drops them,
	// even when a stream fails. A stream failing to write an entry is not
	// given the next ones: Flush returns the error of each failed stream,
	// joined.
	Flush() error
}

// logLevelOf returns the level held by the value, as described by
// LogLevel From, and whether it holds one.
func logLevelOf(
	val any,
) (LogLevel, bool) {
	if name, ok := val.(string); ok {
		switch strings.ToLower(name) {
		case levelLogNone:
			return LogNone, true
		case levelLogFatal:
			return LogFatal, true
		case levelLogError:
			return LogError, true
		case levelLogWarning:
			return LogWarning, true
		case levelLogNotice:
			return LogNotice, true
		case levelLogInfo:
			return LogInfo, true
		case levelLogDebug:
			return LogDebug, true
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(name), 64)
		if err != nil {
			return LogNone, false
		}
		val = n
	}

	var n float64
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n = float64(rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n = float64(rv.Uint())
	case reflect.Float32, reflect.Float64:
		n = rv.Float()
	default:
		return LogNone, false
	}
	if n != math.Trunc(n) || n < float64(LogNone) || n > float64(LogDebug) {
		return LogNone, false
	}
	return LogLevel(n), true
}
