package migration

import (
	"errors"
	"slices"
	"sync"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// NewMigrator returns the [flam.IMigrator] of the id, applying the
// migrations, all of the group, to the database of the connection,
// stamping them with the clock, and reporting its progress to the logger
// adaptor, which may be nil.
//
// It orders the migrations once: every migration after the ones it depends
// on, and the migrations that do not depend on one another in id order,
// so that the order is the same on every run. It then creates, when
// missing, the table [RecordTable] in which the migrators record, by group
// and id, the migrations applied.
//
// It fails with ErrNilConnection, ErrNilClock, or ErrNilMigration when the
// connection, the clock or a migration is nil, ErrForeignGroup when a
// migration is not of the group, ErrDuplicateMigration when two
// migrations have the same id, ErrUnknownDependency when a migration
// depends on an id that is not the one of a migration of the group,
// ErrDependencyCycle when migrations depend on one another in a cycle, or
// ErrRecords when the table cannot be created.
//
// Each migration is applied or reverted in a transaction of its own, along
// with its record, so that a failing migration leaves neither its changes
// nor its record, on the databases whose schema changes are transactional.
// A migrator serializes its own calls, but not those of other migrators
// or processes on the same database.
func NewMigrator(
	id string,
	group string,
	migrations []flam.IMigration,
	connection flam.IDatabaseConnection,
	logger flam.IMigrationLoggerAdaptor,
	clock flam.IClock,
) (flam.IMigrator, error) {
	switch {
	case value.IsNil(connection):
		return nil, newErrMigrator(ErrNilConnection, migratorNewMessage, id, group)
	case value.IsNil(clock):
		return nil, newErrMigrator(ErrNilClock, migratorNewMessage, id, group)
	}

	ordered, err := migratorOrder(id, group, migrations)
	if err != nil {
		return nil, err
	}
	if err := connection.AutoMigrate(&record{}); err != nil {
		return nil, newErrRecords(migratorNewMessage, id, group, err)
	}

	m := &migrator{
		id:         id,
		group:      group,
		migrations: ordered,
		connection: connection,
		clock:      clock,
	}
	if !value.IsNil(logger) {
		m.logger = logger
	}
	return m, nil
}

// migrator is the [flam.IMigrator] returned by [NewMigrator].
type migrator struct {
	mu         sync.Mutex
	id         string
	group      string
	migrations []flam.IMigration
	connection flam.IDatabaseConnection
	logger     flam.IMigrationLoggerAdaptor
	clock      flam.IClock
}

func (m *migrator) Id() string {
	return m.id
}

func (m *migrator) List() ([]flam.MigrationInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, err := m.records(migratorListMessage)
	if err != nil {
		return nil, err
	}
	infos := make([]flam.MigrationInfo, 0, len(m.migrations))
	for _, migration := range m.migrations {
		info := m.info(migration)
		if r, ok := installed[migration.Id()]; ok {
			info.InstalledAt = new(r.InstalledAt)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (m *migrator) CanUp() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, err := m.pending(migratorListMessage)
	return err == nil && len(pending) > 0
}

func (m *migrator) CanDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok, err := m.last(migratorListMessage)
	return err == nil && ok
}

func (m *migrator) Up() (*flam.MigrationInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, err := m.pending(migratorUpMessage)
	if err != nil {
		return nil, err
	}
	if len(pending) == 0 {
		return nil, newErrMigrator(ErrNoPending, migratorUpMessage, m.id, m.group)
	}
	return m.up(pending[0])
}

func (m *migrator) UpAll() ([]flam.MigrationInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pending, err := m.pending(migratorUpMessage)
	if err != nil {
		return nil, err
	}
	infos := []flam.MigrationInfo{}
	for _, migration := range pending {
		info, err := m.up(migration)
		if err != nil {
			return infos, err
		}
		infos = append(infos, *info)
	}
	return infos, nil
}

func (m *migrator) Down() (*flam.MigrationInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.downLast()
}

func (m *migrator) DownAll() ([]flam.MigrationInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	infos := []flam.MigrationInfo{}
	for {
		info, err := m.downLast()
		switch {
		case errors.Is(err, ErrNoInstalled):
			return infos, nil
		case err != nil:
			return infos, err
		}
		infos = append(infos, *info)
	}
}

// info returns the description of the migration, without install time.
func (m *migrator) info(
	migration flam.IMigration,
) flam.MigrationInfo {
	return flam.MigrationInfo{
		Id:          migration.Id(),
		Description: migration.Description(),
	}
}

// records returns the records of the group, by migration id.
func (m *migrator) records(
	message string,
) (map[string]record, error) {
	var rows []record
	if err := m.connection.Where("migration_group = ?", m.group).Find(&rows).Error; err != nil {
		return nil, newErrRecords(message, m.id, m.group, err)
	}
	installed := make(map[string]record, len(rows))
	for _, r := range rows {
		installed[r.Id] = r
	}
	return installed, nil
}

// pending returns the migrations not applied, in order.
func (m *migrator) pending(
	message string,
) ([]flam.IMigration, error) {
	installed, err := m.records(message)
	if err != nil {
		return nil, err
	}
	var pending []flam.IMigration
	for _, migration := range m.migrations {
		if _, ok := installed[migration.Id()]; !ok {
			pending = append(pending, migration)
		}
	}
	return pending, nil
}

// last returns the record of the last migration applied in the group, and
// whether there is one.
func (m *migrator) last(
	message string,
) (record, bool, error) {
	var rows []record
	err := m.connection.Where("migration_group = ?", m.group).
		Order("sequence desc").Limit(1).Find(&rows).Error
	if err != nil {
		return record{}, false, newErrRecords(message, m.id, m.group, err)
	}
	if len(rows) == 0 {
		return record{}, false, nil
	}
	return rows[0], true, nil
}

// downLast reverts the last migration applied.
func (m *migrator) downLast() (*flam.MigrationInfo, error) {
	last, ok, err := m.last(migratorDownMessage)
	switch {
	case err != nil:
		return nil, err
	case !ok:
		return nil, newErrMigrator(ErrNoInstalled, migratorDownMessage, m.id, m.group)
	}

	index := slices.IndexFunc(m.migrations, func(migration flam.IMigration) bool {
		return migration.Id() == last.Id
	})
	if index < 0 {
		return nil, newErrMigration(ErrUnknownMigration, migratorDownMessage, m.id, m.group, last.Id, nil)
	}
	return m.down(m.migrations[index])
}

// up applies the migration and records it, in a transaction.
func (m *migrator) up(
	migration flam.IMigration,
) (*flam.MigrationInfo, error) {
	info := m.info(migration)
	m.logUp(info, nil, false)

	installed := m.clock.Now()
	err := m.connection.Transaction(func(tx *gorm.DB) error {
		if err := migration.Up(m.connection.WithConnection(tx)); err != nil {
			return err
		}
		var sequence uint64
		err := tx.Model(&record{}).Where("migration_group = ?", m.group).
			Select("COALESCE(MAX(sequence), 0)").Scan(&sequence).Error
		if err != nil {
			return err
		}
		return tx.Create(&record{
			Group:       m.group,
			Id:          info.Id,
			Sequence:    sequence + 1,
			Description: info.Description,
			InstalledAt: installed,
		}).Error
	})
	if err != nil {
		m.logUp(info, err, true)
		return nil, newErrMigration(ErrUp, migratorUpMessage, m.id, m.group, info.Id, err)
	}

	m.logUp(info, nil, true)
	info.InstalledAt = &installed
	return &info, nil
}

// down reverts the migration and deletes its record, in a transaction.
func (m *migrator) down(
	migration flam.IMigration,
) (*flam.MigrationInfo, error) {
	info := m.info(migration)
	m.logDown(info, nil, false)

	err := m.connection.Transaction(func(tx *gorm.DB) error {
		if err := migration.Down(m.connection.WithConnection(tx)); err != nil {
			return err
		}
		return tx.Where("migration_group = ? AND migration_id = ?", m.group, info.Id).
			Delete(&record{}).Error
	})
	if err != nil {
		m.logDown(info, err, true)
		return nil, newErrMigration(ErrDown, migratorDownMessage, m.id, m.group, info.Id, err)
	}

	m.logDown(info, nil, true)
	return &info, nil
}

// logUp reports the start of the application of the migration, or its end,
// with its error, to the logger adaptor, when there is one.
func (m *migrator) logUp(
	info flam.MigrationInfo,
	err error,
	end bool,
) {
	switch {
	case m.logger == nil:
	case !end:
		m.logger.UpStart(info)
	case err != nil:
		m.logger.UpError(info, err)
	default:
		m.logger.UpDone(info)
	}
}

// logDown reports the start of the revert of the migration, or its end,
// with its error, to the logger adaptor, when there is one.
func (m *migrator) logDown(
	info flam.MigrationInfo,
	err error,
	end bool,
) {
	switch {
	case m.logger == nil:
	case !end:
		m.logger.DownStart(info)
	case err != nil:
		m.logger.DownError(info, err)
	default:
		m.logger.DownDone(info)
	}
}

// migratorOrder returns the migrations in the order they are applied:
// every migration after the ones it depends on, and the migrations that
// do not depend on one another in id order. It fails when the migrations
// cannot be ordered, as NewMigrator documents.
func migratorOrder(
	id string,
	group string,
	migrations []flam.IMigration,
) ([]flam.IMigration, error) {
	byID := make(map[string]flam.IMigration, len(migrations))
	for _, migration := range migrations {
		switch {
		case value.IsNil(migration):
			return nil, newErrMigrator(ErrNilMigration, migratorNewMessage, id, group)
		case migration.Group() != group:
			return nil, newErrMigration(ErrForeignGroup, migratorNewMessage, id, group, migration.Id(), nil)
		}
		if _, ok := byID[migration.Id()]; ok {
			return nil, newErrMigration(ErrDuplicateMigration, migratorNewMessage, id, group, migration.Id(), nil)
		}
		byID[migration.Id()] = migration
	}

	// the dependencies left to apply of each migration, and the
	// migrations depending on each one
	waiting := make(map[string]map[string]bool, len(byID))
	dependents := make(map[string][]string, len(byID))
	for migrationID, migration := range byID {
		waiting[migrationID] = map[string]bool{}
		for _, dependency := range migration.Dependencies() {
			if _, ok := byID[dependency]; !ok {
				return nil, newErrMigration(ErrUnknownDependency, migratorNewMessage, id, group, migrationID, nil).
					With(ErrorFieldDependency, dependency)
			}
			if !waiting[migrationID][dependency] {
				waiting[migrationID][dependency] = true
				dependents[dependency] = append(dependents[dependency], migrationID)
			}
		}
	}

	var ready []string
	for migrationID, dependencies := range waiting {
		if len(dependencies) == 0 {
			ready = append(ready, migrationID)
		}
	}
	ordered := make([]flam.IMigration, 0, len(byID))
	for len(ready) > 0 {
		slices.Sort(ready)
		next := ready[0]
		ready = ready[1:]
		ordered = append(ordered, byID[next])
		delete(waiting, next)
		for _, dependent := range dependents[next] {
			delete(waiting[dependent], next)
			if len(waiting[dependent]) == 0 {
				ready = append(ready, dependent)
			}
		}
	}

	if len(waiting) > 0 {
		cycle := make([]string, 0, len(waiting))
		for migrationID := range waiting {
			cycle = append(cycle, migrationID)
		}
		slices.Sort(cycle)
		return nil, newErrMigrator(ErrDependencyCycle, migratorNewMessage, id, group).
			With(ErrorFieldMigrations, cycle)
	}
	return ordered, nil
}

// Messages of the errors of the migrators.
const (
	migratorNewMessage  = "unable to create migrator"
	migratorListMessage = "unable to list migrations"
	migratorUpMessage   = "unable to apply migrations"
	migratorDownMessage = "unable to revert migrations"
)
