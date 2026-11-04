package validator

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// converterFactoryArgs are the dependencies of the converter factory, taken
// from the container. The group tag repeats ConverterCreatorGroup, as a
// struct tag cannot use a constant.
type converterFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IValidationErrorConverterCreator `group:"flam.validator.converters.creator"`
}

// newConverterFactory returns the converter factory creating the converters
// configured at ConverterConfigPath with the creators of the
// ConverterCreatorGroup group.
func newConverterFactory(
	args converterFactoryArgs,
) (flam.IValidationErrorConverterFactory, error) {
	return factory.New(args.Config.Bag(ConverterConfigPath), args.Creators...)
}
