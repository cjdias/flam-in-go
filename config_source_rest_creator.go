package flam

type restConfigSourceCreator struct {
	config                    Config
	configRestClientGenerator ConfigRestClientGenerator
	configParserFactory       ConfigParserFactory
}

var _ ConfigSourceCreator = (*restConfigSourceCreator)(nil)

func newRestConfigSourceCreator(
	config Config,
	configRestClientGenerator ConfigRestClientGenerator,
	configParserFactory ConfigParserFactory,
) ConfigSourceCreator {
	return &restConfigSourceCreator{
		config:                    config,
		configRestClientGenerator: configRestClientGenerator,
		configParserFactory:       configParserFactory}
}

func (creator restConfigSourceCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigSourceDriverRest
}

func (creator restConfigSourceCreator) Create(
	config Bag,
) (ConfigSource, error) {
	priority := config.Int("priority", creator.config.Int(PathConfigDefaultPriority))
	uri := config.String("uri")
	parserId := config.String("parser_id", creator.config.String(PathConfigDefaultRestParserId))
	configPath := config.String("path.config", creator.config.String(PathConfigDefaultRestConfigPath))

	switch {
	case uri == "":
		return nil, newErrInvalidResourceConfig("restConfigSource", "uri", config)
	case parserId == "":
		return nil, newErrInvalidResourceConfig("restConfigSource", "parser_id", config)
	case configPath == "":
		return nil, newErrInvalidResourceConfig("restConfigSource", "config_path", config)
	}

	requester, e := creator.configRestClientGenerator.Create()
	if e != nil {
		return nil, e
	}

	parser, e := creator.configParserFactory.Get(parserId)
	if e != nil {
		return nil, e
	}

	return newRestConfigSource(
		priority,
		requester,
		uri,
		parser,
		configPath)
}
