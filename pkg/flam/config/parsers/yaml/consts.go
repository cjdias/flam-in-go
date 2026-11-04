package yaml

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.parsers.yaml"

const (
	// Id is the id of the default parser configured by the bundle returned
	// from [NewBundle].
	Id = "yaml"

	// Driver is the driver name of the YAML parsers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.config.parsers.driver.yaml"
)

// Context fields of the [*flam.Error] values returned by the parsers.
const (
	// ErrorFieldID holds the id of the parser.
	ErrorFieldID = "id"
)
