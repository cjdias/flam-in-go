package trigger

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the trigger feature, providing to
// the application container a single factory, both as a
// [flam.ITriggerFactory] and as a [flam.ICronTriggerFactory], reading the
// time from the [flam.IClock] of the container: register the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/clock] too, or provide another
// IClock.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the factory, and so its running triggers. As the
// factory is taken from the container, closing an application that never
// used it creates it first, and fails when it cannot be created, such as
// without an IClock.
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
	f flam.ITriggerFactory,
) error {
	return f.Close() // always nil
}
