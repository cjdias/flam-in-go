package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
)

// NewBundle returns the [flam.IBundle] of the standard parser driver,
// providing its [flam.IValidationParserCreator] in the
// [validator.ParserCreatorGroup] group. Its parsers take their
// translators from the [flam.ITranslatorFactory] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/translator], to register along
// with it when translating the messages. It is also a
// [flam.IConfigurableBundle], configuring by default a parser of id [Id]
// with this driver and the default fields, and setting it as the default
// parser of the validators, at [validator.DefaultParserConfigPath].
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
	if err := bag.Set(validator.ParserConfigPath+"."+Id, container.Bag{
		factory.ConfigFieldDriver: Driver,
	}); err != nil {
		return err
	}
	return bag.Set(validator.DefaultParserConfigPath, Id)
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(validator.ParserCreatorGroup)).
		Register()
}
