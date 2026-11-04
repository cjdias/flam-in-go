package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
)

// NewBundle returns the [flam.IBundle] of the standard validator driver,
// providing its [flam.IValidatorCreator] in the
// [validator.ValidatorCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a validator of id
// [Id] with this driver, its parser and converter taking their defaults.
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
	return bag.Set(validator.ValidatorConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	})
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(validator.ValidatorCreatorGroup)).
		Register()
}
