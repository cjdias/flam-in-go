package pubsub

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the pubsub feature,
// providing to the application container a new [flam.IPubSub] with string
// ids and string messages.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the PubSub, returning the error of [PubSub.Close],
// such as the one refusing a Close called from a handler of the PubSub,
// which leaves it open for a later Close. As the PubSub is taken from the
// container, closing an application that never used it creates it first;
// once closed, the container keeps providing the closed PubSub.
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
		QueueProvider(b.providePubSub).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return app.Invoke(b.closePubSub)
}

func (b *bundleImpl) providePubSub() flam.IPubSub[string, string] {
	return New[string, string]()
}

func (b *bundleImpl) closePubSub(
	ps flam.IPubSub[string, string],
) error {
	return ps.Close()
}
