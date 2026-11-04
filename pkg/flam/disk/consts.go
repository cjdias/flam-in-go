package disk

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.disk"

const (
	// DiskConfigPath is the configuration path holding one disk
	// configuration per disk id, as read by the disk factory.
	DiskConfigPath = "flam.disk.disks"

	// DiskCreatorGroup is the container group the disk driver bundles
	// provide their [flam.IDiskCreator] in, collected by the disk factory.
	DiskCreatorGroup = "flam.disk.disks.creator"
)
