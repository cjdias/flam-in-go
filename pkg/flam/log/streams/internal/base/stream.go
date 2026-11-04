package base

import (
	"io"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
)

// Writer writes the lines of the entries of a stream, told the timestamp
// of each entry, such as to choose the file of its day. Its calls are
// serialized by the stream.
type Writer interface {
	WriteLine(timestamp time.Time, line string) error
}

// Plain returns the Writer writing the lines to w, whatever their
// timestamp.
func Plain(
	w io.Writer,
) Writer {
	return plainWriter{w: w}
}

// New returns a stream of the id keeping the entries of the level or more
// severe, signaled on one of the channels, or broadcast, and writing them,
// serialized, to the writer, one per line. Closing the stream closes the
// closer, which may be nil, but not the serializer, owned by its factory.
func New(
	id string,
	level flam.LogLevel,
	channels []string,
	serializer flam.ILogSerializer,
	writer Writer,
	closer io.Closer,
) *Stream {
	s := &Stream{
		id:         id,
		level:      level,
		serializer: serializer,
		writer:     writer,
		closer:     closer,
	}
	for _, channel := range channels {
		s.addChannel(channel)
	}
	return s
}

// Stream is the [flam.ILogStream] of the drivers. It is safe for concurrent
// use: its writes are serialized, so that the lines never interleave.
type Stream struct {
	id         string
	serializer flam.ILogSerializer
	writer     Writer
	closer     io.Closer

	mu       sync.Mutex // guards the fields below, and the writes
	level    flam.LogLevel
	channels []string // sorted
	closed   bool
}

// compile time check that *Stream implements ILogStream
var _ flam.ILogStream = (*Stream)(nil)

func (s *Stream) Id() string {
	return s.id
}

// Close closes the closer given to New, once, and returns its error. The
// writes fail afterwards with log.ErrStreamClosed.
func (s *Stream) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

func (s *Stream) GetLevel() flam.LogLevel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.level
}

// SetLevel sets the level, failing with log.ErrInvalidLevel when it is not
// between flam.LogNone, which keeps no entry, and flam.LogDebug.
func (s *Stream) SetLevel(
	level flam.LogLevel,
) error {
	if level < flam.LogNone || level > flam.LogDebug {
		return flam.NewWrappedError(log.ErrInvalidLevel, streamSetLevelMessage).
			With(log.ErrorFieldID, s.id).
			With(log.ErrorFieldLevel, level)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.level = level
	return nil
}

func (s *Stream) HasChannel(
	channel string,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := slices.BinarySearch(s.channels, channel)
	return found
}

// ListChannels returns, sorted, the channels the stream listens to.
func (s *Stream) ListChannels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.channels)
}

// AddChannel makes the stream listen to the channel, log.AllChannels
// making it listen to any channel. It fails with log.ErrInvalidChannel for
// an empty channel, and does nothing for a channel listened to already.
func (s *Stream) AddChannel(
	channel string,
) error {
	if channel == "" {
		return flam.NewWrappedError(log.ErrInvalidChannel, streamAddChannelMessage).
			With(log.ErrorFieldID, s.id).
			With(log.ErrorFieldChannel, channel)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addChannel(channel)
	return nil
}

// RemoveChannel stops the stream listening to the channel, if it does.
func (s *Stream) RemoveChannel(
	channel string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i, found := slices.BinarySearch(s.channels, channel); found {
		s.channels = slices.Delete(s.channels, i, i+1)
	}
	return nil
}

func (s *Stream) RemoveAllChannels() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels = nil
	return nil
}

// Signal writes the entry when the stream keeps its level and listens to
// its channel, or to log.AllChannels, with the channel added to its
// context at log.ContextFieldChannel.
func (s *Stream) Signal(
	timestamp time.Time,
	level flam.LogLevel,
	channel, message string,
	ctx ...flam.IBag,
) error {
	return s.write(timestamp, level, true, channel, message, ctx)
}

// Broadcast writes the entry when the stream keeps its level.
func (s *Stream) Broadcast(
	timestamp time.Time,
	level flam.LogLevel,
	message string,
	ctx ...flam.IBag,
) error {
	return s.write(timestamp, level, false, "", message, ctx)
}

// Messages of the errors returned by the stream.
const (
	streamSetLevelMessage   = "unable to set log stream level"
	streamAddChannelMessage = "unable to add log stream channel"
	streamWriteMessage      = "unable to write log entry"
)

// addChannel adds the channel to the sorted channels, unless there
// already.
func (s *Stream) addChannel(
	channel string,
) {
	if i, found := slices.BinarySearch(s.channels, channel); !found {
		s.channels = slices.Insert(s.channels, i, channel)
	}
}

// write serializes the entry, with its contexts merged, the later winning,
// and writes it, followed by a newline, when the stream keeps it. An entry
// of level flam.LogNone is never kept.
func (s *Stream) write(
	timestamp time.Time,
	level flam.LogLevel,
	signal bool,
	channel, message string,
	ctx []flam.IBag,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return flam.NewWrappedError(log.ErrStreamClosed, streamWriteMessage).
			With(log.ErrorFieldID, s.id)
	}
	if level <= flam.LogNone || level > s.level {
		return nil
	}
	if signal && !s.listens(channel) {
		return nil
	}

	merged := container.Bag{}
	for _, c := range ctx {
		switch bag := c.(type) {
		case container.Bag: // as the logger gives, copied without sorting
			maps.Copy(merged, bag)
		default:
			if value.IsNil(c) {
				continue
			}
			for key, val := range c.All() {
				merged[key] = val
			}
		}
	}
	if signal {
		merged[log.ContextFieldChannel] = channel
	}

	line := s.serializer.Serialize(timestamp, level, message, merged) + "\n"
	if err := s.writer.WriteLine(timestamp, line); err != nil {
		return flam.NewWrappedError(err, streamWriteMessage).
			With(log.ErrorFieldID, s.id)
	}
	return nil
}

// listens reports whether the stream listens to the channel, or to every
// channel.
func (s *Stream) listens(
	channel string,
) bool {
	for _, c := range []string{log.AllChannels, channel} {
		if _, found := slices.BinarySearch(s.channels, c); found {
			return true
		}
	}
	return false
}

// plainWriter is the Writer returned by Plain.
type plainWriter struct {
	w io.Writer
}

func (p plainWriter) WriteLine(
	_ time.Time,
	line string,
) error {
	_, err := io.WriteString(p.w, line)
	return err
}
