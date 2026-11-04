package console

import (
	"io"
	"os"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Serializers flam.ILogSerializerFactory
}

// newCreator returns the [flam.ILogStreamCreator] of the console streams,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILogStreamCreator {
	return &creator{
		config:      args.Config,
		serializers: args.Serializers,
	}
}

// creator is the [flam.ILogStreamCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	serializers flam.ILogSerializerFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns a stream writing to the output, read when the stream is
// created, and leaving it open when closed.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ILogStream, error) {
	id := bag.String(factory.ConfigFieldId)

	settings, invalid := base.Read(c.config, bag, base.Defaults{
		LevelPath:      DefaultLevelConfigPath,
		Level:          defaultLevel,
		SerializerPath: DefaultSerializerConfigPath,
		Serializer:     defaultSerializer,
	})
	if invalid != "" {
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, invalid)
	}
	output, _ := base.Field(c.config, bag, ConfigFieldOutput, DefaultOutputConfigPath, defaultOutput).(string)
	var writer io.Writer
	switch strings.ToLower(output) {
	case OutputStdout:
		writer = os.Stdout
	case OutputStderr:
		writer = os.Stderr
	default:
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, ConfigFieldOutput)
	}

	serializer, err := base.Serializer(c.serializers, sentinels, creatorCreateMessage, id, settings.Serializer)
	if err != nil {
		return nil, err
	}
	return base.New(id, settings.Level, settings.Channels, serializer, base.Plain(writer), nil), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create console log stream"
