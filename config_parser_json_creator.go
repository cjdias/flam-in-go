package flam

type jsonConfigParserCreator struct{}

var _ ConfigParserCreator = (*jsonConfigParserCreator)(nil)

func newJsonConfigParserCreator() ConfigParserCreator {
	return &jsonConfigParserCreator{}
}

func (jsonConfigParserCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == ConfigParserDriverJson
}

func (jsonConfigParserCreator) Create(
	_ Bag,
) (ConfigParser, error) {
	return newJsonConfigParser(), nil
}
