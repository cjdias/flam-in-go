package flam

import (
	"io"

	"go.uber.org/dig"
)

type ValidatorErrorConverterFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (ValidatorErrorConverter, error)
	Store(id string, converter ValidatorErrorConverter) error
	Remove(id string) error
	RemoveAll() error
}

type validatorErrorConverterFactoryArgs struct {
	dig.In

	Creators      []ValidatorErrorConverterCreator `group:"flam.validators.converters.creator"`
	FactoryConfig FactoryConfig
}

func newValidatorErrorConverterFactory(
	args validatorErrorConverterFactoryArgs,
) (ValidatorErrorConverterFactory, error) {
	var creators []FactoryResourceCreator[ValidatorErrorConverter]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("ValidatorErrorConverter"),
		PathValidatorErrorConverters)
}
