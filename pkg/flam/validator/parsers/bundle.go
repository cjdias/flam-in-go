package parsers

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// validation parser driver, [standard.NewBundle], to be registered along
// with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/validator.NewBundle], which
// provides the [flam.IValidationParserFactory]. Do not register any of the
// driver bundles along with this one, as a bundle id can only be
// registered once.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IBundle
var _ flam.IBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueBundle(standard.NewBundle()).
		Register()
}
