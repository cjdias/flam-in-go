package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the standard client driver,
// providing its [flam.IClientCreator] in the [client.ClientCreatorGroup]
// group. It is also a [flam.IConfigurableBundle], configuring by default a
// client of id [Id] with this driver, and setting the defaults of the
// client fields.
//
// The creator takes the [flam.IClock] from the container: register the
// clock bundle to create standard clients.
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
		client.ClientConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultTimeoutConfigPath:           defaultTimeout.String(),
		DefaultRetriesConfigPath:           defaultRetries,
		DefaultRetryDelayConfigPath:        defaultRetryDelay.String(),
		DefaultRetryMaxDelayConfigPath:     defaultRetryMaxDelay.String(),
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
		QueueProvider(newCreator, dig.Group(client.ClientCreatorGroup)).
		Register()
}
