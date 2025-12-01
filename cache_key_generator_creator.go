package flam

type CacheKeyGeneratorCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (CacheKeyGenerator, error)
}
