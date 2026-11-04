package memory

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The clock is optional, so that the locker factory can be
// created without the clock bundle, failing to create the memory lockers
// only.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Clock  flam.IClock `optional:"true"`
}

// newCreator returns the [flam.ILockerCreator] of the memory lockers,
// reading the defaults of the missing fields from the configuration. It
// is the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILockerCreator {
	return &creator{config: args.Config, clock: args.Clock}
}

// creator is the [flam.ILockerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
	clock  flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the locker of the shared fields, failing at the first
// invalid one, over a store of its own.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ILocker, error) {
	id := bag.String(factory.ConfigFieldId)
	options, field := base.ReadOptions(c.config, bag)
	if field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}
	if c.clock == nil {
		return nil, base.NewErr(ErrClock, creatorCreateMessage, id, nil)
	}
	return base.NewLocker(id, newStore(c.clock), c.clock, options), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create memory locker"
