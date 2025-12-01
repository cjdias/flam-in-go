package flam

type DatabaseDialectCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (DatabaseDialect, error)
}
