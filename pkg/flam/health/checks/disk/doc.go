// Package disk is the health check driver of the disks: each check
// stats a path of a disk of the
// [github.com/cjdias/flam-in-go/pkg/flam.IDiskFactory], taking it from
// the factory, and so creating it, on its first run, such as:
//
//	flam:
//	  health:
//	    checks:
//	      uploads: {driver: flam.health.checks.driver.disk}
//	      reports: {driver: flam.health.checks.driver.disk, disk: s3, path: /reports}
//
// The check stats the root "/" of the disk unless its path field is set,
// failing when the path does not exist, or when the backend, such as a
// bucket, does not answer. As a disk takes no context, a stat outlasting
// the timeout of the probe is left to end on its own.
//
// The check counts for the readiness probe unless its kind is set, as a
// disk that cannot be reached makes the application unable to serve,
// rather than broken.
package disk
