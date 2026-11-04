package redis

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the redis broker driver,
// providing its [flam.IBrokerCreator] in the [broker.BrokerCreatorGroup]
// group. It is also a [flam.IConfigurableBundle], setting the defaults of
// the broker fields. It configures no broker. Its brokers take their
// connection from the [flam.IRedisConnectionFactory] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/redis], to be registered before
// the broker bundle.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	for path, value := range map[string]any{
		DefaultConnectionConfigPath: defaultConnection,
		DefaultModeConfigPath:       defaultMode,
		DefaultStartConfigPath:      defaultStart,
		DefaultBatchConfigPath:      defaultBatch,
		DefaultBlockConfigPath:      defaultBlock.String(),
		DefaultClaimIdleConfigPath:  defaultClaimIdle.String(),
		DefaultMaxLenConfigPath:     defaultMaxLen,
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(broker.BrokerCreatorGroup)).
		Register()
}
