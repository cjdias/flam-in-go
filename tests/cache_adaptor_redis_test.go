package tests

import (
	"errors"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cjdias/flam-in-go"
	"github.com/cjdias/flam-in-go/tests/mocks"
)

func Test_RedisCacheAdaptor_Has(t *testing.T) {
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

	newIntCmd := func(val int64) *redis.IntCmd {
		res := &redis.IntCmd{}
		res.SetVal(val)
		return res
	}

	t.Run("should return error if no object have been passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has()
			assert.Nil(t, res)
			assert.ErrorIs(t, e, flam.ErrMissingCacheObject)
		}))
	})

	t.Run("should return key generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		expectedErr := errors.New("expected error")

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("", expectedErr)
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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has(object1, object2)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return multi key existence check error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		response := redis.IntCmd{}
		response.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1", "key2").Return(&response)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has(object1, object2)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return list of trues if all the keys exists", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		response := redis.IntCmd{}
		response.SetVal(2)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1", "key2").Return(&response)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has(object1, object2)
			assert.ElementsMatch(t, []bool{true, true}, res)
			assert.NoError(t, e)
		}))
	})

	t.Run("should return individual key check error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		errIntCmd := &redis.IntCmd{}
		errIntCmd.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1", "key2", "key3").Return(newIntCmd(2))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1").Return(newIntCmd(1))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key2").Return(newIntCmd(0))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key3").Return(errIntCmd)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has(object1, object2, object3)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return list of trues on the existing keys", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		errIntCmd := &redis.IntCmd{}
		errIntCmd.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1", "key2", "key3").Return(newIntCmd(2))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key1").Return(newIntCmd(1))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key2").Return(newIntCmd(0))
		redisConnectionMock.EXPECT().Exists(gomock.Any(), "key3").Return(newIntCmd(1))
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Has(object1, object2, object3)
			assert.ElementsMatch(t, []bool{true, false, true}, res)
			assert.NoError(t, e)
		}))
	})
}

func Test_RedisCacheAdaptor_Get(t *testing.T) {
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

	t.Run("should return error if no object have been passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Get()
			assert.Nil(t, res)
			assert.ErrorIs(t, e, flam.ErrMissingCacheObject)
		}))
	})

	t.Run("should return key generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}

		expectedErr := errors.New("expected error")
		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("", expectedErr)
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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Get(object1, object2)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return multi get result error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		result := redis.SliceCmd{}
		result.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().MGet(gomock.Any(), "key1", "key2", "key3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Get(object1, object2, object3)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return deserialization result error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		expectedErr := errors.New("expected error")
		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Deserialize("value1", object1).Return(nil)
		cacheSerializerMock.EXPECT().Deserialize("value2", object2).Return(nil)
		cacheSerializerMock.EXPECT().Deserialize("value3", object3).Return(expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		result := redis.SliceCmd{}
		result.SetVal([]interface{}{"value1", "value2", "value3"})

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().MGet(gomock.Any(), "key1", "key2", "key3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Get(object1, object2, object3)
			assert.Nil(t, res)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return multi retrieval values", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Deserialize("value1", object1).Return(nil)
		cacheSerializerMock.EXPECT().Deserialize("value2", object2).Return(nil)
		cacheSerializerMock.EXPECT().Deserialize("value3", object3).Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		result := redis.SliceCmd{}
		result.SetVal([]interface{}{"value1", "value2", "value3"})

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().MGet(gomock.Any(), "key1", "key2", "key3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			res, e := got.Get(object1, object2, object3)
			assert.ElementsMatch(t, []any{object1, object2, object3}, res)
			assert.NoError(t, e)
		}))
	})
}

func Test_RedisCacheAdaptor_Set(t *testing.T) {
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

	t.Run("should return error if no object have been passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Set()
			assert.ErrorIs(t, e, flam.ErrMissingCacheObject)
		}))
	})

	t.Run("should return key generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}

		expectedErr := errors.New("expected error")
		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("", expectedErr)
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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Set(object1, object2)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return serialization error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		expectedErr := errors.New("expected error")
		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object1).Return("value1", nil)
		cacheSerializerMock.EXPECT().Serialize(object2).Return("value2", nil)
		cacheSerializerMock.EXPECT().Serialize(object3).Return("", expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Set(object1, object2, object3)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return connection error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object1).Return("value1", nil)
		cacheSerializerMock.EXPECT().Serialize(object2).Return("value2", nil)
		cacheSerializerMock.EXPECT().Serialize(object3).Return("value3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		result := redis.StatusCmd{}
		result.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().MSet(gomock.Any(), "key1", "value1", "key2", "value2", "key3", "value3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Set(object1, object2, object3)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return no error on valid storing", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object1).Return("value1", nil)
		cacheSerializerMock.EXPECT().Serialize(object2).Return("value2", nil)
		cacheSerializerMock.EXPECT().Serialize(object3).Return("value3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		result := redis.StatusCmd{}

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().MSet(gomock.Any(), "key1", "value1", "key2", "value2", "key3", "value3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Set(object1, object2, object3)
			assert.NoError(t, e)
		}))
	})
}

