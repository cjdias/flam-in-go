package flam

import "os"

type fileLogStreamCreator struct {
	logStreamCreator

	config      Config
	diskFactory DiskFactory
}

var _ LogStreamCreator = (*fileLogStreamCreator)(nil)

func newFileLogStreamCreator(
	config Config,
	diskFactory DiskFactory,
	logSerializerFactory LogSerializerFactory,
) LogStreamCreator {
	return &fileLogStreamCreator{
		logStreamCreator: logStreamCreator{
			logSerializerFactory: logSerializerFactory},
		config:      config,
		diskFactory: diskFactory}
}

func (creator fileLogStreamCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == LogStreamDriverFile
}

func (creator fileLogStreamCreator) Create(
	config Bag,
) (LogStream, error) {
	level := LogLevelFrom(config.Get("level"), LogLevelFrom(creator.config.String(PathLogDefaultLevel)))
	channels := creator.getChannels(config.Slice("channels"))
	serializerId := config.String("serializer_id", creator.config.String(PathLogDefaultSerializerId))
	diskId := config.String("disk_id", creator.config.String(PathLogDefaultDiskId))
	path := config.String("path")

	switch {
	case serializerId == "":
		return nil, newErrInvalidResourceConfig("fileLogStream", "serializer_id", config)
	case diskId == "":
		return nil, newErrInvalidResourceConfig("fileLogStream", "disk_id", config)
	case path == "":
		return nil, newErrInvalidResourceConfig("fileLogStream", "path", config)
	}

	serializer, e := creator.logSerializerFactory.Get(serializerId)
	if e != nil {
		return nil, e
	}

	disk, e := creator.diskFactory.Get(diskId)
	if e != nil {
		return nil, e
	}

	file, e := disk.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if e != nil {
		return nil, e
	}

	return newLogStream(level, channels, serializer, file, true), nil
}
