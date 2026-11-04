package migration

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// migrator creator when a field of the migrator configuration, or its
	// default, does not hold an id: a non-empty connection id, a non-empty
	// group, and a logger adaptor id.
	ErrInvalidField = errors.New("invalid migrator field")

	// ErrConnection is the error wrapped, along with the error of the
	// [flam.IDatabaseConnectionFactory], by the error returned from the
	// migrator creator when the connection cannot be taken from the
	// factory.
	ErrConnection = errors.New("unable to get migrator connection")

	// ErrLogger is the error wrapped, along with the error of the
	// [flam.IMigrationLoggerAdaptorFactory], by the error returned from the
	// migrator creator when the logger adaptor cannot be taken from the
	// factory.
	ErrLogger = errors.New("unable to get migrator logger adaptor")

	// ErrNilConnection is the error wrapped by the error returned from
	// [NewMigrator] when the connection is nil.
	ErrNilConnection = errors.New("nil migrator connection")

	// ErrNilClock is the error wrapped by the error returned from
	// [NewMigrator] when the clock is nil.
	ErrNilClock = errors.New("nil migrator clock")

	// ErrNilMigration is the error wrapped by the error returned from
	// [NewMigrator], and from the migrator creator, when a migration is
	// nil.
	ErrNilMigration = errors.New("nil migration")

	// ErrForeignGroup is the error wrapped by the error returned from
	// [NewMigrator] when a migration is not of the group of the migrator.
	ErrForeignGroup = errors.New("migration of another group")

	// ErrDuplicateMigration is the error wrapped by the error returned
	// from [NewMigrator] when two migrations have the same id.
	ErrDuplicateMigration = errors.New("duplicate migration id")

	// ErrUnknownDependency is the error wrapped by the error returned from
	// [NewMigrator] when a migration depends on an id that is not the one
	// of a migration of the group.
	ErrUnknownDependency = errors.New("unknown migration dependency")

	// ErrDependencyCycle is the error wrapped by the error returned from
	// [NewMigrator] when migrations depend on one another in a cycle.
	ErrDependencyCycle = errors.New("migration dependency cycle")

	// ErrRecords is the error wrapped, along with the error of the
	// database, by the errors returned when the table of the applied
	// migrations cannot be created, read or written.
	ErrRecords = errors.New("unable to access the migration records")

	// ErrNoPending is the error wrapped by the error returned from the Up
	// method of a migrator when every migration is applied.
	ErrNoPending = errors.New("no pending migration")

	// ErrNoInstalled is the error wrapped by the error returned from the
	// Down method of a migrator when no migration is applied.
	ErrNoInstalled = errors.New("no installed migration")

	// ErrUnknownMigration is the error wrapped by the error returned from
	// the Down method of a migrator when the last migration applied is not
	// one of its migrations anymore.
	ErrUnknownMigration = errors.New("unknown installed migration")

	// ErrUp is the error wrapped, along with the error of the migration or
	// of the database, by the error returned when a migration cannot be
	// applied.
	ErrUp = errors.New("unable to apply migration")

	// ErrDown is the error wrapped, along with the error of the migration
	// or of the database, by the error returned when a migration cannot be
	// reverted.
	ErrDown = errors.New("unable to revert migration")

	// ErrIrreversible is the error wrapped by the error returned from the
	// Down method of a migration returned by [NewMigration] without down
	// function.
	ErrIrreversible = errors.New("irreversible migration")

	// ErrFile is the error wrapped, along with the error of the file
	// system or of the database, by the error returned when the SQL file
	// of a migration returned by [NewFileMigration] cannot be read or run.
	ErrFile = errors.New("unable to run migration file")
)

// newErrField returns the error wrapping the sentinel with the message and
// the ErrorFieldID context field and the field context field.
func newErrField(
	sentinel error,
	message string,
	id string,
	field string,
	val any,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(field, val)
}

// newErrJoined returns the error wrapping the sentinel, joined with err,
// with the message and the ErrorFieldID context field and the field
// context field.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	field string,
	val any,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, err), message).
		With(ErrorFieldID, id).
		With(field, val)
}

// newErrMigrator returns the error wrapping the sentinel with the message
// and the ErrorFieldID and ErrorFieldGroup context fields.
func newErrMigrator(
	sentinel error,
	message string,
	id string,
	group string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldGroup, group)
}

// newErrRecords returns the error wrapping ErrRecords, joined with the
// error of the database, with the message and the ErrorFieldID and
// ErrorFieldGroup context fields.
func newErrRecords(
	message string,
	id string,
	group string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrRecords, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldGroup, group)
}

// newErrMigration returns the error wrapping the sentinel, joined with err
// when not nil, with the message and the ErrorFieldID, ErrorFieldGroup and
// ErrorFieldMigration context fields.
func newErrMigration(
	sentinel error,
	message string,
	id string,
	group string,
	migration string,
	err error,
) *flam.Error {
	cause := sentinel
	if err != nil {
		cause = fmt.Errorf("%w: %w", sentinel, err)
	}
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldID, id).
		With(ErrorFieldGroup, group).
		With(ErrorFieldMigration, migration)
}

// newErrFile returns the error wrapping ErrFile, joined with err, with the
// message and the ErrorFieldMigration and ErrorFieldPath context fields.
func newErrFile(
	message string,
	migration string,
	path string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrFile, err), message).
		With(ErrorFieldMigration, migration).
		With(ErrorFieldPath, path)
}

// newErrBoot returns the error wrapping the error of a migrator when the
// application boots, with the message and the ErrorFieldID context field.
func newErrBoot(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
}
