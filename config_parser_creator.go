package flam

type ConfigParserCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (ConfigParser, error)
}
