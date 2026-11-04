package disk

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// diskFactoryArgs are the dependencies of the disk factory, taken from the
// container. The group tag repeats DiskCreatorGroup, as a struct tag cannot
// use a constant.
type diskFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IDiskCreator `group:"flam.disk.disks.creator"`
}

// newDiskFactory returns the disk factory creating the disks configured at
// DiskConfigPath with the creators of the DiskCreatorGroup group.
func newDiskFactory(
	args diskFactoryArgs,
) (flam.IDiskFactory, error) {
	return factory.New(args.Config.Bag(DiskConfigPath), args.Creators...)
}
