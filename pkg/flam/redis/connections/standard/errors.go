package standard

import (
	"errors"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the connection configuration, or its default,
// does not hold a value of the expected type and range.
var ErrInvalidField = errors.New("invalid standard redis connection field")
