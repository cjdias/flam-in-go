package memory

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
)

// NewBundle returns the [flam.IBundle] of the memory adaptor driver,
// providing its [flam.ICacheAdaptorCreator] in the
// [cache.AdaptorCreatorGroup] group. Its adaptors read the time from the
// [flam.IClock] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/clock], to register along with
// it. It configures no adaptor.
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
		QueueProvider(newCreator, dig.Group(cache.AdaptorCreatorGroup)).
		Register()
}
