package flam

type redisCacheAdaptorCreator struct {
	config                 Config
	keyGeneratorFactory    CacheKeyGeneratorFactory
	serializerFactory      CacheSerializerFactory
	redisConnectionFactory RedisConnectionFactory
}

func newRedisCacheAdaptorCreator(
	config Config,
	keyGeneratorFactory CacheKeyGeneratorFactory,
	serializerFactory CacheSerializerFactory,
	redisConnectionFactory RedisConnectionFactory,
) CacheAdaptorCreator {
	return &redisCacheAdaptorCreator{
		config:                 config,
		keyGeneratorFactory:    keyGeneratorFactory,
		serializerFactory:      serializerFactory,
		redisConnectionFactory: redisConnectionFactory,
	}
}

func (redisCacheAdaptorCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == CacheAdaptorDriverRedis
}

func (creator redisCacheAdaptorCreator) Create(
	config Bag,
) (CacheAdaptor, error) {
	keyGeneratorId := config.String("key_generator_id", creator.config.String(PathCacheDefaultKeyGeneratorId))
	serializerId := config.String("serializer_id", creator.config.String(PathCacheDefaultSerializerId))
	connectionId := config.String("connection_id")

	switch {
	case keyGeneratorId == "":
		return nil, newErrInvalidResourceConfig("redisCacheAdaptor", "key_generator_id", config)
	case serializerId == "":
		return nil, newErrInvalidResourceConfig("redisCacheAdaptor", "serializer_id", config)
	case connectionId == "":
		return nil, newErrInvalidResourceConfig("redisCacheAdaptor", "connection_id", config)
	}

	keyGenerator, e := creator.keyGeneratorFactory.Get(keyGeneratorId)
	if e != nil {
		return nil, e
	}

	serializer, e := creator.serializerFactory.Get(serializerId)
	if e != nil {
		return nil, e
	}

	connection, e := creator.redisConnectionFactory.Get(connectionId)
	if e != nil {
		return nil, e
	}

	return newRedisCacheAdaptor(
		keyGenerator,
		serializer,
		connection), nil
}
