package log

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The logger is optional, so that registering the driver does
// not require the log feature: its absence only fails the creation of an
// interceptor.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Clock  flam.IClock
	Logger flam.ILogger `optional:"true"`
}

// newCreator returns the [flam.IGrpcInterceptorCreator] of the log
// interceptors, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IGrpcInterceptorCreator {
	return &creator{
		config: args.Config,
		clock:  args.Clock,
		logger: args.Logger,
	}
}

// creator is the [flam.IGrpcInterceptorCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
	clock  flam.IClock
	logger flam.ILogger
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the interceptor of the channel and the levels of the
// configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IGrpcInterceptor, error) {
	id := bag.String(factory.ConfigFieldId)

	channel, ok := c.field(bag, ConfigFieldChannel, DefaultChannelConfigPath, defaultChannel).(string)
	if !ok || channel == "" {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldChannel)
	}
	levels := map[string]flam.LogLevel{}
	for field, def := range map[string]struct {
		path  string
		level flam.LogLevel
	}{
		ConfigFieldLevel:      {DefaultLevelConfigPath, defaultLevel},
		ConfigFieldLevelError: {DefaultLevelErrorConfigPath, defaultLevelError},
	} {
		const invalid = flam.LogLevel(-1)
		level := flam.LogNone.From(c.field(bag, field, def.path, def.level), invalid)
		if level == invalid {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
		}
		levels[field] = level
	}
	if c.logger == nil {
		return nil, newErrCreate(ErrMissingLogger, creatorCreateMessage, id, "")
	}

	return &interceptor{
		id:         id,
		logger:     c.logger,
		clock:      c.clock,
		channel:    channel,
		level:      levels[ConfigFieldLevel],
		levelError: levels[ConfigFieldLevelError],
	}, nil
}

// field returns the value of the field of the interceptor configuration,
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
const creatorCreateMessage = "unable to create log grpc interceptor"
