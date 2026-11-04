package outbox

import (
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/migration"
)

// Row is the gorm model of the [Table] table, holding a message of the
// outbox. The times set by the relays are stored in UTC, truncated to the
// microsecond.
type Row struct {
	// ID is the id of the message, increasing with each message added, so
	// that it orders the messages.
	ID uint64 `gorm:"primaryKey;autoIncrement"`

	// Topic is the topic to publish the message on.
	Topic string `gorm:"size:191;not null;index:idx_flam_outbox_topic,priority:1"`

	// Payload is the payload of the message.
	Payload []byte `gorm:"not null"`

	// Headers is the JSON encoding of the headers of the message, or ""
	// for none.
	Headers string `gorm:"type:text"`

	// CreatedAt is the time the message was added at.
	CreatedAt time.Time

	// Attempts is the number of failed attempts to publish the message.
	Attempts int `gorm:"not null"`

	// NextAttemptAt is the time before which the message failing to be
	// published is not published again, or nil.
	NextAttemptAt *time.Time

	// LockedUntil is the time the lease of the claimed message ends at, or
	// nil.
	LockedUntil *time.Time

	// Token identifies the claim of the message, changed by each claim, so
	// that the relay of an expired lease cannot record an outcome.
	Token string `gorm:"size:64;not null"`

	// LastError is the error of the last failed attempt, or "".
	LastError string `gorm:"type:text"`

	// SentAt is the time the message was published at, or nil.
	SentAt *time.Time `gorm:"index"`
}

// TableName returns [Table], the table of the model.
func (Row) TableName() string {
	return Table
}

// Migrate creates or updates the [Table] table of the database, as the
// relays do when created unless their [ConfigFieldMigrate] field is false,
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
// [github.com/cjdias/flam-in-go/pkg/flam/migration], whose relays then set
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
	migrateMessage       = "unable to migrate outbox table"
	migrationDescription = "create the " + Table + " table"
)
