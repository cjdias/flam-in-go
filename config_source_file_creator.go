package flam

type fileConfigSourceCreator struct {
	config              Config
	diskFactory         DiskFactory
	configParserFactory ConfigParserFactory
}

var _ ConfigSourceCreator = (*fileConfigSourceCreator)(nil)

func newFileConfigSourceCreator(
	config Config,
	diskFactory DiskFactory,
	configParserFactory ConfigParserFactory,
) ConfigSourceCreator {
	return &fileConfigSourceCreator{
		config:              config,
		diskFactory:         diskFactory,
		configParserFactory: configParserFactory}
}

func (creator fileConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverFile
}

func (creator fileConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	diskId := config.String("disk_id", creator.config.String(PathConfigDefaultFileDiskId))
	path := config.String("path")
	parserId := config.String("parser_id", creator.config.String(PathConfigDefaultFileParserId))

	switch {
	case diskId == "":
		return nil, newErrInvalidResourceConfig("fileConfigSource", "disk_id", config)
	case path == "":
		return nil, newErrInvalidResourceConfig("fileConfigSource", "path", config)
	case parserId == "":
		return nil, newErrInvalidResourceConfig("fileConfigSource", "parser_id", config)
	}

	disk, e := creator.diskFactory.Get(diskId)
	if e != nil {
		return nil, e
	}

	parser, e := creator.configParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	return newFileConfigSource(
		priority,
		disk,
		path,
		parser)
}
