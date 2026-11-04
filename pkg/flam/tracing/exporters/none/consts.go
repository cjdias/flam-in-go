package none

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.tracing.exporters.none"

const (
	// Id is the id of the exporter configured by the bundle returned from
	// [NewBundle].
	Id = "none"

	// Driver is the driver name of the none exporters, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.tracing.exporters.driver.none"
)
