package migration

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.migration"

// Configuration paths and container groups of the migrators, the logger
// adaptors and the migrations.
const (
	// MigratorConfigPath is the configuration path holding one migrator
	// configuration per migrator id, as read by the migrator factory.
	MigratorConfigPath = "flam.migration.migrators"

	// MigratorCreatorGroup is the container group holding the
	// [flam.IMigratorCreator] values collected by the migrator factory,
	// among which the one of the bundle returned by [NewBundle].
	MigratorCreatorGroup = "flam.migration.migrators.creator"

	// LoggerConfigPath is the configuration path holding one logger
	// adaptor configuration per adaptor id, as read by the logger adaptor
	// factory.
	LoggerConfigPath = "flam.migration.loggers"

	// LoggerCreatorGroup is the container group the logger adaptor driver
	// bundles provide their [flam.IMigrationLoggerAdaptorCreator] in,
	// collected by the logger adaptor factory.
	LoggerCreatorGroup = "flam.migration.loggers.creator"

	// MigrationGroup is the container group the application provides its
	// [flam.IMigration] values in, each migrator taking those of its group.
	MigrationGroup = "flam.migration.migrations"

	// BootConfigPath is the configuration path of a bool telling whether
	// the migration bundle, when the application boots, applies every
	// pending migration of every migrator configured at
	// [MigratorConfigPath]. It defaults to false.
	BootConfigPath = "flam.migration.boot"
)

// Driver is the driver name of the migrators created by the creator of the
// bundle returned by [NewBundle], held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration, which may also be left out.
const Driver = "flam.migration.migrators.driver.standard"

// Fields of a migrator configuration.
const (
	// ConfigFieldConnection is the field holding the id of the database
	// connection of the migrator, in the connection factory.
	ConfigFieldConnection = "connection"

	// ConfigFieldGroup is the field holding the group of the migrations of
	// the migrator. It defaults to the id of the migrator.
	ConfigFieldGroup = "group"

	// ConfigFieldLogger is the field holding the id of the logger adaptor
	// of the migrator, in the logger adaptor factory. An empty id leaves
	// the migrator without logger adaptor.
	ConfigFieldLogger = "logger"
)

// Configuration paths of the values taken by the fields missing from a
// migrator configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], unset unless configured.
	DefaultConnectionConfigPath = "flam.migration.defaults.migrators.connection"

	// DefaultLoggerConfigPath is the path of the default of
	// [ConfigFieldLogger], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/migration/loggers/standard] to
	// its adaptor, and unset otherwise: the migrators then have no logger
	// adaptor.
	DefaultLoggerConfigPath = "flam.migration.defaults.migrators.logger"
)

// RecordTable is the name of the table in which the migrators record the
// migrations applied, keyed by their group and id.
const RecordTable = "__migrations"

// File name suffixes of the SQL files of the migrations returned by
// [NewFileMigration].
const (
	// UpFileSuffix is the suffix of the file applying the migration.
	UpFileSuffix = ".up.sql"

	// DownFileSuffix is the suffix of the file reverting the migration.
	DownFileSuffix = ".down.sql"
)

// Context fields of the [*flam.Error] values returned by the migration
// package.
const (
	// ErrorFieldID holds the id of a migrator.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the connection of a migrator.
	ErrorFieldConnection = "connection"

	// ErrorFieldLogger holds the id of the logger adaptor of a migrator.
	ErrorFieldLogger = "logger"

	// ErrorFieldGroup holds the group of a migrator.
	ErrorFieldGroup = "group"

	// ErrorFieldMigration holds the id of a migration.
	ErrorFieldMigration = "migration"

	// ErrorFieldDependency holds the id of a dependency of a migration.
	ErrorFieldDependency = "dependency"

	// ErrorFieldMigrations holds, sorted, the ids of the migrations of a
	// dependency cycle, or depending on one.
	ErrorFieldMigrations = "migrations"

	// ErrorFieldPath holds the path of the SQL file of a migration.
	ErrorFieldPath = "path"
)

// Defaults of the migration bundle behaviour.
const (
	defaultBoot = false
)
