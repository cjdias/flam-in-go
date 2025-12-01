package flam

import (
	"io"

	"go.uber.org/dig"
)

type RedisConnectionFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (RedisConnection, error)
	Store(id string, connection RedisConnection) error
	Remove(id string) error
	RemoveAll() error
}

type redisConnectionFactoryArgs struct {
	dig.In

	Creators      []RedisConnectionCreator `group:"flam.redis.connections.creator"`
	FactoryConfig FactoryConfig
}

func newRedisConnectionFactory(
	args redisConnectionFactoryArgs,
) (RedisConnectionFactory, error) {
	var creators []FactoryResourceCreator[RedisConnection]
	for _, creator := range args.Creators {
		creators = append(creators, creator)
	}

	return NewFactory(
		creators,
		args.FactoryConfig,
		DriverFactoryConfigValidator("RedisConnection"),
		PathRedisConnections)
}
