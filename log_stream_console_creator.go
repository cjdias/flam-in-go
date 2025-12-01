package flam

import "os"

type consoleLogStreamCreator struct {
	logStreamCreator

	config Config
}

var _ LogStreamCreator = (*consoleLogStreamCreator)(nil)

func newConsoleLogStreamCreator(
	config Config,
	logSerializerFactory LogSerializerFactory,
) LogStreamCreator {
	return &consoleLogStreamCreator{
		logStreamCreator: logStreamCreator{
			logSerializerFactory: logSerializerFactory},
		config: config}
}

func (creator consoleLogStreamCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == LogStreamDriverConsole
}

func (creator consoleLogStreamCreator) Create(
	config Bag,
) (LogStream, error) {
	level := LogLevelFrom(config.Get("level"), LogLevelFrom(creator.config.String(PathLogDefaultLevel)))
	channels := creator.getChannels(config.Slice("channels"))
	serializerId := config.String("serializer_id", creator.config.String(PathLogDefaultSerializerId))

	if serializerId == "" {
		return nil, newErrInvalidResourceConfig("consoleLogStream", "serializer_id", config)
	}

	serializer, e := creator.logSerializerFactory.Get(serializerId)
	if e != nil {
		return nil, e
	}

	return newLogStream(level, channels, serializer, os.Stdout, false), nil
}
