package local

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.disk.disks.local"

const (
	// Id is the id of the default disk configured by the bundle returned
	// from [NewBundle].
	Id = "local"

	// Driver is the driver name of the local disks, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.disk.disks.driver.local"
)

// ConfigFieldRoot is the optional field of a local disk configuration
// holding the directory the disk is restricted to: its paths are relative
// to that directory, and paths escaping it fail with
// [os.ErrNotExist]. Without it, or when empty, the disk is the whole file
// system of the operating system.
const ConfigFieldRoot = "root"
