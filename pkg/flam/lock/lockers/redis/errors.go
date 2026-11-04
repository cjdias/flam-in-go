package redis

import (
	"errors"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the locker configuration, or its default,
	// does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid redis locker field")

	// ErrConnection is the error wrapped, along with the error of the
	// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], by
	// the error returned from the creator when the redis connection cannot
	// be taken from the factory, or when the container provides no
	// factory, as the redis bundle is not registered.
	ErrConnection = errors.New("unable to get redis locker connection")

	// ErrClock is the error wrapped by the error returned from the creator
	// when no bundle provides the
	// [github.com/cjdias/flam-in-go/pkg/flam.IClock].
	ErrClock = errors.New("unable to get redis locker clock")
)

// errNoFactory is the cause of the ErrConnection errors when the container
// provides no connection factory.
var errNoFactory = errors.New("no redis connection factory, register the redis bundle")
