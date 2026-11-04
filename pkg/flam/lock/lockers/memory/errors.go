package memory

import (
	"errors"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the locker configuration, or its default,
	// does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid memory locker field")

	// ErrClock is the error wrapped by the error returned from the creator
	// when no bundle provides the
	// [github.com/cjdias/flam-in-go/pkg/flam.IClock].
	ErrClock = errors.New("unable to get memory locker clock")
)
