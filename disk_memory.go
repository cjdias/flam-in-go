package flam

import (
	"github.com/spf13/afero"
)

type memoryDiskCreator struct{}

var _ DiskCreator = (*memoryDiskCreator)(nil)

func newMemoryDiskCreator() DiskCreator {
	return &memoryDiskCreator{}
}

func (memoryDiskCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DiskDriverMemory
}

func (memoryDiskCreator) Create(
	_ Bag,
) (Disk, error) {
	return afero.NewMemMapFs(), nil
}
