package sources

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/consul"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/dir"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/env"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/etcd"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/file"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/rest"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// configuration source driver, [env.NewBundle], [file.NewBundle],
// [dir.NewBundle], [rest.NewBundle], [consul.NewBundle] and
// [etcd.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/config.NewBundle], which provides
// the [flam.IConfigSourceFactory]. Do not register any of the driver
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
		QueueBundle(env.NewBundle()).
		QueueBundle(file.NewBundle()).
		QueueBundle(dir.NewBundle()).
		QueueBundle(rest.NewBundle()).
		QueueBundle(consul.NewBundle()).
		QueueBundle(etcd.NewBundle()).
		Register()
}
