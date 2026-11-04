package redis

import (
	"errors"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the broker configuration, or its default,
	// does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid redis broker field")

	// ErrConnection is the error wrapped, along with the error of the
	// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], by
	// the error returned from the creator when the redis connection cannot
	// be taken from the factory, or when the container provides no
	// factory, as the redis bundle is not registered.
	ErrConnection = errors.New("unable to get redis broker connection")
)
