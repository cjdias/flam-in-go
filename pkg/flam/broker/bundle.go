package broker

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the broker feature, providing to
// the application container a single [flam.IBrokerFactory], created on
// first use from the configuration at [BrokerConfigPath] and the creators
// of the [BrokerCreatorGroup] group. The configuration is read once, when
// the factory is created. It provides no broker driver: register them
// with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/broker/brokers].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the broker factory, and so its brokers and their
// subscriptions. As the factory is taken from the container, closing an
// application that never used it creates it first, and fails when it
// cannot be created. As the bundles close in the reverse order of their
// registration, register the bundles of what the brokers use, such as the
// redis bundle, before this one.
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
		QueueProvider(newBrokerFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeBrokerFactory)
}

func (b *bundleImpl) closeBrokerFactory(
	f flam.IBrokerFactory,
) error {
	return f.Close()
}
