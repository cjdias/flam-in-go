package cache

import (
	"errors"
)

// The errors wrapped by the errors of the drivers when caching an object
// fails, whatever the driver, so that the callers of an adaptor can tell
// them apart.
var (
	// ErrKey is the error wrapped by the error of a key generator, and so
	// of an adaptor, when the key of an object cannot be generated.
	ErrKey = errors.New("unable to generate cache key")

	// ErrSerialize is the error wrapped by the error of a serializer, and
	// so of an adaptor, when an object cannot be serialized.
	ErrSerialize = errors.New("unable to serialize cache value")

	// ErrDeserialize is the error wrapped by the error of a serializer,
	// and so of an adaptor, when a cached value cannot be deserialized
	// into an object.
	ErrDeserialize = errors.New("unable to deserialize cache value")

	// ErrStorage is the error wrapped, along with the error of the store,
	// by the error of an adaptor when its store fails.
	ErrStorage = errors.New("cache storage failure")
)
