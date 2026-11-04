package consul

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
)

// NewBundle returns the [flam.IBundle] of the consul source driver,
// providing its [flam.IConfigSourceCreator] in the
// [config.SourceCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], setting the defaults of the source fields at
// their default paths, such as [DefaultAddressConfigPath]. It configures no
// source, as a source needs a key.
//
// The creator takes the [flam.IConfigParserFactory] from the container,
// when provided: register the configuration bundle to create consul
// sources.
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
		DefaultPriorityConfigPath:   defaultPriority,
		DefaultReloadConfigPath:     defaultReload,
		DefaultAddressConfigPath:    defaultAddress,
		DefaultTokenConfigPath:      defaultToken,
		DefaultDatacenterConfigPath: defaultDatacenter,
		DefaultParserConfigPath:     defaultParser,
		DefaultTimeoutConfigPath:    defaultTimeout.String(),
		DefaultCAFileConfigPath:     defaultCAFile,
		DefaultCertFileConfigPath:   defaultCertFile,
		DefaultKeyFileConfigPath:    defaultKeyFile,
		DefaultSkipVerifyConfigPath: defaultSkipVerify,
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
		QueueProvider(newCreator, dig.Group(config.SourceCreatorGroup)).
		Register()
}
