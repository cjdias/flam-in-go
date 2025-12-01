package flam

import (
	"io"

	"go.uber.org/dig"
)

type ConfigParserFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (ConfigParser, error)
	Store(id string, parser ConfigParser) error
	Remove(id string) error
	RemoveAll() error
}

type configParserFactoryArgs struct {
	dig.In

	Creators      []ConfigParserCreator `group:"flam.config.parsers.creator"`
	FactoryConfig FactoryConfig
}

func newConfigParserFactory(
	args configParserFactoryArgs,
) (ConfigParserFactory, error) {
	var creators []FactoryResourceCreator[ConfigParser]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("ConfigParser"),
		PathConfigParsers)
}
