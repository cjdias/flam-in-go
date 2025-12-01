package flam

import (
	"io"

	"go.uber.org/dig"
)

type ValidatorParserFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (ValidatorParser, error)
	Store(id string, parser ValidatorParser) error
	Remove(id string) error
	RemoveAll() error
}

type validatorParserFactoryArgs struct {
	dig.In

	Creators      []ValidatorParserCreator `group:"flam.validators.parsers.creator"`
	FactoryConfig FactoryConfig
}

func newValidatorParserFactory(
	args validatorParserFactoryArgs,
) (ValidatorParserFactory, error) {
	var creators []FactoryResourceCreator[ValidatorParser]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("ValidatorParser"),
		PathValidatorParsers)
}
