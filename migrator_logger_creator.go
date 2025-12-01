package flam

type MigratorLoggerCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (MigratorLogger, error)
}
