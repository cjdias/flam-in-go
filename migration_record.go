package flam

import "time"

type migrationRecord struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time `sql:"index"`

	Version     string
	Description string
}

func (migrationRecord) TableName() string {
	return "__migrations"
}
