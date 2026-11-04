// Package cache is the cache feature: the bundle returned from [NewBundle]
// provides the
// [github.com/cjdias/flam-in-go/pkg/flam.ICacheSerializerFactory], the
// [github.com/cjdias/flam-in-go/pkg/flam.ICacheKeyGeneratorFactory] and
// the [github.com/cjdias/flam-in-go/pkg/flam.ICacheAdaptorFactory] of an
// application, which create the serializers, key generators and adaptors
// described by the configuration at [SerializerConfigPath],
// [KeyGeneratorConfigPath] and [AdaptorConfigPath] with the creators
// provided in the [SerializerCreatorGroup], [KeyGeneratorCreatorGroup] and
// [AdaptorCreatorGroup] container groups.
//
// An adaptor names its serializer and key generator by their ids, in the
// [ConfigFieldSerializer] and [ConfigFieldKeyGenerator] fields, or takes
// their defaults, such as:
//
//	flam:
//	  redis:
//	    connections:
//	      main: {driver: flam.redis.connections.driver.standard}
//	  cache:
//	    adaptors:
//	      main: {driver: flam.cache.adaptors.driver.redis, connection: main}
//
// the serializer and the key generator taking their defaults, those of
// [github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json] and
// [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field].
//
// The errors of the drivers wrap [ErrKey], [ErrSerialize],
// [ErrDeserialize] or [ErrStorage] when caching an object fails.
//
// A [Loader], returned from [NewLoader], reads the objects through an
// adaptor as the cache-aside pattern does, loading the objects missing
// from the cache with a load function, once per key across its concurrent
// callers, and storing them, with an optional ttl and jitter, while
// optionally remembering the objects not found for a while:
//
//	users, err := cache.NewLoader(adaptor,
//		func(ctx context.Context, u *User) error { return repo.Load(ctx, u) },
//		cache.WithTTL(time.Minute), cache.WithJitter(10*time.Second))
//	...
//	u := &User{Id: 1}
//	err = users.Get(ctx, u)
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/cache/serializers],
// [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators] and
// [github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors]. They import this
// package for its paths, groups, fields and errors, so this package never
// imports them.
package cache
