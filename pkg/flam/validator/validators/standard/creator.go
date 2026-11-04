package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/validator"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config     flam.IConfig
	Parsers    flam.IValidationParserFactory
	Converters flam.IValidationErrorConverterFactory
}

// newCreator returns the [flam.IValidatorCreator] of the standard
// validators, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IValidatorCreator {
	return &creator{
		config:     args.Config,
		parsers:    args.Parsers,
		converters: args.Converters,
	}
}

// creator is the [flam.IValidatorCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config     flam.IConfig
	parsers    flam.IValidationParserFactory
	converters flam.IValidationErrorConverterFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the validator with the parser and the converter of the
// ids of the configuration, or of their defaults, without converter when
// its id is empty.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IValidator, error) {
	id := bag.String(factory.ConfigFieldId)

	parserID, ok := c.field(bag, validator.ConfigFieldParser, validator.DefaultParserConfigPath)
	if !ok || parserID == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, validator.ConfigFieldParser)
	}
	converterID, ok := c.field(bag, validator.ConfigFieldConverter, validator.DefaultConverterConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, validator.ConfigFieldConverter)
	}

	parser, err := c.parsers.Get(parserID)
	if err != nil {
		return nil, newErrJoined(ErrParser, creatorCreateMessage, id, ErrorFieldParser, parserID, err)
	}
	var converter flam.IValidationErrorConverter
	if converterID != "" {
		if converter, err = c.converters.Get(converterID); err != nil {
			return nil, newErrJoined(ErrConverter, creatorCreateMessage, id, ErrorFieldConverter, converterID, err)
		}
	}
	return NewValidator(id, parser, converter)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard validator"

// field returns the id held by the field of the validator configuration,
// or else by the configuration at the default path, empty when unset, and
// false when it is not a string.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
) (string, bool) {
	val := bag.Get(field)
	if val == nil {
		val = c.config.Get(defaultPath)
	}
	if val == nil {
		return "", true
	}
	s, ok := val.(string)
	return s, ok
}
