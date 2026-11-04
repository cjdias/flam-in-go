package standard

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The logger is optional, so that registering the driver does
// not require the log feature: its absence only fails the creation of an
// adaptor.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Logger flam.ILogger `optional:"true"`
}

// newCreator returns the [flam.IRestLoggerAdaptorCreator] of the standard
// adaptors, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestLoggerAdaptorCreator {
	return &creator{
		config: args.Config,
		logger: args.Logger,
	}
}

// creator is the [flam.IRestLoggerAdaptorCreator] of the configurations
// with the [Driver] driver.
type creator struct {
	config flam.IConfig
	logger flam.ILogger
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the adaptor of the channel and the levels of the
// configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestLoggerAdaptor, error) {
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
		ConfigFieldLevelStart:    {DefaultLevelStartConfigPath, defaultLevelStart},
		ConfigFieldLevelError:    {DefaultLevelErrorConfigPath, defaultLevelError},
		ConfigFieldLevelShutdown: {DefaultLevelShutdownConfigPath, defaultLevelShutdown},
		ConfigFieldLevelDone:     {DefaultLevelDoneConfigPath, defaultLevelDone},
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

	return &adaptor{
		id:       id,
		logger:   c.logger,
		channel:  channel,
		start:    levels[ConfigFieldLevelStart],
		error:    levels[ConfigFieldLevelError],
		shutdown: levels[ConfigFieldLevelShutdown],
		done:     levels[ConfigFieldLevelDone],
	}, nil
}

// field returns the value of the field of the adaptor configuration, or
// else the one stored in the configuration at the default path, or else
// the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard rest logger"
