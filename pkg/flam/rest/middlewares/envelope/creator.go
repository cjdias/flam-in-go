package envelope

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The logger is optional: without it, the middlewares answer
// the panics without reporting them.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Logger flam.ILogger `optional:"true"`
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the envelope
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
		logger: args.Logger,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	logger flam.ILogger
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the channel, the level and the
// exposure of the errors of the configuration, or of their defaults,
// reporting to the logger of the container, if any.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	channel, ok := c.field(bag, ConfigFieldChannel, DefaultChannelConfigPath, defaultChannel).(string)
	if !ok || channel == "" {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldChannel)
	}
	const invalid = flam.LogLevel(-1)
	level := flam.LogNone.From(c.field(bag, ConfigFieldLevel, DefaultLevelConfigPath, defaultLevel), invalid)
	if level == invalid {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldLevel)
	}
	exposeErrors, ok := c.field(bag, ConfigFieldExposeErrors, DefaultExposeErrorsConfigPath, defaultExposeErrors).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldExposeErrors)
	}

	return &middleware{
		id:           id,
		logger:       c.logger,
		channel:      channel,
		level:        level,
		exposeErrors: exposeErrors,
	}, nil
}

// field returns the value of the field of the middleware configuration,
// or else the one stored in the configuration at the default path, or
// else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create envelope rest middleware"
