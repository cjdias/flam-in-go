package cache

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
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

// The errors of the [Loader].
var (
	// ErrNotFound is the error the load function of a [Loader] returns,
	// wrapped or as is, when the object it loads does not exist, so that
	// the loader can remember it, with [WithNegativeTTL], and its callers
	// can tell it apart.
	ErrNotFound = errors.New("cache object not found")

	// ErrNilAdaptor and ErrNilLoad are the errors wrapped by the error
	// returned from [NewLoader] when the adaptor or the load function is
	// nil.
	ErrNilAdaptor = errors.New("nil cache loader adaptor")
	ErrNilLoad    = errors.New("nil cache loader load function")

	// ErrNotKeyed is the error wrapped by the error returned from
	// [NewLoader] when the adaptor does not tell the keys of its objects,
	// as a [flam.ICacheKeyedAdaptor] does, and no [WithKeyGenerator]
	// option gives a key generator.
	ErrNotKeyed = errors.New("cache loader adaptor tells no keys")

	// ErrInvalidOption is the error wrapped by the error returned from
	// [NewLoader] when an option holds an invalid value.
	ErrInvalidOption = errors.New("invalid cache loader option")

	// ErrNilObject is the error wrapped by the error returned from
	// [Loader.Get] when the object is nil.
	ErrNilObject = errors.New("nil cache loader object")

	// ErrLoadPanic is the error wrapped by the error returned from
	// [Loader.Get] when the load function panicked.
	ErrLoadPanic = errors.New("cache loader load function panicked")
)

// newErrLoader returns the error wrapping the cause with the message, and
// the ErrorFieldID context field of the adaptor, and the ErrorFieldKey
// one when the key is known.
func newErrLoader(
	cause error,
	message string,
	id string,
	key string,
) *flam.Error {
	e := flam.NewWrappedError(cause, message).
		With(ErrorFieldID, id)
	if key != "" {
		e = e.With(ErrorFieldKey, key)
	}
	return e
}

// newErrInvalidOption returns the error wrapping ErrInvalidOption with
// the ErrorFieldID context field of the adaptor and the ErrorFieldOption
// one.
func newErrInvalidOption(
	id string,
	option string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidOption, loaderNewMessage).
		With(ErrorFieldID, id).
		With(ErrorFieldOption, option)
}
