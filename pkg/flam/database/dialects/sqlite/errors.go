package sqlite

import (
	"errors"
)

// ErrInvalidField is the error wrapped by the error returned from the
// creator when a field of the dialect configuration, or its default, does
// not hold a value of the expected type: a non-empty path and a bag of
// parameters.
var ErrInvalidField = errors.New("invalid sqlite database dialect field")
