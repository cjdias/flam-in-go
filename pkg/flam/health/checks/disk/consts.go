package disk

import (
	"github.com/cjdias/flam-in-go/pkg/flam/health"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.checks.disk"

// Driver is the driver name of the disk checks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.health.checks.driver.disk"

// Fields of a disk check configuration, along with the fields shared by
// the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/health].
const (
	// ConfigFieldDisk is the field holding the id of the disk the check
	// checks, in the disk factory, the check id unless set.
	ConfigFieldDisk = "disk"

	// ConfigFieldPath is the field holding the path of the disk the check
	// stats, the root "/" unless set.
	ConfigFieldPath = "path"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the checks.
const (
	// ErrorFieldID holds the id of the check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldDisk holds the id of the disk of the check.
	ErrorFieldDisk = "disk"

	// ErrorFieldPath holds the path the check stats.
	ErrorFieldPath = "path"
)

// Defaults of the fields of a check configuration.
const (
	defaultKind = health.KindReadiness
	defaultPath = "/"
)
