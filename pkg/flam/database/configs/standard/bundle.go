package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// NewBundle returns the [flam.IBundle] of the standard configuration
// driver, providing its [flam.IDatabaseConfigCreator] in the
// [database.ConfigCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a configuration of id
// [Id] with this driver, its logger discarding, and making it the default
// configuration of the connections, at [database.DefaultConfigConfigPath].
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
		database.ConfigConfigPath + "." + Id: container.Bag{
			factory.ConfigFieldDriver: Driver,
			ConfigFieldLogger:         container.Bag{"type": LoggerDiscard},
		},
		database.DefaultConfigConfigPath: Id,
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
		QueueProvider(newCreator, dig.Group(database.ConfigCreatorGroup)).
		Register()
}
