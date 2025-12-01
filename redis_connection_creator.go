package flam

type RedisConnectionCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (RedisConnection, error)
}
