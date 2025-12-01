package flam

import (
	"io"

	"go.uber.org/dig"
)

type CacheKeyGeneratorFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (CacheKeyGenerator, error)
	Store(id string, generator CacheKeyGenerator) error
	Remove(id string) error
	RemoveAll() error
}

type cacheKeyGeneratorFactoryArgs struct {
	dig.In

	Creators      []CacheKeyGeneratorCreator `group:"flam.cache.key_generators.creator"`
	FactoryConfig FactoryConfig
}

func newCacheKeyGeneratorFactory(
	args cacheKeyGeneratorFactoryArgs,
) (CacheKeyGeneratorFactory, error) {
	var creators []FactoryResourceCreator[CacheKeyGenerator]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("CacheKeyGenerator"),
		PathCacheKeyGenerators)
}
