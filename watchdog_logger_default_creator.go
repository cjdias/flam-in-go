package flam

type defaultWatchdogLoggerCreator struct {
	config Config
	logger Logger
}

var _ WatchdogLoggerCreator = (*defaultWatchdogLoggerCreator)(nil)

func newDefaultWatchdogLoggerCreator(
	config Config,
	logger Logger,
) WatchdogLoggerCreator {
	return &defaultWatchdogLoggerCreator{
		config: config,
		logger: logger}
}

func (defaultWatchdogLoggerCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == WatchdogLoggerDriverDefault
}

func (creator defaultWatchdogLoggerCreator) Create(
	config Bag,
) (WatchdogLogger, error) {
	channel := config.String("channel", creator.config.String(PathWatchdogDefaultLoggerChannel))
	startLevel := LogLevelFrom(config.Get("levels.start", creator.config.Get(PathWatchdogDefaultLoggerStartLevel)))
	errorLevel := LogLevelFrom(config.Get("levels.error", creator.config.Get(PathWatchdogDefaultLoggerErrorLevel)))
	doneLevel := LogLevelFrom(config.Get("levels.done", creator.config.Get(PathWatchdogDefaultLoggerDoneLevel)))

	if channel == "" {
		return nil, newErrInvalidResourceConfig("defaultWatchdogLogger", "channel", config)
	}

	return newDefaultWatchdogLogger(
		creator.logger,
		channel,
		startLevel,
		errorLevel,
		doneLevel), nil
}
