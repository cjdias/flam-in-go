package flam

import (
	"context"
	"time"
)

// ICacheSerializer turns the objects stored by an [ICacheAdaptor] into
// strings and back. It is held by an [ICacheSerializerFactory].
type ICacheSerializer interface {
	IResource

	// Serialize returns the string of the object.
	Serialize(obj any) (string, error)

	// Deserialize decodes the string into the object, a non-nil pointer.
	Deserialize(data string, obj any) error
}

// ICacheSerializerCreator creates the cache serializers of a driver, for
// the [ICacheSerializerFactory].
type ICacheSerializerCreator = IResourceCreator[ICacheSerializer]

// ICacheSerializerFactory creates and holds the cache serializers, one per
// id.
type ICacheSerializerFactory = IFactory[ICacheSerializer]

// ICacheKeyGenerator returns the key an [ICacheAdaptor] stores an object
// under, typically from its identity, so that the objects of equal
// identity share it. It is held by an [ICacheKeyGeneratorFactory].
type ICacheKeyGenerator interface {
	IResource

	// Generate returns the key of the object.
	Generate(obj any) (string, error)
}

// ICacheKeyGeneratorCreator creates the cache key generators of a driver,
// for the [ICacheKeyGeneratorFactory].
type ICacheKeyGeneratorCreator = IResourceCreator[ICacheKeyGenerator]

// ICacheKeyGeneratorFactory creates and holds the cache key generators,
// one per id.
type ICacheKeyGeneratorFactory = IFactory[ICacheKeyGenerator]

// ICacheAdaptor caches objects in a store, each under the key of its
// [ICacheKeyGenerator], as the string of its [ICacheSerializer]. The
// objects are pointers, those given to Get being filled with the cached
// values, so that an object holding only its identity, such as an id,
// fetches the whole cached object. Every method handles its objects in
// one go, and does nothing for none. The context bounds the calls to the
// store, a nil one being replaced with [context.Background]. It is held
// by an [ICacheAdaptorFactory].
type ICacheAdaptor interface {
	IResource

	// Has reports, for each object, whether the cache holds a value under
	// its key.
	Has(ctx context.Context, objects ...any) ([]bool, error)

	// Get sets each object to the value the cache holds under its key,
	// replacing the whole value it points to, and returns, for each
	// object, the object when set, or nil when the cache holds no value
	// under its key, leaving it unchanged. On failure, no object is
	// changed.
	Get(ctx context.Context, objects ...any) ([]any, error)

	// Set stores the objects, with no expiration.
	Set(ctx context.Context, objects ...any) error

	// SetEphemeral stores the objects, expiring after the ttl, which must
	// be positive.
	SetEphemeral(ctx context.Context, ttl time.Duration, objects ...any) error

	// Del removes the values held under the keys of the objects, if any.
	Del(ctx context.Context, objects ...any) error
}

// ICacheAdaptorCreator creates the cache adaptors of a driver, for the
// [ICacheAdaptorFactory].
type ICacheAdaptorCreator = IResourceCreator[ICacheAdaptor]

// ICacheAdaptorFactory creates and holds the cache adaptors, one per id.
type ICacheAdaptorFactory = IFactory[ICacheAdaptor]

// ICacheKeyedAdaptor is an [ICacheAdaptor] that also tells the keys it
// stores its objects under, and removes the values by key, for the code
// working on the keys rather than on the objects: the
// [github.com/cjdias/flam-in-go/pkg/flam/cache.Loader] deduplicates its
// loads by key, and the adaptors of
// [github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/invalidating] send
// the keys to remove to the other processes. The memory, redis and
// invalidating adaptors implement it.
type ICacheKeyedAdaptor interface {
	ICacheAdaptor

	// Keys returns the key of each object, the one its other methods
	// generate, failing as they do when one cannot be generated.
	Keys(objects ...any) ([]string, error)

	// DelKeys removes the values held under the keys, if any.
	DelKeys(ctx context.Context, keys ...string) error
}

// ICacheAdaptorWrapperCreator is an [ICacheAdaptorCreator] whose adaptors
// wrap other adaptors of the [ICacheAdaptorFactory] holding them. As the
// container creates the factory with its creators, such a creator cannot
// take the factory from the container: the factory created by the bundle
// of [github.com/cjdias/flam-in-go/pkg/flam/cache] hands itself to every
// creator of its group implementing this interface, once created, before
// creating any adaptor.
type ICacheAdaptorWrapperCreator interface {
	ICacheAdaptorCreator

	// UseFactory gives the creator the factory it takes the wrapped
	// adaptors from.
	UseFactory(factory ICacheAdaptorFactory)
}

// ICacheLoader is the interface of a
// [github.com/cjdias/flam-in-go/pkg/flam/cache.Loader], for code reading
// objects of type T through the cache, without depending on the Loader
// type itself, so that tests can fake it.
type ICacheLoader[T any] interface {
	// Get sets the object, holding its identity, to its cached value, or
	// else loads it and caches it. See
	// [github.com/cjdias/flam-in-go/pkg/flam/cache.Loader.Get].
	Get(ctx context.Context, obj *T) error
}
