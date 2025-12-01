package flam

import (
	"sync"
	"time"
)

type Logger interface {
	Signal(level LogLevel, channel, message string, ctx ...Bag)
	SignalFatal(channel, message string, ctx ...Bag)
	SignalError(channel, message string, ctx ...Bag)
	SignalWarning(channel, message string, ctx ...Bag)
	SignalNotice(channel, message string, ctx ...Bag)
	SignalInfo(channel, message string, ctx ...Bag)
	SignalDebug(channel, message string, ctx ...Bag)
	Broadcast(level LogLevel, message string, ctx ...Bag)
	BroadcastFatal(message string, ctx ...Bag)
	BroadcastError(message string, ctx ...Bag)
	BroadcastWarning(message string, ctx ...Bag)
	BroadcastNotice(message string, ctx ...Bag)
	BroadcastInfo(message string, ctx ...Bag)
	BroadcastDebug(message string, ctx ...Bag)
	Flush() error
}

type loggerEntryReg struct {
	timestamp time.Time
	level     LogLevel
	channel   string
	message   string
	ctx       Bag
}

type logger struct {
	locker  sync.Locker
	streams map[string]LogStream
	buffer  []loggerEntryReg
}

func newLogger() *logger {
	return &logger{
		locker:  &sync.Mutex{},
		streams: map[string]LogStream{},
		buffer:  []loggerEntryReg{}}
}

func (logger *logger) Close() error {
	return logger.Flush()
}

func (logger *logger) Signal(
	level LogLevel,
	channel,
	message string,
	ctx ...Bag,
) {
	context := Bag{}
	for _, c := range ctx {
		context.Merge(c)
	}

	logger.locker.Lock()
	defer logger.locker.Unlock()

	logger.buffer = append(logger.buffer, loggerEntryReg{
		timestamp: time.Now(),
		level:     level,
		channel:   channel,
		message:   message,
		ctx:       context})
}

func (logger *logger) SignalFatal(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogFatal, channel, message, ctx...)
}

func (logger *logger) SignalError(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogError, channel, message, ctx...)
}

func (logger *logger) SignalWarning(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogWarning, channel, message, ctx...)
}

func (logger *logger) SignalNotice(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogNotice, channel, message, ctx...)
}

func (logger *logger) SignalInfo(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogInfo, channel, message, ctx...)
}

func (logger *logger) SignalDebug(
	channel,
	message string,
	ctx ...Bag,
) {
	logger.Signal(LogDebug, channel, message, ctx...)
}

func (logger *logger) Broadcast(
	level LogLevel,
	message string,
	ctx ...Bag,
) {
	context := Bag{}
	for _, c := range ctx {
		context.Merge(c)
	}

	logger.locker.Lock()
	defer logger.locker.Unlock()

	logger.buffer = append(logger.buffer, loggerEntryReg{
		timestamp: time.Now(),
		level:     level,
		channel:   "",
		message:   message,
		ctx:       context})
}

func (logger *logger) BroadcastFatal(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogFatal, message, ctx...)
}

func (logger *logger) BroadcastError(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogError, message, ctx...)
}

func (logger *logger) BroadcastWarning(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogWarning, message, ctx...)
}

func (logger *logger) BroadcastNotice(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogNotice, message, ctx...)
}

func (logger *logger) BroadcastInfo(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogInfo, message, ctx...)
}

func (logger *logger) BroadcastDebug(
	message string,
	ctx ...Bag,
) {
	logger.Broadcast(LogDebug, message, ctx...)
}

func (logger *logger) Flush() error {
	logger.locker.Lock()
	defer logger.locker.Unlock()

	for _, entry := range logger.buffer {
		for _, stream := range logger.streams {
			if entry.channel != "" {
				if e := stream.Signal(
					entry.timestamp,
					entry.level,
					entry.channel,
					entry.message,
					entry.ctx,
				); e != nil {
					return e
				}
			} else {
				if e := stream.Broadcast(
					entry.timestamp,
					entry.level,
					entry.message,
					entry.ctx,
				); e != nil {
					return e
				}
			}
		}
	}

	logger.buffer = []loggerEntryReg{}

	return nil
}
