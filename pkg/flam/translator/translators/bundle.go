package translators

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/english"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/french"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/german"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/portuguese"
	"github.com/cjdias/flam-in-go/pkg/flam/translator/translators/spanish"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// translator driver, [english.NewBundle], [portuguese.NewBundle], [spanish.NewBundle], [french.NewBundle], [german.NewBundle], to be
// registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/translator.NewBundle], which
// provides the [flam.ITranslatorFactory]. Do not register any of the
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
		QueueBundle(english.NewBundle()).
		QueueBundle(portuguese.NewBundle()).
		QueueBundle(spanish.NewBundle()).
		QueueBundle(french.NewBundle()).
		QueueBundle(german.NewBundle()).
		Register()
}