func Test_RedisCacheAdaptor_SetEphemeral(t *testing.T) {
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

	t.Run("should return error if no object have been passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.SetEphemeral(time.Second)
			assert.ErrorIs(t, e, flam.ErrMissingCacheObject)
		}))
	})

	t.Run("should return key generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object := struct{}{}

		expectedErr := errors.New("expected error")
		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object).Return("", expectedErr)
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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.SetEphemeral(time.Second, object)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return serialization error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object).Return("value", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		expectedErr := errors.New("expected error")
		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object).Return("", expectedErr)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.SetEphemeral(time.Second, object)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return communication error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object).Return("key", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object).Return("value", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		result := redis.StatusCmd{}
		result.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Set(gomock.Any(), "key", "value", time.Second).Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.SetEphemeral(time.Second, object)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return no error if all object have been stored", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		cacheSerializerMock.EXPECT().Serialize(object1).Return("value1", nil)
		cacheSerializerMock.EXPECT().Serialize(object2).Return("value2", nil)
		cacheSerializerMock.EXPECT().Serialize(object3).Return("value3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		result := redis.StatusCmd{}

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Set(gomock.Any(), "key1", "value1", time.Second).Return(&result)
		redisConnectionMock.EXPECT().Set(gomock.Any(), "key2", "value2", time.Second).Return(&result)
		redisConnectionMock.EXPECT().Set(gomock.Any(), "key3", "value3", time.Second).Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.SetEphemeral(time.Second, object1, object2, object3)
			assert.NoError(t, e)
		}))
	})
}

func Test_RedisCacheAdaptor_Del(t *testing.T) {
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

	t.Run("should return error if no object have been passed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Del()
			assert.ErrorIs(t, e, flam.ErrMissingCacheObject)
		}))
	})

	t.Run("should return key generation error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object := struct{}{}

		expectedErr := errors.New("expected error")
		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object).Return("", expectedErr)
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

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Del(object)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return communication error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object).Return("key", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		expectedErr := errors.New("expected error")
		result := redis.IntCmd{}
		result.SetErr(expectedErr)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Del(gomock.Any(), "key").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Del(object)
			assert.ErrorIs(t, e, expectedErr)
		}))
	})

	t.Run("should return no error if all elements have been deleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		app := flam.NewApplication(config)
		defer func() { _ = app.Close() }()

		object1 := struct{}{}
		object2 := struct{}{}
		object3 := struct{}{}

		cacheKeyGeneratorMock := mocks.NewMockCacheKeyGenerator(ctrl)
		cacheKeyGeneratorMock.EXPECT().Generate(object1).Return("key1", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object2).Return("key2", nil)
		cacheKeyGeneratorMock.EXPECT().Generate(object3).Return("key3", nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheKeyGeneratorFactory) {
			require.NoError(t, factory.Store("my_key_generator", cacheKeyGeneratorMock))
		}))

		cacheSerializerMock := mocks.NewMockCacheSerializer(ctrl)
		require.NoError(t, app.Container().Invoke(func(factory flam.CacheSerializerFactory) {
			require.NoError(t, factory.Store("my_serializer", cacheSerializerMock))
		}))

		result := redis.IntCmd{}
		result.SetVal(3)

		redisConnectionMock := mocks.NewMockRedisConnection(ctrl)
		redisConnectionMock.EXPECT().Del(gomock.Any(), "key1", "key2", "key3").Return(&result)
		redisConnectionMock.EXPECT().Close().Return(nil)
		require.NoError(t, app.Container().Invoke(func(factory flam.RedisConnectionFactory) {
			require.NoError(t, factory.Store("my_connection", redisConnectionMock))
		}))

		require.NoError(t, app.Boot())

		assert.NoError(t, app.Container().Invoke(func(factory flam.CacheAdaptorFactory) {
			got, e := factory.Get("my_adaptor")
			assert.NotNil(t, got)
			assert.NoError(t, e)

			e = got.Del(object1, object2, object3)
			assert.NoError(t, e)
		}))
	})
}
