package cache

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// serializerFactoryArgs are the dependencies of the serializer factory,
// taken from the container. The group tag repeats SerializerCreatorGroup,
// as a struct tag cannot use a constant.
type serializerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ICacheSerializerCreator `group:"flam.cache.serializers.creator"`
}

// newSerializerFactory returns the serializer factory creating the
// serializers configured at SerializerConfigPath with the creators of the
// SerializerCreatorGroup group.
func newSerializerFactory(
	args serializerFactoryArgs,
) (flam.ICacheSerializerFactory, error) {
	return factory.New(args.Config.Bag(SerializerConfigPath), args.Creators...)
}
