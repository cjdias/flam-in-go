package standard

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.validator.parsers.standard"

const (
	// Id is the id of the default parser configured by the bundle returned
	// from [NewBundle].
	Id = "standard"

	// Driver is the driver name of the standard parsers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.validator.parsers.driver.standard"
)

// Fields of a standard parser configuration.
const (
	// ConfigFieldAnnotation is the field holding the name of the struct
	// tag of the ids of the fields, such as `paramId:"3"`, "paramId"
	// unless set.
	ConfigFieldAnnotation = "annotation"

	// ConfigFieldTranslator is the field holding the id of the translator
	// of the messages, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.ITranslatorFactory], an empty
	// id leaving them untranslated.
	ConfigFieldTranslator = "translator"
)

// Configuration paths of the values taken by the fields missing from a
// parser configuration.
const (
	// DefaultTranslatorConfigPath is the path of the default of
	// [ConfigFieldTranslator], unset unless configured.
	DefaultTranslatorConfigPath = "flam.validator.defaults.parsers.standard.translator"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the parsers.
const (
	// ErrorFieldID holds the id of the parser.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldTranslator holds the id of the translator of the parser.
	ErrorFieldTranslator = "translator"

	// ErrorFieldLocale holds the locale of the translator of the parser.
	ErrorFieldLocale = "locale"
)

// Defaults of the fields of a parser configuration.
const (
	defaultAnnotation = "paramId"
)
