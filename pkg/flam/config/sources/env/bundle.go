package env

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// NewBundle returns the [flam.IBundle] of the env source driver, providing
// its [flam.IConfigSourceCreator] in the [config.SourceCreatorGroup] group.
// It is also a [flam.IConfigurableBundle], setting the defaults of the
// source fields at [DefaultPriorityConfigPath], [DefaultReloadConfigPath],
// [DefaultFilesConfigPath] and [DefaultMappingsConfigPath]. It configures
// no source: an env source is useless without mappings.
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
		DefaultPriorityConfigPath: defaultPriority,
		DefaultReloadConfigPath:   defaultReload,
		DefaultFilesConfigPath:    []string{},
		DefaultMappingsConfigPath: container.Bag{},
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
