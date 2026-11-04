package migration

import (
	"io/fs"
	"slices"
	"strings"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewMigration returns the [flam.IMigration] of the id, description, group
// and dependencies, applied by up and reverted by down. A nil up applies
// nothing, so that the migration is only recorded, while a nil down makes
// the migration irreversible: its Down method fails with ErrIrreversible.
func NewMigration(
	id string,
	description string,
	group string,
	dependencies []string,
	up func(connection flam.IDatabaseConnection) error,
	down func(connection flam.IDatabaseConnection) error,
) flam.IMigration {
	return &migrationImpl{
		id:           id,
		description:  description,
		group:        group,
		dependencies: slices.Clone(dependencies),
		up:           up,
		down:         down,
	}
}

// NewFileMigration returns the [flam.IMigration] of the id, description,
// group and dependencies, running the SQL of the file of files at name
// followed by [UpFileSuffix] to apply it, and by [DownFileSuffix] to
// revert it, such as "sql/001_users.up.sql" for the name
// "sql/001_users", typically from an embed.FS. A file holding nothing but
// spaces runs nothing, and a missing file fails with ErrFile.
//
// A file may hold several statements when the database driver runs them
// in one call, as the SQLite and PostgreSQL ones do, and the MySQL one
// does with the "multiStatements" parameter.
func NewFileMigration(
	id string,
	description string,
	group string,
	dependencies []string,
	files fs.FS,
	name string,
) flam.IMigration {
	run := func(suffix string) func(flam.IDatabaseConnection) error {
		return func(connection flam.IDatabaseConnection) error {
			path := name + suffix
			content, err := fs.ReadFile(files, path)
			if err != nil {
				return newErrFile(migrationFileMessage, id, path, err)
			}
			if strings.TrimSpace(string(content)) == "" {
				return nil
			}
			if err := connection.Exec(string(content)).Error; err != nil {
				return newErrFile(migrationFileMessage, id, path, err)
			}
			return nil
		}
	}
	return NewMigration(id, description, group, dependencies, run(UpFileSuffix), run(DownFileSuffix))
}

// migrationImpl is the [flam.IMigration] returned by [NewMigration].
type migrationImpl struct {
	id           string
	description  string
	group        string
	dependencies []string
	up           func(connection flam.IDatabaseConnection) error
	down         func(connection flam.IDatabaseConnection) error
}

func (m *migrationImpl) Id() string {
	return m.id
}

func (m *migrationImpl) Description() string {
	return m.description
}

func (m *migrationImpl) Group() string {
	return m.group
}

// Dependencies returns a copy of the dependencies, which the caller may
// change.
func (m *migrationImpl) Dependencies() []string {
	return slices.Clone(m.dependencies)
}

func (m *migrationImpl) Up(
	connection flam.IDatabaseConnection,
) error {
	if m.up == nil {
		return nil
	}
	return m.up(connection)
}

func (m *migrationImpl) Down(
	connection flam.IDatabaseConnection,
) error {
	if m.down == nil {
		return flam.NewWrappedError(ErrIrreversible, migrationDownMessage).
			With(ErrorFieldMigration, m.id)
	}
	return m.down(connection)
}

// Messages of the errors of the migrations.
const (
	migrationFileMessage = "unable to run migration file"
	migrationDownMessage = "unable to revert migration"
)
