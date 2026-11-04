package kafka

import (
	"errors"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the broker configuration, or its default, does
// not hold a value of the expected type and range.
var ErrInvalidField = errors.New("invalid kafka broker field")
