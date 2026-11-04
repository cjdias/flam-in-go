package memory

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the adaptor configuration, or its default,
	// does not hold a value of the expected type: non-empty serializer and
	// key generator ids, a positive max entries and a non-negative sweep
	// period. [NewAdaptor] also returns it for a max entries that is not
	// positive or a negative sweep period.
	ErrInvalidField = errors.New("invalid memory cache adaptor field")

	// ErrSerializer is the error wrapped, along with the error of the
	// [flam.ICacheSerializerFactory], by the error returned from the
	// creator when the serializer cannot be taken from the factory.
	ErrSerializer = errors.New("unable to get memory cache adaptor serializer")

	// ErrKeyGenerator is the error wrapped, along with the error of the
	// [flam.ICacheKeyGeneratorFactory], by the error returned from the
	// creator when the key generator cannot be taken from the factory.
	ErrKeyGenerator = errors.New("unable to get memory cache adaptor key generator")

	// ErrClock is the error wrapped by the error returned from the creator
	// when no bundle provides the [flam.IClock].
	ErrClock = errors.New("unable to get memory cache adaptor clock")

	// ErrNilKeyGenerator, ErrNilSerializer and ErrNilClock are the errors
	// wrapped by the error returned from [NewAdaptor] when the key
	// generator, the serializer or the clock is nil.
	ErrNilKeyGenerator = errors.New("nil memory cache adaptor key generator")
	ErrNilSerializer   = errors.New("nil memory cache adaptor serializer")
	ErrNilClock        = errors.New("nil memory cache adaptor clock")

	// ErrInvalidTTL is the error wrapped by the error returned from
	// SetEphemeral when the ttl is not positive.
	ErrInvalidTTL = errors.New("invalid memory cache adaptor ttl")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// error of a factory, with the message and the ErrorFieldID context field
// and the field of the id taken from the factory.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	field string,
	value string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", sentinel, err), message).
		With(ErrorFieldID, id).
		With(field, value)
}

// newErrNil returns the error wrapping the sentinel of a nil argument, of
// an invalid ttl or of a missing clock, with the message and the
// ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrStorage returns the error wrapping cache.ErrStorage, joined with
// the error of the context, with the message and the ErrorFieldID context
// field.
func newErrStorage(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", cache.ErrStorage, err), message).
		With(ErrorFieldID, id)
}

// newErrObject returns the error wrapping the error of the key generator
// or the serializer for an object, with the message and the ErrorFieldID
// context field, and the cache.ErrorFieldKey one when the key is known.
func newErrObject(
	message string,
	id string,
	key string,
	err error,
) *flam.Error {
	e := flam.NewWrappedError(err, message).
		With(ErrorFieldID, id)
	if key != "" {
		e = e.With(cache.ErrorFieldKey, key)
	}
	return e
}
