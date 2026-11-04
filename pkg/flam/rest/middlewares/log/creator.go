package log

import (
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The logger is optional, so that registering the driver does
// not require the log feature: its absence only fails the creation of a
// middleware.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
	Clock  flam.IClock
	Logger flam.ILogger `optional:"true"`
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the log
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
		clock:  args.Clock,
		logger: args.Logger,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
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

// Create returns the middleware of the channel, the levels, the JSON
// decoding, the maximum body size, and the redacted headers and query
// parameters of the configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
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
		ConfigFieldLevelRequest:  {DefaultLevelRequestConfigPath, defaultLevelRequest},
		ConfigFieldLevelResponse: {DefaultLevelResponseConfigPath, defaultLevelResponse},
		ConfigFieldLevelError:    {DefaultLevelErrorConfigPath, defaultLevelError},
	} {
		const invalid = flam.LogLevel(-1)
		level := flam.LogNone.From(c.field(bag, field, def.path, def.level), invalid)
		if level == invalid {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
		}
		levels[field] = level
	}
	parseJSON, ok := c.field(bag, ConfigFieldParseJSON, DefaultParseJSONConfigPath, defaultParseJSON).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldParseJSON)
	}
	maxBody, ok := c.field(bag, ConfigFieldMaxBody, DefaultMaxBodyConfigPath, defaultMaxBody).(int)
	if !ok || maxBody < 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMaxBody)
	}
	redactHeaders, ok := creatorNames(c.field(bag, ConfigFieldRedactHeaders, DefaultRedactHeadersConfigPath, defaultRedactHeaders()))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldRedactHeaders)
	}
	redactParams, ok := creatorNames(c.field(bag, ConfigFieldRedactParams, DefaultRedactParamsConfigPath, defaultRedactParams()))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldRedactParams)
	}
	if c.logger == nil {
		return nil, newErrCreate(ErrMissingLogger, creatorCreateMessage, id, "")
	}

	return &middleware{
		id:            id,
		logger:        c.logger,
		clock:         c.clock,
		channel:       channel,
		levelRequest:  levels[ConfigFieldLevelRequest],
		levelResponse: levels[ConfigFieldLevelResponse],
		levelError:    levels[ConfigFieldLevelError],
		parseJSON:     parseJSON,
		maxBody:       maxBody,
		redactHeaders: redactHeaders,
		redactParams:  redactParams,
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

// creatorNames returns the set of the header or query parameter names of
// the value, a []string or a []any of strings, lower cased, as the names
// are matched whatever their case, and whether the value is such a list.
func creatorNames(
	value any,
) (map[string]struct{}, bool) {
	names, ok := settings.Strings(value)
	if !ok {
		return nil, false
	}
	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		set[strings.ToLower(name)] = struct{}{}
	}
	return set, true
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create log rest middleware"
