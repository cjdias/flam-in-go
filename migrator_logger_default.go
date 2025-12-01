package flam

import "fmt"

type defaultMigratorLoggerLevels struct {
	start LogLevel
	error LogLevel
	done  LogLevel
}

type defaultMigratorLogger struct {
	logger  Logger
	channel string
	level   defaultMigratorLoggerLevels
}

var _ MigratorLogger = (*defaultMigratorLogger)(nil)

func newDefaultMigratorLogger(
	logger Logger,
	channel string,
	startLevel LogLevel,
	errorLevel LogLevel,
	doneLevel LogLevel,
) MigratorLogger {
	return &defaultMigratorLogger{
		logger:  logger,
		channel: channel,
		level: defaultMigratorLoggerLevels{
			start: startLevel,
			error: errorLevel,
			done:  doneLevel}}
}

func (logger *defaultMigratorLogger) LogUpStart(
	migration MigrationInfo,
) {
	logger.logger.Signal(
		logger.level.start,
		logger.channel,
		fmt.Sprintf("migration '%s' up action started", migration.Version))
}

func (logger *defaultMigratorLogger) LogUpError(
	migration MigrationInfo,
	e error,
) {
	logger.logger.Signal(
		logger.level.error,
		logger.channel,
		fmt.Sprintf("migration '%s' up action error: %v", migration.Version, e))
}

func (logger *defaultMigratorLogger) LogUpDone(
	migration MigrationInfo,
) {
	logger.logger.Signal(
		logger.level.done,
		logger.channel,
		fmt.Sprintf("migration '%s' up action terminated", migration.Version))
}

func (logger *defaultMigratorLogger) LogDownStart(
	migration MigrationInfo,
) {
	logger.logger.Signal(
		logger.level.start,
		logger.channel,
		fmt.Sprintf("migration '%s' down action started", migration.Version))
}

func (logger *defaultMigratorLogger) LogDownError(
	migration MigrationInfo,
	e error,
) {
	logger.logger.Signal(
		logger.level.error,
		logger.channel,
		fmt.Sprintf("migration '%s' down action error: %v", migration.Version, e))
}

func (logger *defaultMigratorLogger) LogDownDone(
	migration MigrationInfo,
) {
	logger.logger.Signal(
		logger.level.done,
		logger.channel,
		fmt.Sprintf("migration '%s' down action terminated", migration.Version))
}
