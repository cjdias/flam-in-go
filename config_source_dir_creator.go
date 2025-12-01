package flam

type dirConfigSourceCreator struct {
	config              Config
	diskFactory         DiskFactory
	configParserFactory ConfigParserFactory
}

var _ ConfigSourceCreator = (*dirConfigSourceCreator)(nil)

func newDirConfigSourceCreator(
	config Config,
	diskFactory DiskFactory,
	configParserFactory ConfigParserFactory,
) ConfigSourceCreator {
	return &dirConfigSourceCreator{
		config:              config,
		diskFactory:         diskFactory,
		configParserFactory: configParserFactory}
}

func (creator dirConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverDir
}

func (creator dirConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	diskId := config.String("disk_id", creator.config.String(PathConfigDefaultFileDiskId))
	path := config.String("path")
	parserId := config.String("parser_id", creator.config.String(PathConfigDefaultFileParserId))
	recursive := config.Bool("recursive")

	switch {
	case diskId == "":
		return nil, newErrInvalidResourceConfig("dirConfigSource", "disk_id", config)
	case path == "":
		return nil, newErrInvalidResourceConfig("dirConfigSource", "path", config)
	case parserId == "":
		return nil, newErrInvalidResourceConfig("dirConfigSource", "parser_id", config)
	}

	disk, e := creator.diskFactory.Get(diskId)
	if e != nil {
		return nil, e
	}

	parser, e := creator.configParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	return newDirConfigSource(
		priority,
		disk,
		path,
		parser,
		recursive)
}
