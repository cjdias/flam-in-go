package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// NewBundle returns the [flam.IBundle] of the standard server driver,
// providing its [flam.IRestServerCreator] in the
// [rest.ServerCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the server fields.
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
		DefaultAddressConfigPath:           defaultAddress,
		DefaultReadHeaderTimeoutConfigPath: defaultReadHeaderTimeout.String(),
		DefaultReadTimeoutConfigPath:       defaultReadTimeout.String(),
		DefaultWriteTimeoutConfigPath:      defaultWriteTimeout.String(),
		DefaultIdleTimeoutConfigPath:       defaultIdleTimeout.String(),
		DefaultMaxHeaderBytesConfigPath:    defaultMaxHeaderBytes,
		DefaultShutdownTimeoutConfigPath:   defaultShutdownTimeout.String(),
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
		QueueProvider(newCreator, dig.Group(rest.ServerCreatorGroup)).
		Register()
}
