package flam

import (
	"github.com/spf13/afero"
)

type osDiskCreator struct{}

var _ DiskCreator = (*osDiskCreator)(nil)

func newOsDiskCreator() DiskCreator {
	return &osDiskCreator{}
}

func (osDiskCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == DiskDriverOS
}

func (osDiskCreator) Create(
	_ Bag,
) (Disk, error) {
	return afero.NewOsFs(), nil
}
