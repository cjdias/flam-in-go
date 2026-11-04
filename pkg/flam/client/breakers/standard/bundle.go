package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the standard circuit breaker
// driver, providing its [flam.IClientCircuitBreakerCreator] in the
// [client.BreakerCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a circuit breaker of
// id [Id] with this driver, and setting the defaults of the breaker
// fields.
//
// The creator takes the [flam.IClock] from the container: register the
// clock bundle to create standard circuit breakers.
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
		client.BreakerConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultFailureThresholdConfigPath:   defaultFailureThreshold,
		DefaultOpenTimeoutConfigPath:        defaultOpenTimeout.String(),
		DefaultHalfOpenRequestsConfigPath:   defaultHalfOpenRequests,
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
		QueueProvider(newCreator, dig.Group(client.BreakerCreatorGroup)).
		Register()
}
