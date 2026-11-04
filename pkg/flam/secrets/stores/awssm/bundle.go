package awssm

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// NewBundle returns the [flam.IBundle] of the awssm store driver, providing
// its [flam.ISecretStoreCreator] in the [secrets.StoreCreatorGroup] group.
// It is also a [flam.IConfigurableBundle], setting the defaults of the
// store fields at their default paths, such as [DefaultRegionConfigPath].
// It configures no store: an application configures the stores of its
// secrets.
//
// The creator takes the [flam.IClock] of the container, provided by the
// bundle of [github.com/cjdias/flam-in-go/pkg/flam/clock], for the cache of
// the stores.
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
		DefaultRegionConfigPath:          defaultRegion,
		DefaultEndpointConfigPath:        defaultEndpoint,
		DefaultProfileConfigPath:         defaultProfile,
		DefaultAccessKeyIDConfigPath:     defaultAccessKeyID,
		DefaultSecretAccessKeyConfigPath: defaultSecretAccessKey,
		DefaultSessionTokenConfigPath:    defaultSessionToken,
		DefaultVersionStageConfigPath:    defaultVersionStage,
		DefaultTimeoutConfigPath:         defaultTimeout.String(),
		DefaultMaxAttemptsConfigPath:     defaultMaxAttempts,
		DefaultCacheTTLConfigPath:        defaultCacheTTL.String(),
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
		QueueProvider(newCreator, dig.Group(secrets.StoreCreatorGroup)).
		Register()
}
