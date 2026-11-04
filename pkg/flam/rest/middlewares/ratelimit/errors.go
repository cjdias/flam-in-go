package ratelimit

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the middleware configuration, or its
	// default, does not hold a value of the expected type: a finite rate
	// over 0, a burst and a maximum number of buckets of 1 or more, a
	// trusted proxy header string and an IPv6 prefix length from 0 to 128.
	ErrInvalidField = errors.New("invalid ratelimit rest middleware field")

	// ErrRateLimited is the error of the 429 envelope answering a request
	// finding the bucket of its client empty.
	ErrRateLimited = errors.New("rate limit exceeded")
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
