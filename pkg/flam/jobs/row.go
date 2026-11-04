package jobs

import (
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
)

// Row is the gorm model of the [Table] table, holding a job of a queue.
// The times are stored in UTC, truncated to the microsecond.
type Row struct {
	// ID is the id of the job, increasing with each job enqueued.
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	// Queue is the id of the queue of the job.
	Queue string `gorm:"size:191;not null;uniqueIndex:idx_flam_jobs_key,priority:1;index:idx_flam_jobs_claim,priority:1"`

	// Kind is the kind of the job.
	Kind string `gorm:"size:191;not null"`

	// DedupKey is the deduplication key of the job, unique in its queue,
	// or nil.
	DedupKey *string `gorm:"size:191;uniqueIndex:idx_flam_jobs_key,priority:2"`

	// Payload is the JSON encoding of the payload of the job.
	Payload []byte `gorm:"not null"`

	// State is the state of the job, such as [StatePending].
	State string `gorm:"size:16;not null;index:idx_flam_jobs_claim,priority:2"`

	// Attempt is the number of attempts made, the one under way included.
	Attempt int `gorm:"not null"`

	// MaxAttempts is the number of attempts the job is allowed.
	MaxAttempts int `gorm:"not null"`

	// RunAt is the time before which the pending job does not run.
	RunAt time.Time `gorm:"not null;index:idx_flam_jobs_claim,priority:3"`

	// LockedUntil is the time the lease of the running job ends at, or nil.
	LockedUntil *time.Time

	// Token identifies the claim of the job, changed by each claim, so
	// that the worker of an expired lease cannot record an outcome.
	Token string `gorm:"size:64;not null"`

	// LastError is the error of the last failed attempt, or "".
	LastError string `gorm:"type:text"`

	// EnqueuedAt is the time the job was enqueued at.
	EnqueuedAt time.Time `gorm:"not null"`

	// FinishedAt is the time the job was completed at, or made dead at,
	// or nil.
	FinishedAt *time.Time `gorm:"index"`
}

// TableName returns [Table], the table of the model.
func (Row) TableName() string {
	return Table
}

// Migrate creates or updates the [Table] table of the database, as the
// queues do when created unless their [ConfigFieldMigrate] field is false,
// failing with the error wrapping [ErrMigrate].
func Migrate(
	db *gorm.DB,
) error {
	if err := db.AutoMigrate(&Row{}); err != nil {
		return newErr(ErrMigrate, migrateMessage, err)
	}
	return nil
}

// NewMigration returns the [flam.IMigration] of the id, group and
// dependencies creating the [Table] table with [Migrate] and dropping it,
// for the applications migrating their databases with
// [github.com/cjdias/flam-in-go/pkg/flam/migration], whose queues then set
// [ConfigFieldMigrate] to false.
func NewMigration(
	id string,
	group string,
	dependencies ...string,
) flam.IMigration {
	return migration.NewMigration(id, migrationDescription, group, dependencies,
		func(connection flam.IDatabaseConnection) error {
			return Migrate(connection.Gorm())
		},
		func(connection flam.IDatabaseConnection) error {
			return connection.Migrator().DropTable(Table)
		})
}

// Messages and descriptions of Migrate and NewMigration.
const (
	migrateMessage       = "unable to migrate job table"
	migrationDescription = "create the " + Table + " table"
)
