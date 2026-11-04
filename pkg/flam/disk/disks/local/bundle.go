package local

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the local disk driver, providing
// its [flam.IDiskCreator] in the [disk.DiskCreatorGroup] group. It is also
// a [flam.IConfigurableBundle], configuring by default a disk of id [Id]
// with this driver and no [ConfigFieldRoot], giving access to the whole
// file system of the operating system.
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
	return bag.Set(disk.DiskConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	})
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(disk.DiskCreatorGroup)).
		Register()
}
