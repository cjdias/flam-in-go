package env

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
)

// NewBundle returns the [flam.IBundle] of the env store driver, providing
// its [flam.ISecretStoreCreator] in the [secrets.StoreCreatorGroup] group.
// It is also a [flam.IConfigurableBundle], configuring by default a store
// of id [Id] with this driver, and setting the default of the store field
// at [DefaultPrefixConfigPath].
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
		secrets.StoreConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultPrefixConfigPath:            defaultPrefix,
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
