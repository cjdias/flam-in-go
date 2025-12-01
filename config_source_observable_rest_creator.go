package flam

type observableRestConfigSourceCreator struct {
	config                    Config
	configRestClientGenerator ConfigRestClientGenerator
	configParserFactory       ConfigParserFactory
	timer                     Timer
}

var _ ConfigSourceCreator = (*observableRestConfigSourceCreator)(nil)

func newObservableRestConfigSourceCreator(
	config Config,
	configRestClientGenerator ConfigRestClientGenerator,
	configParserFactory ConfigParserFactory,
	timer Timer,
) ConfigSourceCreator {
	return &observableRestConfigSourceCreator{
		config:                    config,
		configRestClientGenerator: configRestClientGenerator,
		configParserFactory:       configParserFactory,
		timer:                     timer}
}

func (creator observableRestConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverObservableRest
}

func (creator observableRestConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	uri := config.String("uri")
	parserId := config.String("parser_id", creator.config.String(PathConfigDefaultRestParserId))
	configPath := config.String("path.config", creator.config.String(PathConfigDefaultRestConfigPath))
	timestampPath := config.String("path.timestamp", creator.config.String(PathConfigDefaultRestTimestampPath))

	switch {
	case uri == "":
		return nil, newErrInvalidResourceConfig("observableRestConfigSource", "uri", config)
	case parserId == "":
		return nil, newErrInvalidResourceConfig("observableRestConfigSource", "parser_id", config)
	case configPath == "":
		return nil, newErrInvalidResourceConfig("observableRestConfigSource", "config_path", config)
	case timestampPath == "":
		return nil, newErrInvalidResourceConfig("observableRestConfigSource", "timestamp_path", config)
	}

	requester, e := creator.configRestClientGenerator.Create()
	if e != nil {
		return nil, e
	}

	parser, e := creator.configParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	return newObservableRestConfigSource(
		priority,
		requester,
		uri,
		parser,
		configPath,
		timestampPath,
		creator.timer)
}
