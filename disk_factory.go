package flam

import (
	"io"

	"go.uber.org/dig"
)

type DiskFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (Disk, error)
	Store(id string, disk Disk) error
	Remove(id string) error
	RemoveAll() error
}

type diskFactoryArgs struct {
	dig.In

	Creators      []DiskCreator `group:"flam.disks.creator"`
	FactoryConfig FactoryConfig
}

func newDiskFactory(
	args diskFactoryArgs,
) (DiskFactory, error) {
	var creators []FactoryResourceCreator[Disk]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("Disk"),
		PathDisks)
}
