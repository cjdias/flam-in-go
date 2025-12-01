package flam

type defaultMigratorLoggerCreator struct {
	config Config
	logger Logger
}

var _ MigratorLoggerCreator = (*defaultMigratorLoggerCreator)(nil)

func newDefaultMigratorLoggerCreator(
	config Config,
	logger Logger,
) MigratorLoggerCreator {
	return &defaultMigratorLoggerCreator{
		config: config,
		logger: logger}
}

func (creator defaultMigratorLoggerCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == MigratorLoggerDriverDefault
}

func (creator defaultMigratorLoggerCreator) Create(
	config Bag,
) (MigratorLogger, error) {
	channel := config.String("channel", creator.config.String(PathMigratorDefaultLoggerChannel))
	startLevel := LogLevelFrom(config.Get("levels.start", creator.config.Get(PathMigratorDefaultLoggerStartLevel)))
	errorLevel := LogLevelFrom(config.Get("levels.error", creator.config.Get(PathMigratorDefaultLoggerErrorLevel)))
	doneLevel := LogLevelFrom(config.Get("levels.done", creator.config.Get(PathMigratorDefaultLoggerDoneLevel)))

	if channel == "" {
		return nil, newErrInvalidResourceConfig("defaultMigratorLogger", "channel", config)
	}

	return newDefaultMigratorLogger(
		creator.logger,
		channel,
		startLevel,
		errorLevel,
		doneLevel), nil
}
