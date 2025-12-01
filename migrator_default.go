package flam

import (
	"slices"
	"time"

	"gorm.io/gorm"
)

type defaultMigrator struct {
	connection DatabaseConnection
	logger     MigratorLogger
	migrations []Migration
	dao        *migrationDao
}

var _ Migrator = (*defaultMigrator)(nil)

func newDefaultMigrator(
	connection DatabaseConnection,
	logger MigratorLogger,
	migrations []Migration,
) (Migrator, error) {
	dao, e := newMigrationDao(connection)
	if e != nil {
		return nil, e
	}

	return &defaultMigrator{
		connection: connection,
		logger:     logger,
		migrations: migrations,
		dao:        dao}, nil
}

func (migrator *defaultMigrator) List() ([]MigrationInfo, error) {
	installed, e := migrator.dao.List(migrator.connection)
	if e != nil {
		return nil, e
	}

	var migrations []MigrationInfo
	for _, migration := range migrator.migrations {
		var createdAt *time.Time
		for _, m := range installed {
			if m.Version == migration.Version() {
				createdAt = &m.CreatedAt
				break
			}
		}

		migrations = append(migrations, MigrationInfo{
			Version:     migration.Version(),
			Description: migration.Description(),
			InstalledAt: createdAt,
		})
	}

	return migrations, nil
}

func (migrator *defaultMigrator) Current() (*MigrationInfo, error) {
	last, e := migrator.dao.Last(migrator.connection)
	switch {
	case e != nil:
		return nil, e
	case last.ID == 0:
		return nil, nil
	}

	return &MigrationInfo{
		Version:     last.Version,
		Description: last.Description,
		InstalledAt: &last.CreatedAt}, nil
}

func (migrator *defaultMigrator) CanUp() bool {
	list, e := migrator.List()
	if e != nil {
		return false
	}

	for _, migration := range list {
		if migration.InstalledAt == nil {
			return true
		}
	}

	return false
}

func (migrator *defaultMigrator) CanDown() bool {
	list, e := migrator.List()
	if e != nil {
		return false
	}

	for _, migration := range list {
		if migration.InstalledAt != nil {
			return true
		}
	}

	return false
}

func (migrator *defaultMigrator) Up() error {
	last, e := migrator.dao.Last(migrator.connection)
	if e != nil {
		return e
	}

	if last.ID == 0 && len(migrator.migrations) > 0 {
		return migrator.up(migrator.migrations[0])
	}

	if len(migrator.migrations) != 0 &&
		migrator.migrations[len(migrator.migrations)-1].Version() != last.Version {
		for i, migration := range migrator.migrations {
			if migration.Version() == last.Version {
				return migrator.up(migrator.migrations[i+1])
			}
		}
	}

	return ErrUnknownResource
}

func (migrator *defaultMigrator) UpAll() error {
	last, e := migrator.dao.Last(migrator.connection)
	if e != nil {
		return e
	}

	execute := false
	for _, migration := range migrator.migrations {
		if last.ID == 0 || execute {
			if e := migrator.up(migration); e != nil {
				return e
			}
		} else if migration.Version() == last.Version {
			execute = true
		}
	}

	return nil
}

func (migrator *defaultMigrator) Down() error {
	last, e := migrator.dao.Last(migrator.connection)
	if e != nil {
		return e
	}

	if last.ID != 0 {
		for i, migration := range migrator.migrations {
			if migration.Version() == last.Version {
				return migrator.down(migrator.migrations[i], last)
			}
		}
	}

	return ErrUnknownResource
}

func (migrator *defaultMigrator) DownAll() error {
	if len(migrator.migrations) == 0 {
		return nil
	}

	slices.Reverse(migrator.migrations)

	for {
		last, e := migrator.dao.Last(migrator.connection)
		switch {
		case e != nil:
			return e
		case last.ID == 0:
			return nil
		}

		for i, migration := range migrator.migrations {
			if migration.Version() == last.Version {
				e = migrator.down(migrator.migrations[i], last)
				if e != nil {
					return e
				}
				break
			}
		}
	}
}

func (migrator *defaultMigrator) up(
	migration Migration,
) error {
	migrator.logUpStart(migration)

	if e := migrator.connection.Transaction(func(tx *gorm.DB) error {
		if e := migration.Up(tx); e != nil {
			migrator.logUpError(migration, e)
			return e
		}

		if _, e := migrator.dao.Up(tx, migration.Version(), migration.Description()); e != nil {
			migrator.logUpError(migration, e)
			return e
		}

		return nil
	}); e != nil {
		return e
	}

	migrator.logUpDone(migration)

	return nil
}

func (migrator *defaultMigrator) down(
	migration Migration,
	record *migrationRecord,
) error {
	migrator.logDownStart(migration)

	if e := migrator.connection.Transaction(func(tx *gorm.DB) error {
		if e := migration.Down(tx); e != nil {
			migrator.logDownError(migration, e)
			return e
		}

		if e := migrator.dao.Down(tx, record); e != nil {
			migrator.logDownError(migration, e)
			return e
		}

		return nil
	}); e != nil {
		return e
	}

	migrator.logDownDone(migration)

	return nil
}

func (migrator *defaultMigrator) logUpStart(
	migration Migration,
) {
	if migrator.logger != nil {
		migrator.logger.LogUpStart(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()})
	}
}

func (migrator *defaultMigrator) logUpError(
	migration Migration,
	e error,
) {
	if migrator.logger != nil {
		migrator.logger.LogUpError(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()},
			e)
	}
}

func (migrator *defaultMigrator) logUpDone(
	migration Migration,
) {
	if migrator.logger != nil {
		migrator.logger.LogUpDone(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()})
	}
}

func (migrator *defaultMigrator) logDownStart(
	migration Migration,
) {
	if migrator.logger != nil {
		migrator.logger.LogDownStart(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()})
	}
}

func (migrator *defaultMigrator) logDownError(
	migration Migration,
	e error,
) {
	if migrator.logger != nil {
		migrator.logger.LogDownError(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()},
			e)
	}
}

func (migrator *defaultMigrator) logDownDone(
	migration Migration,
) {
	if migrator.logger != nil {
		migrator.logger.LogDownDone(
			MigrationInfo{
				Version:     migration.Version(),
				Description: migration.Description()})
	}
}
