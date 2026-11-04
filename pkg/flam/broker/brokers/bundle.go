package brokers

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/kafka"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/nats"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// broker driver, [redis.NewBundle], [nats.NewBundle] and
// [kafka.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/broker.NewBundle], which
// provides the [flam.IBrokerFactory]. Do not register any of the driver
// bundles along with this one, as a bundle id can only be registered
// once.
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
		QueueBundle(redis.NewBundle()).
		QueueBundle(nats.NewBundle()).
		QueueBundle(kafka.NewBundle()).
		Register()
}
