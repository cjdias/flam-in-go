package standard

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.validator.validators.standard"

const (
	// Id is the id of the default validator configured by the bundle
	// returned from [NewBundle].
	Id = "standard"

	// Driver is the driver name of the standard validators, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.validator.validators.driver.standard"
)

// Context fields of the [*flam.Error] values returned by the creator and
// [NewValidator], along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/validator].
const (
	// ErrorFieldID holds the id of the validator.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldParser holds the id of the parser of the validator.
	ErrorFieldParser = "parser"

	// ErrorFieldConverter holds the id of the converter of the validator.
	ErrorFieldConverter = "converter"
)
