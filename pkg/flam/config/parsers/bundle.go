package parsers

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// configuration parser driver, [json.NewBundle] and [yaml.NewBundle], to be
// registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/config.NewBundle], which provides
// the [flam.IConfigParserFactory]. Do not register any of the driver
// bundles along with this one, as a bundle id can only be registered once.
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
		QueueBundle(json.NewBundle()).
		QueueBundle(yaml.NewBundle()).
		Register()
}
