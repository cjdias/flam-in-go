package flam

type CacheAdaptorCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (CacheAdaptor, error)
}
