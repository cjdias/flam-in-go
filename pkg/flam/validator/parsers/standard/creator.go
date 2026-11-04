package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The translator factory is optional, so that the parsers can
// be created without the translator bundle, failing to create the parsers
// of a translator only.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Translators flam.ITranslatorFactory `optional:"true"`
}

// newCreator returns the [flam.IValidationParserCreator] of the standard
// parsers, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IValidationParserCreator {
	return &creator{
		config:      args.Config,
		translators: args.Translators,
	}
}

// creator is the [flam.IValidationParserCreator] of the configurations
// with the [Driver] driver.
type creator struct {
	config      flam.IConfig
	translators flam.ITranslatorFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the parser of the annotation and the translator of the
// configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IValidationParser, error) {
	id := bag.String(factory.ConfigFieldId)

	annotation, ok := bag.Get(ConfigFieldAnnotation, defaultAnnotation).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldAnnotation)
	}
	translatorID, ok := c.translatorID(bag)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTranslator)
	}

	var translator flam.ITranslator
	if translatorID != "" {
		if c.translators == nil {
			return nil, newErrTranslator(creatorCreateMessage, id, translatorID, errNoTranslators)
		}
		var err error
		if translator, err = c.translators.Get(translatorID); err != nil {
			return nil, newErrTranslator(creatorCreateMessage, id, translatorID, err)
		}
	}
	return NewParser(id, annotation, translator)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard validation parser"

// translatorID returns the id held by the translator field of the parser
// configuration, or else by the configuration at its default path, empty
// when unset, and false when it is not a string.
func (c *creator) translatorID(
	bag flam.IBag,
) (string, bool) {
	val := bag.Get(ConfigFieldTranslator)
	if val == nil {
		val = c.config.Get(DefaultTranslatorConfigPath)
	}
	if val == nil {
		return "", true
	}
	s, ok := val.(string)
	return s, ok
}
