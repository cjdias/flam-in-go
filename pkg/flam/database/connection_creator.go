package database

import (
	"maps"

	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// connectionCreatorArgs are the dependencies of the connection creator,
// taken from the container.
type connectionCreatorArgs struct {
	dig.In

	Config   flam.IConfig
	Dialects flam.IDatabaseDialectFactory
	Configs  flam.IDatabaseConfigFactory
}

// newConnectionCreator returns the [flam.IDatabaseConnectionCreator] of the
// gorm connections, reading the defaults of the missing fields from the
// configuration. It is a provider registered by the bundle returned from
// [NewBundle].
func newConnectionCreator(
	args connectionCreatorArgs,
) flam.IDatabaseConnectionCreator {
	return &connectionCreator{
		config:   args.Config,
		dialects: args.Dialects,
		configs:  args.Configs,
	}
}

// connectionCreator is the [flam.IDatabaseConnectionCreator] of the
// configurations without driver or with the [Driver] driver.
type connectionCreator struct {
	config   flam.IConfig
	dialects flam.IDatabaseDialectFactory
	configs  flam.IDatabaseConfigFactory
}

func (c *connectionCreator) Accept(
	bag flam.IBag,
) bool {
	driver := bag.Get(factory.ConfigFieldDriver)
	return driver == nil || driver == Driver
}

// Create opens the connection with the dialect and the gorm configuration
// of the ids of the configuration, or of their defaults.
func (c *connectionCreator) Create(
	bag flam.IBag,
) (flam.IDatabaseConnection, error) {
	id := bag.String(factory.ConfigFieldId)

	dialectID, ok := c.field(bag, ConfigFieldDialect, DefaultDialectConfigPath)
	if !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, id, ConfigFieldDialect)
	}
	configID, ok := c.field(bag, ConfigFieldConfig, DefaultConfigConfigPath)
	if !ok {
		return nil, newErrInvalidField(connectionCreatorCreateMessage, id, ConfigFieldConfig)
	}

	dialect, err := c.dialects.Get(dialectID)
	if err != nil {
		return nil, newErrDialect(connectionCreatorCreateMessage, id, dialectID, err)
	}
	config, err := c.configs.Get(configID)
	if err != nil {
		return nil, newErrConfig(connectionCreatorCreateMessage, id, configID, err)
	}

	// a copy, with maps of its own, as gorm registers the plugins in it
	gormConfig := config.Config
	gormConfig.Plugins = maps.Clone(gormConfig.Plugins)
	gormConfig.ClauseBuilders = maps.Clone(gormConfig.ClauseBuilders)
	db, err := gorm.Open(dialect.Gorm(), &gormConfig)
	if err != nil {
		return nil, newErrOpen(connectionCreatorCreateMessage, id, dialectID, configID, err)
	}
	return NewConnection(id, db)
}

// field returns the id held by the field of the connection configuration,
// or else by the default path, and false when it is not a non-empty
// string.
func (c *connectionCreator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
) (string, bool) {
	val := c.config.Get(defaultPath)
	if bag.Has(field) {
		val = bag.Get(field)
	}
	s, ok := val.(string)
	return s, ok && s != ""
}

// connectionCreatorCreateMessage is the message of the errors returned by
// Create.
const connectionCreatorCreateMessage = "unable to create database connection"
