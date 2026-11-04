package validator

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// validatorFactoryArgs are the dependencies of the validator factory, taken
// from the container. The group tag repeats ValidatorCreatorGroup, as a
// struct tag cannot use a constant.
type validatorFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IValidatorCreator `group:"flam.validator.validators.creator"`
}

// newValidatorFactory returns the validator factory creating the validators
// configured at ValidatorConfigPath with the creators of the
// ValidatorCreatorGroup group.
func newValidatorFactory(
	args validatorFactoryArgs,
) (flam.IValidatorFactory, error) {
	return factory.New(args.Config.Bag(ValidatorConfigPath), args.Creators...)
}
