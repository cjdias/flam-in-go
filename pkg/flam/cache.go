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
