package dir

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/internal/static"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The factories are optional, so that registering the driver
// does not require the disk feature: their absence only fails the
// creation of a source.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Disks   flam.IDiskFactory         `optional:"true"`
	Parsers flam.IConfigParserFactory `optional:"true"`
}

// newCreator returns the [flam.IConfigSourceCreator] of the dir sources,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IConfigSourceCreator {
	return &creator{
		config:  args.Config,
		disks:   args.Disks,
		parsers: args.Parsers,
	}
}

// creator is the [flam.IConfigSourceCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config  flam.IConfig
	disks   flam.IDiskFactory
	parsers flam.IConfigParserFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

func (c *creator) Create(
	bag flam.IBag,
) (flam.IConfigSource, error) {
	id := bag.String(factory.ConfigFieldId)

	priority, ok := c.field(bag, ConfigFieldPriority, DefaultPriorityConfigPath, defaultPriority).(int)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPriority)
	}
	reload, ok := c.field(bag, ConfigFieldReload, DefaultReloadConfigPath, defaultReload).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldReload)
	}
	dirPath, ok := bag.Get(ConfigFieldPath).(string)
	if !ok || dirPath == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPath)
	}
	recursive, ok := c.field(bag, ConfigFieldRecursive,
		DefaultRecursiveConfigPath, defaultRecursive).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldRecursive)
	}
	diskID, ok := c.field(bag, ConfigFieldDisk, DefaultDiskConfigPath, defaultDisk).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldDisk)
	}
	parserID, ok := c.field(bag, ConfigFieldParser, DefaultParserConfigPath, defaultParser).(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldParser)
	}

	if c.disks == nil {
		return nil, newErrDisk(creatorCreateMessage, id, diskID, ErrMissingFactory)
	}
	disk, err := c.disks.Get(diskID)
	if err != nil {
		return nil, newErrDisk(creatorCreateMessage, id, diskID, err)
	}
	if c.parsers == nil {
		return nil, newErrParser(creatorCreateMessage, id, parserID, ErrMissingFactory)
	}
	var parser flam.IConfigParser
	if parserID != "" {
		if parser, err = c.parsers.Get(parserID); err != nil {
			return nil, newErrParser(creatorCreateMessage, id, parserID, err)
		}
	}

	s := &source{
		Source:    base.New(id, priority),
		diskID:    diskID,
		disk:      disk,
		path:      dirPath,
		recursive: recursive,
		parserID:  parserID,
		parser:    parser,
		parsers:   c.parsers,
	}
	values, snapshot, err := s.load(creatorCreateMessage, nil)
	if err != nil {
		return nil, err
	}
	s.Swap(values)
	s.snapshot = snapshot
	if !reload {
		return static.New(s), nil
	}
	return s, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create dir config source"

// field returns the value of the field of the source configuration, or
// else its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return base.Field(c.config, bag, field, defaultPath, builtIn)
}
