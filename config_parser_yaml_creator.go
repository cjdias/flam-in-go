package flam

type yamlConfigParserCreator struct{}

var _ ConfigParserCreator = (*yamlConfigParserCreator)(nil)

func newYamlConfigParserCreator() ConfigParserCreator {
	return &yamlConfigParserCreator{}
}

func (yamlConfigParserCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigParserDriverYaml
}

func (yamlConfigParserCreator) Create(
	_ Bag,
) (ConfigParser, error) {
	return newYamlConfigParser(), nil
}
