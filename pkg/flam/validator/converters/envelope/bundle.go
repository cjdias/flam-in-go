package envelope

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
)

// NewBundle returns the [flam.IBundle] of the envelope converter driver,
// providing its [flam.IValidationErrorConverterCreator] in the
// [validator.ConverterCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default a converter of id
// [Id] with this driver, and setting the default of its status, at
// [DefaultStatusConfigPath]. It does not make it the default converter
// of the validators: set [validator.DefaultConverterConfigPath] to [Id],
// or the converter field of a validator, to use it.
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
	if err := bag.Set(validator.ConverterConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	}); err != nil {
		return err
	}
	return bag.Set(DefaultStatusConfigPath, defaultStatus)
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(validator.ConverterCreatorGroup)).
		Register()
}
