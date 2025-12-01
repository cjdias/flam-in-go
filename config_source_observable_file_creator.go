package flam

type observableFileConfigSourceCreator struct {
	fileConfigSourceCreator

	timer Timer
}

var _ ConfigSourceCreator = (*observableFileConfigSourceCreator)(nil)

func newObservableFileConfigSourceCreator(
	config Config,
	diskFactory DiskFactory,
	configParserFactory ConfigParserFactory,
	timer Timer,
) ConfigSourceCreator {
	return &observableFileConfigSourceCreator{
		fileConfigSourceCreator: fileConfigSourceCreator{
			config:              config,
			diskFactory:         diskFactory,
			configParserFactory: configParserFactory},
		timer: timer}
}

func (creator observableFileConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverObservableFile
}

func (creator observableFileConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	diskId := config.String("disk_id", creator.config.String(PathConfigDefaultFileDiskId))
	path := config.String("path")
	parserId := config.String("parser_id", creator.config.String(PathConfigDefaultFileParserId))

	switch {
	case diskId == "":
		return nil, newErrInvalidResourceConfig("observableFileConfigSource", "disk_id", config)
	case path == "":
		return nil, newErrInvalidResourceConfig("observableFileConfigSource", "path", config)
	case parserId == "":
		return nil, newErrInvalidResourceConfig("observableFileConfigSource", "parser_id", config)
	}

	disk, e := creator.diskFactory.Get(diskId)
	if e != nil {
		return nil, e
	}

	parser, e := creator.configParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	return newObservableFileConfigSource(
		priority,
		disk,
		path,
		parser,
		creator.timer)
}
