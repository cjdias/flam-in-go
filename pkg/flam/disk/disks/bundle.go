package disks

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/gcs"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/s3"
)

// NewBundle returns the [flam.IBundle] registering the bundles of every
// disk driver, [memory.NewBundle], [local.NewBundle], [s3.NewBundle] and
// [gcs.NewBundle], to be registered along with the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/disk.NewBundle], which provides
// the [flam.IDiskFactory]. Do not register any of the driver bundles along
// with this one, as a bundle id can only be registered once.
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
		QueueBundle(memory.NewBundle()).
		QueueBundle(local.NewBundle()).
		QueueBundle(s3.NewBundle()).
		QueueBundle(gcs.NewBundle()).
		Register()
}
