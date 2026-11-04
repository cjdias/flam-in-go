package validator

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
)

// NewBundle returns the [flam.IBundle] of the validator feature, providing
// to the application container a single [flam.IValidationParserFactory],
// [flam.IValidationErrorConverterFactory] and [flam.IValidatorFactory],
// created on first use from the configuration at [ParserConfigPath],
// [ConverterConfigPath] and [ValidatorConfigPath] and the creators of the
// [ParserCreatorGroup], [ConverterCreatorGroup] and [ValidatorCreatorGroup]
// groups. The configuration is read once, when a factory is created. It
// provides no driver: register it with the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/validator/parsers] and
// [github.com/cjdias/flam-in-go/pkg/flam/validator/validators], and the
// bundles providing the converter creators, if any.
//
// The bundle also implements [flam.IClosableBundle]: closing the
// application closes the validator factory, then the converter factory,
// then the parser factory, even when one fails, returning their errors
// joined. As the factories are taken from the container, closing an
// application that never used them creates them first, and fails when
// they cannot be created.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IClosableBundle
var _ flam.IClosableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newParserFactory).
		QueueProvider(newConverterFactory).
		QueueProvider(newValidatorFactory).
		Register()
}

func (b *bundleImpl) Close(
	app flam.IApp,
) error {
	return errors.Join(
		app.Invoke(b.closeValidatorFactory),
		app.Invoke(b.closeConverterFactory),
		app.Invoke(b.closeParserFactory),
	)
}

func (b *bundleImpl) closeValidatorFactory(
	f flam.IValidatorFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeConverterFactory(
	f flam.IValidationErrorConverterFactory,
) error {
	return f.Close()
}

func (b *bundleImpl) closeParserFactory(
	f flam.IValidationParserFactory,
) error {
	return f.Close()
}
