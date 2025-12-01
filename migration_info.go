package flam

import "time"

type MigrationInfo struct {
	Version     string
	Description string
	InstalledAt *time.Time
}
