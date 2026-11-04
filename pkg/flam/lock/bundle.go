package lock

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the lock feature, providing to
// the application container a single [flam.ILockerFactory], created on
// first use from the configuration at [LockerConfigPath] and the creators
// of the [LockerCreatorGroup] group. The configuration is read once, when
// the factory is created. It provides no locker driver: register them
// with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/lock/lockers].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the locker factory, and so its lockers, releasing
// the locks they still hold. As the factory is taken from the container,
// closing an application that never used it creates it first, and fails
// when it cannot be created. As the bundles close in the reverse order of
// their registration, register the bundles of what the lockers use, such
// as the redis bundle, before this one.
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
		QueueProvider(newLockerFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeLockerFactory)
}

func (b *bundleImpl) closeLockerFactory(
	f flam.ILockerFactory,
) error {
	return f.Close()
}
