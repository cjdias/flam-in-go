package translator

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// factoryArgs are the dependencies of the translator factory, taken from
// the container. The group tag repeats CreatorGroup, as a struct tag
// cannot use a constant.
type factoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ITranslatorCreator `group:"flam.translator.translators.creator"`
}

// newFactory returns the translator factory creating the translators
// configured at ConfigPath with the creators of the CreatorGroup group.
func newFactory(
	args factoryArgs,
) (flam.ITranslatorFactory, error) {
	return factory.New(args.Config.Bag(ConfigPath), args.Creators...)
}
