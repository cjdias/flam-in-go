package mongo

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the mongo feature, providing to
// the application container a single [flam.IMongoConnectionFactory],
// created on first use from the configuration at [ConnectionConfigPath]
// and the creators of the [ConnectionCreatorGroup] group. The
// configuration is read once, when the factory is created. It provides no
// connection driver: register it with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/mongo/connections].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the connection factory, and so disconnects its
// connections. As the factory is taken from the container, closing an
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
		QueueProvider(newConnectionFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeConnectionFactory)
}

func (b *bundleImpl) closeConnectionFactory(
	f flam.IMongoConnectionFactory,
) error {
	return f.Close()
}
