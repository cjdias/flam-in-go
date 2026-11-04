package redis

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the adaptor configuration, or its default,
	// does not hold a non-empty id.
	ErrInvalidField = errors.New("invalid redis cache adaptor field")

	// ErrSerializer is the error wrapped, along with the error of the
	// [flam.ICacheSerializerFactory], by the error returned from the
	// creator when the serializer cannot be taken from the factory.
	ErrSerializer = errors.New("unable to get redis cache adaptor serializer")

	// ErrKeyGenerator is the error wrapped, along with the error of the
	// [flam.ICacheKeyGeneratorFactory], by the error returned from the
	// creator when the key generator cannot be taken from the factory.
	ErrKeyGenerator = errors.New("unable to get redis cache adaptor key generator")

	// ErrConnection is the error wrapped, along with the error of the
	// [flam.IRedisConnectionFactory], by the error returned from the
	// creator when the connection cannot be taken from the factory, or
	// when no bundle provides the factory.
	ErrConnection = errors.New("unable to get redis cache adaptor connection")

	// ErrNilKeyGenerator, ErrNilSerializer and ErrNilConnection are the
	// errors wrapped by the error returned from [NewAdaptor] when the
	// key generator, the serializer or the connection is nil.
	ErrNilKeyGenerator = errors.New("nil redis cache adaptor key generator")
	ErrNilSerializer   = errors.New("nil redis cache adaptor serializer")
	ErrNilConnection   = errors.New("nil redis cache adaptor connection")

	// ErrInvalidTTL is the error wrapped by the error returned from
	// SetEphemeral when the ttl is not positive.
	ErrInvalidTTL = errors.New("invalid redis cache adaptor ttl")
)

// errNoConnections is the cause of the ErrConnection errors when no
// bundle provides the redis connection factory.
var errNoConnections = errors.New("no redis connection factory")

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

// newErrNil returns the error wrapping the sentinel of a nil argument, or
// of an invalid ttl, with the message and the ErrorFieldID context field.
func newErrNil(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrStorage returns the error wrapping cache.ErrStorage, joined with
// the error of the connection, with the message and the ErrorFieldID
// context field.
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
