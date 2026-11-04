package file

import (
	"os"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The disk factory is optional, so that registering the driver
// does not require the disk feature: its absence only fails the creation
// of a stream.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Serializers flam.ILogSerializerFactory
	Disks       flam.IDiskFactory `optional:"true"`
}

// newCreator returns the [flam.ILogStreamCreator] of the file streams,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILogStreamCreator {
	return &creator{
		config:      args.Config,
		serializers: args.Serializers,
		disks:       args.Disks,
	}
}

// creator is the [flam.ILogStreamCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	serializers flam.ILogSerializerFactory
	disks       flam.IDiskFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns a stream appending to the file, created when missing, and
// closing it when closed.
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
	diskID, ok := base.NonEmpty(base.Field(c.config, bag, ConfigFieldDisk, DefaultDiskConfigPath, defaultDisk))
	if !ok {
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, ConfigFieldDisk)
	}
	path, ok := base.NonEmpty(bag.Get(ConfigFieldPath))
	if !ok {
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, ConfigFieldPath)
	}

	serializer, err := base.Serializer(c.serializers, sentinels, creatorCreateMessage, id, settings.Serializer)
	if err != nil {
		return nil, err
	}
	disk, err := base.Disk(c.disks, sentinels, creatorCreateMessage, id, diskID)
	if err != nil {
		return nil, err
	}
	file, err := disk.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, base.NewErrOpen(sentinels, creatorCreateMessage, id, diskID, path, err)
	}
	return base.New(id, settings.Level, settings.Channels, serializer, base.Plain(file), file), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create file log stream"
