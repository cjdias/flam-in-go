package checks

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/health/checks/database"
	"github.com/cjdias/flam-in-go/pkg/flam/health/checks/kennel"
	"github.com/cjdias/flam-in-go/pkg/flam/health/checks/redis"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// health check driver, [database.NewBundle], [redis.NewBundle] and
// [kennel.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/health.NewBundle], which
// provides the [flam.IHealthCheckFactory]. Do not register any of the
// driver bundles along with this one, as a bundle id can only be
// registered once. As the drivers take what they check from the container
// only when provided, they do not require the bundles of the features
// they check, a check of a feature not registered failing to be created.
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
		QueueBundle(database.NewBundle()).
		QueueBundle(redis.NewBundle()).
		QueueBundle(kennel.NewBundle()).
		Register()
}
