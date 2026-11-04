package memory

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.disk.disks.memory"

const (
	// Id is the id of the default disk configured by the bundle returned
	// from [NewBundle].
	Id = "memory"

	// Driver is the driver name of the in-memory disks, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.disk.disks.driver.memory"
)
