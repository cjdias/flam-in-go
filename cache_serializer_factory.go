package flam

import (
	"io"

	"go.uber.org/dig"
)

type CacheSerializerFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (CacheSerializer, error)
	Store(id string, serializer CacheSerializer) error
	Remove(id string) error
	RemoveAll() error
}

type cacheSerializerFactoryArgs struct {
	dig.In

	Creators      []CacheSerializerCreator `group:"flam.cache.serializers.creator"`
	FactoryConfig FactoryConfig
}

func newCacheSerializerFactory(
	args cacheSerializerFactoryArgs,
) (CacheSerializerFactory, error) {
	var creators []FactoryResourceCreator[CacheSerializer]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("CacheSerializer"),
		PathCacheSerializers)
}
