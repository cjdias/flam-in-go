package translator

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the translator feature,
// providing to the application container a single
// [flam.ITranslatorFactory], created on first use from the configuration
// at [ConfigPath] and the creators of the [CreatorGroup] group. The
// configuration is read once, when the factory is created. It provides no
// translator driver: register it with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/translator/translators].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the translator factory. As the factory is taken
// from the container, closing an application that never used it creates
// it first, and fails when it cannot be created.
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
		QueueProvider(newFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closeFactory)
}

func (b *bundleImpl) closeFactory(
	f flam.ITranslatorFactory,
) error {
	return f.Close()
}
