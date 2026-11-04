package memory

import (
	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newDisk returns a new, empty in-memory disk of the given id.
func newDisk(
	id string,
) flam.IDisk {
	return &diskImpl{
		Fs: afero.NewMemMapFs(),
		id: id,
	}
}

// diskImpl is the in-memory [flam.IDisk].
type diskImpl struct {
	afero.Fs

	id string
}

func (d *diskImpl) Id() string {
	return d.id
}
