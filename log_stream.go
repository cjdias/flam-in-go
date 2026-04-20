package flam

import (
	"io"
	"slices"
	"sort"
	"sync"
	"time"
)

type LogStream interface {
	Close() error

	GetLevel() LogLevel
	SetLevel(level LogLevel) error

	HasChannel(channel string) bool
	ListChannels() []string
	AddChannel(channel string) error
	RemoveChannel(channel string) error
	RemoveAllChannels() error

	Signal(timestamp time.Time, level LogLevel, channel, message string, ctx Bag) error
	Broadcast(timestamp time.Time, level LogLevel, message string, ctx Bag) error
}

type logStream struct {
	mu            sync.Mutex
	level         LogLevel
	channels      []string
	logSerializer LogSerializer
	writer        io.Writer
	doClose       bool
}

func newLogStream(
	level LogLevel,
	channels []string,
	logSerializer LogSerializer,
	writer io.Writer,
	doClose bool,
) *logStream {
	sort.Strings(channels)

	return &logStream{
		level:         level,
		channels:      channels,
		logSerializer: logSerializer,
		writer:        writer,
		doClose:       doClose}
}

func (stream *logStream) Close() error {
	if closer, ok := stream.writer.(io.Closer); stream.doClose && ok {
		return closer.Close()
	}

	return nil
}

func (stream *logStream) GetLevel() LogLevel {
	return stream.level
}

func (stream *logStream) SetLevel(
	level LogLevel,
) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.level = level

	return nil
}

func (stream *logStream) HasChannel(
	channel string,
) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return slices.Contains(stream.channels, channel)
}

func (stream *logStream) ListChannels() []string {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	return stream.channels
}

func (stream *logStream) AddChannel(
	channel string,
) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if !slices.Contains(stream.channels, channel) {
		stream.channels = append(stream.channels, channel)
		sort.Strings(stream.channels)
	}

	return nil
}

func (stream *logStream) RemoveChannel(
	channel string,
) error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.channels = slices.DeleteFunc(stream.channels, func(c string) bool {
		return c == channel
	})

	return nil
}

func (stream *logStream) RemoveAllChannels() error {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	stream.channels = []string{}

	return nil
}

func (stream *logStream) Signal(
	timestamp time.Time,
	level LogLevel,
	channel string,
	message string,
	ctx Bag,
) error {
	if !stream.acceptChannel(channel) {
		return nil
	}

	ctx["channel"] = channel

	return stream.Broadcast(timestamp, level, message, ctx)
}

func (stream *logStream) Broadcast(
	timestamp time.Time,
	level LogLevel,
	message string,
	ctx Bag,
) error {
	if stream.level < level || stream.level == LogNone {
		return nil
	}

	serialized := stream.logSerializer.Serialize(timestamp, level, message, ctx)
	_, e := stream.writer.Write([]byte(serialized))

	return e
}

func (stream *logStream) acceptChannel(
	channel string,
) bool {
	stream.mu.Lock()
	defer stream.mu.Unlock()
	i := sort.SearchStrings(stream.channels, "*")
	if i != len(stream.channels) && stream.channels[i] == "*" {
		return true
	}

	i = sort.SearchStrings(stream.channels, channel)

	return i != len(stream.channels) && stream.channels[i] == channel
}
