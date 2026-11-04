package field

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the field key generator driver,
// providing its [flam.ICacheKeyGeneratorCreator] in the
// [cache.KeyGeneratorCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a key generator of
// id [Id] with this driver and the default fields, and setting it as the
// default key generator of the adaptors, at
// [cache.DefaultKeyGeneratorConfigPath].
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
	if err := bag.Set(cache.KeyGeneratorConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	}); err != nil {
		return err
	}
	return bag.Set(cache.DefaultKeyGeneratorConfigPath, Id)
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(cache.KeyGeneratorCreatorGroup)).
		Register()
}
