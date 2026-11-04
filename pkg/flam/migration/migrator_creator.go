package migration

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// migratorCreatorArgs are the dependencies of the migrator creator, taken
// from the container. The group tag repeats MigrationGroup, as a struct
// tag cannot use a constant.
type migratorCreatorArgs struct {
	dig.In

	Config      flam.IConfig
	Clock       flam.IClock
	Connections flam.IDatabaseConnectionFactory
	Loggers     flam.IMigrationLoggerAdaptorFactory
	Migrations  []flam.IMigration `group:"flam.migration.migrations"`
}

// newMigratorCreator returns the [flam.IMigratorCreator] of the migrators,
// reading the defaults of the missing fields from the configuration. It is
// a provider registered by the bundle returned from [NewBundle].
func newMigratorCreator(
	args migratorCreatorArgs,
) flam.IMigratorCreator {
	return &migratorCreator{
		config:      args.Config,
		clock:       args.Clock,
		connections: args.Connections,
		loggers:     args.Loggers,
		migrations:  args.Migrations,
	}
}

// migratorCreator is the [flam.IMigratorCreator] of the configurations
// without driver or with the [Driver] driver.
type migratorCreator struct {
	config      flam.IConfig
	clock       flam.IClock
	connections flam.IDatabaseConnectionFactory
	loggers     flam.IMigrationLoggerAdaptorFactory
	migrations  []flam.IMigration
}

func (c *migratorCreator) Accept(
	bag flam.IBag,
) bool {
	driver := bag.Get(factory.ConfigFieldDriver)
	return driver == nil || driver == Driver
}

// Create returns the migrator of the migrations of the container of its
// group, with the connection and the logger adaptor of the ids of the
// configuration, or of their defaults.
func (c *migratorCreator) Create(
	bag flam.IBag,
) (flam.IMigrator, error) {
	id := bag.String(factory.ConfigFieldId)

	connectionID, ok := c.field(bag, ConfigFieldConnection, c.config.Get(DefaultConnectionConfigPath))
	if !ok || connectionID == "" {
		return nil, newErrField(ErrInvalidField, migratorCreatorCreateMessage, id, ErrorFieldField, ConfigFieldConnection)
	}
	group, ok := c.field(bag, ConfigFieldGroup, id)
	if !ok || group == "" {
		return nil, newErrField(ErrInvalidField, migratorCreatorCreateMessage, id, ErrorFieldField, ConfigFieldGroup)
	}
	loggerID, ok := c.field(bag, ConfigFieldLogger, c.config.Get(DefaultLoggerConfigPath, ""))
	if !ok {
		return nil, newErrField(ErrInvalidField, migratorCreatorCreateMessage, id, ErrorFieldField, ConfigFieldLogger)
	}

	var migrations []flam.IMigration
	for _, migration := range c.migrations {
		if value.IsNil(migration) {
			return nil, newErrMigrator(ErrNilMigration, migratorCreatorCreateMessage, id, group)
		}
		if migration.Group() == group {
			migrations = append(migrations, migration)
		}
	}

	connection, err := c.connections.Get(connectionID)
	if err != nil {
		return nil, newErrJoined(ErrConnection, migratorCreatorCreateMessage, id, ErrorFieldConnection, connectionID, err)
	}
	var logger flam.IMigrationLoggerAdaptor
	if loggerID != "" {
		if logger, err = c.loggers.Get(loggerID); err != nil {
			return nil, newErrJoined(ErrLogger, migratorCreatorCreateMessage, id, ErrorFieldLogger, loggerID, err)
		}
	}
	return NewMigrator(id, group, migrations, connection, logger, c.clock)
}

// field returns the string held by the field of the migrator
// configuration, or else by def, and false when it is not a string.
func (c *migratorCreator) field(
	bag flam.IBag,
	field string,
	def any,
) (string, bool) {
	val := def
	if bag.Has(field) {
		val = bag.Get(field)
	}
	s, ok := val.(string)
	return s, ok
}

// migratorCreatorCreateMessage is the message of the errors returned by
// Create.
const migratorCreatorCreateMessage = "unable to create migrator"
