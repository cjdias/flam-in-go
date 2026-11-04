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
// AdaptorCreatorGroup group, handing it to those implementing
// flam.ICacheAdaptorWrapperCreator.
func newAdaptorFactory(
	args adaptorFactoryArgs,
) (flam.ICacheAdaptorFactory, error) {
	f, err := factory.New(args.Config.Bag(AdaptorConfigPath), args.Creators...)
	if err != nil {
		return nil, err
	}
	for _, creator := range args.Creators {
		if wrapper, ok := creator.(flam.ICacheAdaptorWrapperCreator); ok {
			wrapper.UseFactory(f)
		}
	}
	return f, nil
}
