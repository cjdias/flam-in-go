package flam

import "fmt"

type defaultWatchdogLoggerLevels struct {
	start LogLevel
	error LogLevel
	done  LogLevel
}

type defaultWatchdogLogger struct {
	logger  Logger
	channel string
	levels  defaultWatchdogLoggerLevels
}

var _ WatchdogLogger = (*defaultWatchdogLogger)(nil)

func newDefaultWatchdogLogger(
	logget Logger,
	channel string,
	startLevel LogLevel,
	errorLevel LogLevel,
	doneLevel LogLevel,
) WatchdogLogger {
	return &defaultWatchdogLogger{
		logger:  logget,
		channel: channel,
		levels: defaultWatchdogLoggerLevels{
			start: startLevel,
			error: errorLevel,
			done:  doneLevel,
		},
	}
}

func (logger defaultWatchdogLogger) LogStart(
	id string,
) {
	logger.logger.Signal(
		logger.levels.start,
		logger.channel,
		fmt.Sprintf("process [%s] starting ...", id))
}

func (logger defaultWatchdogLogger) LogError(
	id string,
	e error,
) {
	logger.logger.Signal(
		logger.levels.error,
		logger.channel,
		fmt.Sprintf("process [%s] error : %v", id, e))
}

func (logger defaultWatchdogLogger) LogDone(
	id string,
) {
	logger.logger.Signal(
		logger.levels.done,
		logger.channel,
		fmt.Sprintf("process [%s] terminated", id))
}
