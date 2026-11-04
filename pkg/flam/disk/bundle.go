package disk

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the disk feature, providing to
// the application container a single [flam.IDiskFactory], created on first
// use from the configuration at [DiskConfigPath] and the creators of the
// [DiskCreatorGroup] group. The configuration is read once, when the
// factory is created.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the factory, and so its disks, returning the error of
// [flam.IFactory.Close]. As the factory is taken from the container,
// closing an application that never used it creates it first, and fails
// when it cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IClosableBundle
var _ flam.IClosableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newDiskFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeDiskFactory)
}

func (b *bundleImpl) closeDiskFactory(
	f flam.IDiskFactory,
) error {
	return f.Close()
}
