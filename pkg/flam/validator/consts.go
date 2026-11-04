package validator

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.validator"

// Configuration paths and container groups of the parsers, converters and
// validators.
const (
	// ParserConfigPath is the configuration path holding one parser
	// configuration per parser id, as read by the parser factory.
	ParserConfigPath = "flam.validator.parsers"

	// ParserCreatorGroup is the container group the parser driver bundles
	// provide their [flam.IValidationParserCreator] in, collected by the
	// parser factory.
	ParserCreatorGroup = "flam.validator.parsers.creator"

	// ConverterConfigPath is the configuration path holding one converter
	// configuration per converter id, as read by the converter factory.
	ConverterConfigPath = "flam.validator.converters"

	// ConverterCreatorGroup is the container group the converter driver
	// bundles provide their [flam.IValidationErrorConverterCreator] in,
	// collected by the converter factory.
	ConverterCreatorGroup = "flam.validator.converters.creator"

	// ValidatorConfigPath is the configuration path holding one validator
	// configuration per validator id, as read by the validator factory.
	ValidatorConfigPath = "flam.validator.validators"

	// ValidatorCreatorGroup is the container group the validator driver
	// bundles provide their [flam.IValidatorCreator] in, collected by the
	// validator factory.
	ValidatorCreatorGroup = "flam.validator.validators.creator"
)

// Fields of a validator configuration shared by the validator drivers.
const (
	// ConfigFieldParser is the field holding the id of the parser of the
	// validator, in the parser factory.
	ConfigFieldParser = "parser"

	// ConfigFieldConverter is the field holding the id of the converter of
	// the validator, in the converter factory, an empty id meaning none.
	ConfigFieldConverter = "converter"
)

// Configuration paths of the values taken by the shared fields missing
// from a validator configuration.
const (
	// DefaultParserConfigPath is the path of the default of
	// [ConfigFieldParser], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/validator/parsers/standard] to
	// its parser.
	DefaultParserConfigPath = "flam.validator.defaults.validators.parser"

	// DefaultConverterConfigPath is the path of the default of
	// [ConfigFieldConverter], unset unless configured.
	DefaultConverterConfigPath = "flam.validator.defaults.validators.converter"
)

// Context fields of the [*flam.Error] values returned by the validators.
const (
	// ErrorFieldID holds the id of a validator.
	ErrorFieldID = "id"

	// ErrorFieldType holds the type of the value of an error, as formatted
	// by fmt %T.
	ErrorFieldType = "type"
)
