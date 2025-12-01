package flam

import (
	"io"

	"go.uber.org/dig"
)

type CacheAdaptorFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (CacheAdaptor, error)
	Store(id string, adaptor CacheAdaptor) error
	Remove(id string) error
	RemoveAll() error
}

type cacheAdaptorFactoryArgs struct {
	dig.In

	Creators      []CacheAdaptorCreator `group:"flam.cache.adaptors.creator"`
	FactoryConfig FactoryConfig
}

func newCacheAdaptorFactory(
	args cacheAdaptorFactoryArgs,
) (CacheAdaptorFactory, error) {
	var creators []FactoryResourceCreator[CacheAdaptor]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("CacheAdaptor"),
		PathCacheAdaptors)
}
