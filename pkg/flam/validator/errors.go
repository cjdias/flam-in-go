package validator

import (
	"errors"
)

// ErrInvalidValue is the error wrapped by the result of a validator when
// the value cannot be validated, such as a value that is not a struct,
// whatever the driver, so that the callers of a validator can tell it
// apart from the failures of a valid value.
var ErrInvalidValue = errors.New("invalid validation value")
