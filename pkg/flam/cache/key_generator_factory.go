package cache

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// keyGeneratorFactoryArgs are the dependencies of the key generator
// factory, taken from the container. The group tag repeats
// KeyGeneratorCreatorGroup, as a struct tag cannot use a constant.
type keyGeneratorFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ICacheKeyGeneratorCreator `group:"flam.cache.key_generators.creator"`
}

// newKeyGeneratorFactory returns the key generator factory creating the
// key generators configured at KeyGeneratorConfigPath with the creators of
// the KeyGeneratorCreatorGroup group.
func newKeyGeneratorFactory(
	args keyGeneratorFactoryArgs,
) (flam.ICacheKeyGeneratorFactory, error) {
	return factory.New(args.Config.Bag(KeyGeneratorConfigPath), args.Creators...)
}
