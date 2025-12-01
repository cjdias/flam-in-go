package flam

import (
	"context"
	"time"
)

type redisCacheAdaptor struct {
	cacheKeyGenerator CacheKeyGenerator
	cageSerializer    CacheSerializer
	redisConnection   RedisConnection
}

func newRedisCacheAdaptor(
	cacheKeyGenerator CacheKeyGenerator,
	cacheSerializer CacheSerializer,
	redisConnection RedisConnection,
) CacheAdaptor {
	return &redisCacheAdaptor{
		cacheKeyGenerator: cacheKeyGenerator,
		cageSerializer:    cacheSerializer,
		redisConnection:   redisConnection,
	}
}

func (adaptor redisCacheAdaptor) Has(
	obj ...any,
) ([]bool, error) {
	if len(obj) == 0 {
		return nil, ErrMissingCacheObject
	}

	keys, e := adaptor.generateKeys(obj...)
	if e != nil {
		return nil, e
	}

	result := make([]bool, len(obj))

	res := adaptor.redisConnection.Exists(context.Background(), keys...)
	switch {
	case res.Err() != nil:
		return nil, res.Err()
	case res.Val() == int64(len(obj)):
		for i := range obj {
			result[i] = true
		}
		return result, nil
	}

	for i, key := range keys {
		res := adaptor.redisConnection.Exists(context.Background(), key)
		if res.Err() != nil {
			return nil, res.Err()
		}

		result[i] = res.Val() != 0
	}

	return result, nil
}

func (adaptor redisCacheAdaptor) Get(
	obj ...any,
) ([]any, error) {
	if len(obj) == 0 {
		return nil, ErrMissingCacheObject
	}

	keys, e := adaptor.generateKeys(obj...)
	if e != nil {
		return nil, e
	}

	res := adaptor.redisConnection.MGet(context.Background(), keys...)
	if res.Err() != nil {
		return nil, res.Err()
	}

	for i, v := range res.Val() {
		if e = adaptor.cageSerializer.Deserialize(v.(string), obj[i]); e != nil {
			return nil, e
		}
	}

	return obj, nil
}

func (adaptor redisCacheAdaptor) Set(
	obj ...any,
) error {
	if len(obj) == 0 {
		return ErrMissingCacheObject
	}

	keys, e := adaptor.generateKeys(obj...)
	if e != nil {
		return e
	}

	var values []interface{}
	for i, v := range obj {
		data, e := adaptor.cageSerializer.Serialize(v)
		if e != nil {
			return e
		}

		values = append(values, keys[i])
		values = append(values, data)
	}

	return adaptor.redisConnection.MSet(context.Background(), values...).Err()
}

func (adaptor redisCacheAdaptor) SetEphemeral(
	ttl time.Duration,
	obj ...any,
) error {
	if len(obj) == 0 {
		return ErrMissingCacheObject
	}

	for _, o := range obj {
		key, e := adaptor.cacheKeyGenerator.Generate(o)
		if e != nil {
			return e
		}

		data, e := adaptor.cageSerializer.Serialize(o)
		if e != nil {
			return e
		}

		if e := adaptor.redisConnection.Set(context.Background(), key, data, ttl).Err(); e != nil {
			return e
		}
	}

	return nil
}

func (adaptor redisCacheAdaptor) Del(
	obj ...any,
) error {
	if len(obj) == 0 {
		return ErrMissingCacheObject
	}

	keys, e := adaptor.generateKeys(obj...)
	if e != nil {
		return e
	}

	return adaptor.redisConnection.Del(context.Background(), keys...).Err()
}

func (adaptor redisCacheAdaptor) generateKeys(
	obj ...any,
) ([]string, error) {
	var keys []string
	for _, o := range obj {
		key, e := adaptor.cacheKeyGenerator.Generate(o)
		if e != nil {
			return nil, e
		}
		keys = append(keys, key)
	}

	return keys, nil
}
