package envelope

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.IValidationErrorConverterCreator] of the
// envelope converters, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IValidationErrorConverterCreator {
	return &creator{
		config: args.Config,
	}
}

// creator is the [flam.IValidationErrorConverterCreator] of the
// configurations with the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the converter of the status of the configuration, or of
// its default.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IValidationErrorConverter, error) {
	id := bag.String(factory.ConfigFieldId)

	status, ok := settings.Field(c.config, bag, ConfigFieldStatus, DefaultStatusConfigPath, defaultStatus).(int)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldStatus)
	}
	return NewConverter(id, status)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create envelope validation converter"
