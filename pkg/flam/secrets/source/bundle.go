package source

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the secrets source driver,
// providing its [flam.IConfigSourceCreator] in the
// [config.SourceCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a source of id [Id]
// with this driver, empty until values are set at
// [DefaultValuesConfigPath], and setting the defaults of the source fields
// at [DefaultPriorityConfigPath], [DefaultReloadConfigPath],
// [DefaultStoreConfigPath] and [DefaultValuesConfigPath].
//
// The creator takes the [flam.ISecretStoreFactory] from the container when
// the first source holding values is created: register the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets] to read secrets.
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
		config.SourceConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultPriorityConfigPath:          defaultPriority,
		DefaultReloadConfigPath:            defaultReload,
		DefaultStoreConfigPath:             defaultStore,
		DefaultValuesConfigPath:            container.Bag{},
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
