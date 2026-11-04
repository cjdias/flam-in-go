package local

import (
	"github.com/spf13/afero"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newDisk returns a local disk of the given id, restricted to the root
// directory unless it is empty.
func newDisk(
	id string,
	root string,
) flam.IDisk {
	fs := afero.NewOsFs()
	if root != "" {
		fs = afero.NewBasePathFs(fs, root)
	}
	return &diskImpl{
		Fs: fs,
		id: id,
	}
}

// diskImpl is the local [flam.IDisk].
type diskImpl struct {
	afero.Fs

	id string
}

func (d *diskImpl) Id() string {
	return d.id
}
