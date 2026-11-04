package json

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.parsers.json"

const (
	// Id is the id of the default parser configured by the bundle returned
	// from [NewBundle].
	Id = "json"

	// Driver is the driver name of the JSON parsers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.config.parsers.driver.json"
)

// Context fields of the [*flam.Error] values returned by the parsers.
const (
	// ErrorFieldID holds the id of the parser.
	ErrorFieldID = "id"
)
