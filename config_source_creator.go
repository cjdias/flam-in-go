package flam

type ConfigSourceCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (ConfigSource, error)
}
