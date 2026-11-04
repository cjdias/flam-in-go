package flam

import (
	"github.com/spf13/afero"
)

// IDisk is a file system, as an [afero.Fs], held by an [IDiskFactory]. As
// an [IResource], its Id method returns the id of the disk in the factory,
// while the Name method of the afero.Fs names the kind of file system,
// such as "MemMapFS".
type IDisk interface {
	afero.Fs
	IResource
}

// IDiskCreator creates the disks of a driver, for the [IDiskFactory]. The
// driver creators provided by the disk drivers accept the disk
// configurations whose
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// holds the name of their driver.
type IDiskCreator = IResourceCreator[IDisk]

// IDiskFactory creates and holds the disks described by the configuration
// at [github.com/cjdias/flam-in-go/pkg/flam/disk.DiskConfigPath], one disk
// configuration per disk id, with the creators of the disk drivers.
type IDiskFactory = IFactory[IDisk]
