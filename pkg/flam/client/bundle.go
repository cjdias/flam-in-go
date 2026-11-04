package client

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the client feature, providing to
// the application container:
//   - a single [flam.IClientFactory], creating the clients configured at
//     [ClientConfigPath] with the creators of the [ClientCreatorGroup]
//     group;
//   - a single [flam.IClientCircuitBreakerFactory], creating the circuit
//     breakers configured at [BreakerConfigPath] with the creators of the
//     [BreakerCreatorGroup] group.
//
// The factories are created on first use, reading the configuration once.
// It provides no driver: register them with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/client/clients] and
// [github.com/cjdias/flam-in-go/pkg/flam/client/breakers].
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the client factory, and so its clients, then the
// circuit breaker factory, even when one fails, returning their errors
// joined. As they are taken from the container, closing an application
// that never used them creates them first, and fails when they cannot be
// created.
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
		QueueProvider(newBreakerFactory).
		QueueProvider(newClientFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeClientFactory),
		app.Invoke(b.closeBreakerFactory),
	)
}

func (b *bundleImpl) closeClientFactory(
	f flam.IClientFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeBreakerFactory(
	f flam.IClientCircuitBreakerFactory,
) error {
	return f.Close()
}
