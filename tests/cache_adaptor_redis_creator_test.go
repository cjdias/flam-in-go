package tests

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_RedisCacheAdaptorCreator(t *testing.T) {
	t.Run("should ignore config without/empty key_generator_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "",
				"serializer_id":    "my_serializer_",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty serializer_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should ignore config without/empty connection_id field", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    ""}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrInvalidResourceConfig)
		}))
	})

	t.Run("should return key generator creation error", func(t *testing.T) {
		config := flam.Bag{}
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return serializer creation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_key_generator": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should return redis connection creation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_key_generator": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.Nil(t, adaptor)
			require.ErrorIs(t, e, flam.ErrUnknownResource)
		}))
	})

	t.Run("should correctly generate the adaptor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_key_generator": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.NotNil(t, adaptor)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the adaptor if default key generator if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheDefaultKeyGeneratorId, "my_key_generator")
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_key_generator": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.NotNil(t, adaptor)
			require.NoError(t, e)
		}))
	})

	t.Run("should correctly generate the adaptor if default serializer if not given", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := flam.Bag{}
		_ = config.Set(flam.PathCacheDefaultSerializerId, "my_serializer")
		_ = config.Set(flam.PathRedisConnections, flam.Bag{
			"my_connection": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheKeyGenerators, flam.Bag{
			"my_key_generator": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheSerializers, flam.Bag{
			"my_serializer": flam.Bag{
				"driver": "mock"}})
		_ = config.Set(flam.PathCacheAdaptors, flam.Bag{
			"my_adaptor": flam.Bag{
				"driver":           flam.CacheAdaptorDriverRedis,
				"key_generator_id": "my_key_generator",
				"serializer_id":    "my_serializer",
				"connection_id":    "my_connection"}})

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		require.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			adaptor, e := factory.Get("my_adaptor")
			require.NotNil(t, adaptor)
			require.NoError(t, e)
		}))
	})
}
