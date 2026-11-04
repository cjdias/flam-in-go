package memory

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDiskCreator] of the in-memory disks. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator() flam.IDiskCreator {
	return &creator{}
}

// creator is the [flam.IDiskCreator] of the configurations with the
// [Driver] driver. Each disk it creates is a new, empty file system, so
// that the disks returned by the Generate method of the factory share no
// content with each other nor with the one returned by Get.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.IDisk, error) {
	return newDisk(bag.String(factory.ConfigFieldId)), nil
}
