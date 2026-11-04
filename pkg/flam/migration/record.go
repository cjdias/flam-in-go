package migration

import (
	"time"
)

// record is the row of a migration applied by a migrator, in RecordTable.
// Sequence orders the migrations applied in a group, so that a migrator
// reverts the last one applied even when two share the same time.
type record struct {
	Group       string `gorm:"column:migration_group;primaryKey;size:191"`
	Id          string `gorm:"column:migration_id;primaryKey;size:191"`
	Sequence    uint64 `gorm:"not null"`
	Description string
	InstalledAt time.Time `gorm:"not null"`
}

// TableName returns RecordTable, the table of the records.
func (record) TableName() string {
	return RecordTable
}
