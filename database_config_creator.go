package flam

type DatabaseConfigCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (DatabaseConfig, error)
}
