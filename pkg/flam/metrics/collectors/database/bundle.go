package database

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
)

// NewBundle returns the [flam.IBundle] providing, in the
// [metrics.CollectorGroup] group, the collectors of the statistics of the
// pools of the database connections listed at [ConnectionsConfigPath], or
// else of every connection configured in the
// [flam.IDatabaseConnectionFactory] of the container, none when the
// container provides no factory. The connections are taken from the
// factory when the registry is created, which fails, with an error
// wrapping [ErrInvalidConnections], [ErrMissingFactory] or
// [ErrConnection], when they cannot be.
//
// Register it along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics] and the database
// feature.
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
		QueueProvider(newCollectors, dig.Group(metrics.CollectorGroup+",flatten")).
		Register()
}
