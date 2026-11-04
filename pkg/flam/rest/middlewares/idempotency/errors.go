package idempotency

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a non-empty
	// cache id and header name, a list of at least one non-empty method, a
	// required bool, ttl and lock ttl durations of more than 0, a maximum
	// body size of 0 or more and a list of non-empty header names.
	ErrInvalidField = errors.New("invalid idempotency rest middleware field")

	// ErrNoFactory is the error wrapped by the error returned from the
	// creator when the container provides no
	// [flam.ICacheAdaptorFactory], as the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/cache] is not registered.
	ErrNoFactory = errors.New("no cache adaptor factory")

	// ErrCacheAdaptor is the error wrapped, along with the error of the
	// cache adaptor factory, by the error returned from the creator when
	// the cache adaptor of the middleware cannot be taken.
	ErrCacheAdaptor = errors.New("unable to get idempotency cache adaptor")

	// ErrMissingKey is the error of the 400 envelope answering a request
	// without idempotency key, when required.
	ErrMissingKey = errors.New("missing idempotency key")

	// ErrInvalidKey is the error of the 400 envelope answering a request
	// whose idempotency key is longer than [MaxKeyLength] bytes.
	ErrInvalidKey = errors.New("invalid idempotency key")

	// ErrReadBody is the error of the envelope answering a request whose
	// body cannot be read, of status 413 when longer than allowed, by a
	// [net/http.MaxBytesReader], and 400 otherwise.
	ErrReadBody = errors.New("unable to read request body")

	// ErrInFlight is the error of the 409 envelope answering a request
	// whose idempotency key is the one of a request still in flight.
	ErrInFlight = errors.New("request with the same idempotency key in flight")

	// ErrKeyReused is the error of the 422 envelope answering a request
	// whose idempotency key is the one of a request of another body.
	ErrKeyReused = errors.New("idempotency key reused with another request body")

	// ErrCache is the error of the 503 envelope answering a request when
	// the cache adaptor fails, as the request cannot then be made
	// idempotent.
	ErrCache = errors.New("idempotency cache unavailable")
)

// newErrCreate returns the error wrapping the sentinel with the message and
// the ErrorFieldID and ErrorFieldField context fields.
func newErrCreate(
	sentinel error,
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrCache returns the error wrapping the sentinel, joined with the
// cause when not nil, with the message and the ErrorFieldID and
// ErrorFieldCache context fields.
func newErrCache(
	sentinel error,
	message string,
	id string,
	cache string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id).
		With(ErrorFieldCache, cache)
}
