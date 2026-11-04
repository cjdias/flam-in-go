// Package disk provides the [flam.IDiskFactory] of an application: the
// bundle returned by [NewBundle] provides a factory creating the disks
// described by the configuration at [DiskConfigPath], with the
// [flam.IDiskCreator] values the disk driver bundles provide in the
// [DiskCreatorGroup] container group.
//
// This bundle provides no driver: register it with the driver bundles of
// the [github.com/cjdias/flam-in-go/pkg/flam/disk/disks] sub packages, or
// with the bundle of that package, which registers every driver.
package disk
