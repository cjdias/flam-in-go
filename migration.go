package flam

type Migration interface {
	Group() string
	Version() string
	Description() string
	Up(connection DatabaseConnection) error
	Down(connection DatabaseConnection) error
}
