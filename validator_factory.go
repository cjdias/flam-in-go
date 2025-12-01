package flam

import (
	"io"

	"go.uber.org/dig"
)

type ValidatorFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (Validator, error)
	Store(id string, validator Validator) error
	Remove(id string) error
	RemoveAll() error
}

type validatorFactoryArgs struct {
	dig.In

	Creators      []ValidatorCreator `group:"flam.validators.creator"`
	FactoryConfig FactoryConfig
}

func newValidatorFactory(
	args validatorFactoryArgs,
) (ValidatorFactory, error) {
	var creators []FactoryResourceCreator[Validator]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("Validator"),
		PathValidators)
}
