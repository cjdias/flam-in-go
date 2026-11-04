package nats

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the NATS broker driver,
// providing its [flam.IBrokerCreator] in the [broker.BrokerCreatorGroup]
// group. It is also a [flam.IConfigurableBundle], setting the defaults of
// the broker fields. It configures no broker.
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
		DefaultURLConfigPath:            defaultURL,
		DefaultConnectTimeoutConfigPath: defaultConnectTimeout.String(),
		DefaultReconnectWaitConfigPath:  defaultReconnectWait.String(),
		DefaultMaxReconnectsConfigPath:  defaultMaxReconnects,
		DefaultJetStreamConfigPath:      defaultJetStream,
		DefaultAckWaitConfigPath:        defaultAckWait.String(),
		DefaultMaxDeliverConfigPath:     defaultMaxDeliver,
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
