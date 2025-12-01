package flam

type rotatingFileLogStreamCreator struct {
	fileLogStreamCreator

	timer Timer
}

var _ LogStreamCreator = (*rotatingFileLogStreamCreator)(nil)

func newRotatingFileLogStreamCreator(
	config Config,
	timer Timer,
	diskFactory DiskFactory,
	logSerializerFactory LogSerializerFactory,
) LogStreamCreator {
	return &rotatingFileLogStreamCreator{
		fileLogStreamCreator: fileLogStreamCreator{
			logStreamCreator: logStreamCreator{
				logSerializerFactory: logSerializerFactory},
			config:      config,
			diskFactory: diskFactory},
		timer: timer}
}

func (creator rotatingFileLogStreamCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == LogStreamDriverRotatingFile
}

func (creator rotatingFileLogStreamCreator) Create(
	config Bag,
) (LogStream, error) {
	level := LogLevelFrom(config.Get("level"), LogLevelFrom(creator.config.String(PathLogDefaultLevel)))
	channels := creator.getChannels(config.Slice("channels"))
	serializerId := config.String("serializer_id", creator.config.String(PathLogDefaultSerializerId))
	diskId := config.String("disk_id", creator.config.String(PathLogDefaultDiskId))
	path := config.String("path")

	switch {
	case serializerId == "":
		return nil, newErrInvalidResourceConfig("rotatingFileLogStream", "serializer_id", config)
	case diskId == "":
		return nil, newErrInvalidResourceConfig("rotatingFileLogStream", "disk_id", config)
	case path == "":
		return nil, newErrInvalidResourceConfig("rotatingFileLogStream", "path", config)
	}

	serializer, e := creator.logSerializerFactory.Get(serializerId)
	if e != nil {
		return nil, e
	}

	disk, e := creator.diskFactory.Get(diskId)
	if e != nil {
		return nil, e
	}

	file, e := newRotatingFileLogWriter(disk, path, creator.timer)
	if e != nil {
		return nil, e
	}

	return newLogStream(level, channels, serializer, file, true), nil
}
