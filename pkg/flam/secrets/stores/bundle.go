package stores

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/env"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/file"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/vault"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// secret store driver, [env.NewBundle], [file.NewBundle] and
// [vault.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets.NewBundle], which provides
// the [flam.ISecretStoreFactory]. Do not register any of the driver
// bundles along with this one, as a bundle id can only be registered once.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBundle
var _ flam.IBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueBundle(env.NewBundle()).
		QueueBundle(file.NewBundle()).
		QueueBundle(vault.NewBundle()).
		Register()
}
