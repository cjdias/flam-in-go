package secrets

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the secrets feature, providing to
// the application container a single [flam.ISecretStoreFactory], created on
// first use from the configuration at [StoreConfigPath] and the creators of
// the [StoreCreatorGroup] group. The configuration is read once, when the
// factory is created. It provides no driver: register it with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets/stores], or with the
// driver bundles of its sub packages.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the store factory, and so its stores, returning its
// error. As the factory is taken from the container, closing an
// application that never used it creates it first, and fails when it
// cannot be created.
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
		QueueProvider(newStoreFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeStoreFactory)
}

func (b *bundleImpl) closeStoreFactory(
	f flam.ISecretStoreFactory,
) error {
	return f.Close()
}
