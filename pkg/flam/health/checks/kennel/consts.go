package kennel

import (
	"github.com/cjdias/flam-in-go/pkg/flam/health"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.checks.kennel"

// Driver is the driver name of the kennel checks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.health.checks.driver.kennel"

// Fields of a kennel check configuration, along with the fields shared by
// the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/health].
const (
	// ConfigFieldProcesses is the field holding the list of the ids of the
	// processes the check watches, every process of the kennel unless
	// set.
	ConfigFieldProcesses = "processes"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the checks.
const (
	// ErrorFieldID holds the id of the check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldProcess holds the id of an unknown process.
	ErrorFieldProcess = "process"

	// ErrorFieldProcesses holds the ids of the active processes not
	// running, a []string.
	ErrorFieldProcesses = "processes"
)

// Defaults of the fields of a check configuration.
const (
	defaultKind = health.KindReadiness
)
