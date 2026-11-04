package memory

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.lock.lockers.memory"

// Driver is the driver name of the memory lockers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration, which holds the fields shared by the locker
// drivers, documented in [github.com/cjdias/flam-in-go/pkg/flam/lock].
const Driver = "flam.lock.lockers.driver.memory"

// Context fields of the [*flam.Error] values returned by the creator,
// along with those of [github.com/cjdias/flam-in-go/pkg/flam/lock].
const (
	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)
