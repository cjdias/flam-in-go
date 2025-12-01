package flam

type MigratorCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (Migrator, error)
}
