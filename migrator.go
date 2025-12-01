package flam

type Migrator interface {
	List() ([]MigrationInfo, error)
	Current() (*MigrationInfo, error)
	CanUp() bool
	CanDown() bool
	Up() error
	UpAll() error
	Down() error
	DownAll() error
}
