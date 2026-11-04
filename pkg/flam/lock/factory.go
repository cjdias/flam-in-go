package lock

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// lockerFactoryArgs are the dependencies of the locker factory, taken
// from the container. The group tag repeats LockerCreatorGroup, as a
// struct tag cannot use a constant.
type lockerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ILockerCreator `group:"flam.lock.lockers.creator"`
}

// newLockerFactory returns the locker factory creating the lockers
// configured at LockerConfigPath with the creators of the
// LockerCreatorGroup group.
func newLockerFactory(
	args lockerFactoryArgs,
) (flam.ILockerFactory, error) {
	return factory.New(args.Config.Bag(LockerConfigPath), args.Creators...)
}
