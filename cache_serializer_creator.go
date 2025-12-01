package flam

type CacheSerializerCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (CacheSerializer, error)
}
