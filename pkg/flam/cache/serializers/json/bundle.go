package json

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the JSON serializer driver,
// providing its [flam.ICacheSerializerCreator] in the
// [cache.SerializerCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a serializer of id
// [Id] with this driver, and setting it as the default serializer of the
// adaptors, at [cache.DefaultSerializerConfigPath].
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	if err := bag.Set(cache.SerializerConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	}); err != nil {
		return err
	}
	return bag.Set(cache.DefaultSerializerConfigPath, Id)
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(cache.SerializerCreatorGroup)).
		Register()
}
