package redis

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
)

// NewBundle returns the [flam.IBundle] of the redis locker driver,
// providing its [flam.ILockerCreator] in the [lock.LockerCreatorGroup]
// group. Its lockers take their connections from the
// [flam.IRedisConnectionFactory] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/redis], and read the time from
// the [flam.IClock] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/clock], to register along with
// them. It configures no locker.
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
		QueueProvider(newCreator, dig.Group(lock.LockerCreatorGroup)).
		Register()
}
