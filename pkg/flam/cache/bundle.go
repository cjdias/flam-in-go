package cache

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the cache feature, providing to
// the application container a single [flam.ICacheSerializerFactory],
// [flam.ICacheKeyGeneratorFactory] and [flam.ICacheAdaptorFactory],
// created on first use from the configuration at [SerializerConfigPath],
// [KeyGeneratorConfigPath] and [AdaptorConfigPath] and the creators of the
// [SerializerCreatorGroup], [KeyGeneratorCreatorGroup] and
// [AdaptorCreatorGroup] groups. The configuration is read once, when a
// factory is created. It provides no driver: register it with the bundles
// of [github.com/cjdias/flam-in-go/pkg/flam/cache/serializers],
// [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators] and
// [github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the adaptor factory, then the key generator factory,
// then the serializer factory, even when one fails, returning their errors
// joined. As the factories are taken from the container, closing an
// application that never used them creates them first, and fails when
// they cannot be created.
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
		QueueProvider(newSerializerFactory).
		QueueProvider(newKeyGeneratorFactory).
		QueueProvider(newAdaptorFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeAdaptorFactory),
		app.Invoke(b.closeKeyGeneratorFactory),
		app.Invoke(b.closeSerializerFactory),
	)
}

func (b *bundleImpl) closeAdaptorFactory(
	f flam.ICacheAdaptorFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeKeyGeneratorFactory(
	f flam.ICacheKeyGeneratorFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeSerializerFactory(
	f flam.ICacheSerializerFactory,
) error {
	return f.Close()
}
