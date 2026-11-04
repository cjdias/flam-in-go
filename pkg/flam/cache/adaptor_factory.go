package cache

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// adaptorFactoryArgs are the dependencies of the adaptor factory, taken
// from the container. The group tag repeats AdaptorCreatorGroup, as a
// struct tag cannot use a constant.
type adaptorFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ICacheAdaptorCreator `group:"flam.cache.adaptors.creator"`
}

// newAdaptorFactory returns the adaptor factory creating the adaptors
// configured at AdaptorConfigPath with the creators of the
// AdaptorCreatorGroup group.
func newAdaptorFactory(
	args adaptorFactoryArgs,
) (flam.ICacheAdaptorFactory, error) {
	return factory.New(args.Config.Bag(AdaptorConfigPath), args.Creators...)
}
