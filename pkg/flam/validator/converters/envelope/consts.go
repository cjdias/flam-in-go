package envelope

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.validator.converters.envelope"

const (
	// Id is the id of the default converter configured by the bundle
	// returned from [NewBundle].
	Id = "envelope"

	// Driver is the driver name of the envelope converters, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.validator.converters.driver.envelope"
)

// Fields of an envelope converter configuration.
const (
	// ConfigFieldStatus is the field holding the HTTP status of the errors
	// of the envelopes, an int in the 400 to 599 range.
	ConfigFieldStatus = "status"
)

// Configuration paths of the values taken by the fields missing from a
// converter configuration.
const (
	// DefaultStatusConfigPath is the path of the default of
	// [ConfigFieldStatus], 400 unless changed.
	DefaultStatusConfigPath = "flam.validator.defaults.converters.envelope.status"
)

// Context fields of the errors of the envelopes.
const (
	// ContextFieldParam holds the name of the param failing its
	// validation, such as "Address.Street".
	ContextFieldParam = "param"
)

// Context fields of the [*flam.Error] values returned by the creator and
// [NewConverter].
const (
	// ErrorFieldID holds the id of the converter.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldStatus holds an invalid status.
	ErrorFieldStatus = "status"
)

// Defaults of the fields of a converter configuration.
const (
	defaultStatus = 400
)
