package local

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDiskCreator] of the local disks. It is the
// provider registered by the bundle returned from [NewBundle].
func newCreator() flam.IDiskCreator {
	return &creator{}
}

// creator is the [flam.IDiskCreator] of the configurations with the
// [Driver] driver, restricting each disk to the directory of its
// [ConfigFieldRoot] field, if any.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.IDisk, error) {
	return newDisk(
		bag.String(factory.ConfigFieldId),
		bag.String(ConfigFieldRoot),
	), nil
}
