package flam

import (
	"time"
)

// MigrationInfo describes a migration, as listed, applied or reverted by
// an [IMigrator].
type MigrationInfo struct {
	// Id is the id of the migration, unique in its group.
	Id string

	// Description is the description of the migration.
	Description string

	// InstalledAt is the time the migration was applied at, or nil when it
	// is not applied.
	InstalledAt *time.Time
}

// IMigrationLoggerAdaptor reports the progress of the migrations of an
// [IMigrator]: each migration applied or reverted starts, then either
// fails or is done. It is held by an [IMigrationLoggerAdaptorFactory].
type IMigrationLoggerAdaptor interface {
	IResource

	// UpStart reports that the migration starts being applied.
	UpStart(info MigrationInfo)

	// UpError reports that the migration failed to be applied, with e.
	UpError(info MigrationInfo, e error)

	// UpDone reports that the migration was applied.
	UpDone(info MigrationInfo)

	// DownStart reports that the migration starts being reverted.
	DownStart(info MigrationInfo)

	// DownError reports that the migration failed to be reverted, with e.
	DownError(info MigrationInfo, e error)

	// DownDone reports that the migration was reverted.
	DownDone(info MigrationInfo)
}

// IMigrationLoggerAdaptorCreator creates the migration logger adaptors of
// a driver, for the [IMigrationLoggerAdaptorFactory].
type IMigrationLoggerAdaptorCreator = IResourceCreator[IMigrationLoggerAdaptor]

// IMigrationLoggerAdaptorFactory creates and holds the migration logger
// adaptors, one per id.
type IMigrationLoggerAdaptorFactory = IFactory[IMigrationLoggerAdaptor]

// IMigration is a change of a database, applied by Up and reverted by
// Down, with the connection of the transaction it runs in. Its Id, unique
// in its group, identifies it among the migrations applied to a database,
// so it must never change once applied.
type IMigration interface {
	IResource

	// Description returns the description of the migration.
	Description() string

	// Group returns the group of the migration: an [IMigrator] handles the
	// migrations of one group.
	Group() string

	// Dependencies returns the ids of the migrations of the group to apply
	// before this one.
	Dependencies() []string

	// Up applies the migration with the connection.
	Up(connection IDatabaseConnection) error

	// Down reverts the migration with the connection.
	Down(connection IDatabaseConnection) error
}

// IMigrator applies and reverts the migrations of a group to a database,
// in the order of their dependencies, migrations that do not depend on
// one another in id order, recording in the database the ones applied.
// It is held by an [IMigratorFactory].
type IMigrator interface {
	IResource

	// List returns the migrations of the group, in the order they are
	// applied, with the time each applied one was applied at.
	List() ([]MigrationInfo, error)

	// CanUp reports whether a migration of the group is not applied. It
	// returns false when the database cannot be read.
	CanUp() bool

	// CanDown reports whether a migration of the group is applied. It
	// returns false when the database cannot be read.
	CanDown() bool

	// Up applies the first migration not applied, and returns it.
	Up() (*MigrationInfo, error)

	// UpAll applies every migration not applied, in order, stopping at the
	// first failure, and returns the ones applied.
	UpAll() ([]MigrationInfo, error)

	// Down reverts the last migration applied, and returns it.
	Down() (*MigrationInfo, error)

	// DownAll reverts every applied migration, the last applied first,
	// stopping at the first failure, and returns the ones reverted.
	DownAll() ([]MigrationInfo, error)
}

// IMigratorCreator creates the migrators, for the [IMigratorFactory].
type IMigratorCreator = IResourceCreator[IMigrator]

// IMigratorFactory creates and holds the migrators, one per id.
type IMigratorFactory = IFactory[IMigrator]
