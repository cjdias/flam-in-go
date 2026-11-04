package secrets

import (
	"errors"
)

// The errors wrapped by the errors of the store drivers, whatever the
// driver, so that the callers of a store can tell them apart.
var (
	// ErrNotFound is the error wrapped by the error of a store when the
	// secret of a key, or the secrets under a path, do not exist.
	ErrNotFound = errors.New("secret not found")

	// ErrInvalidKey is the error wrapped by the error of a store when it
	// refuses a key or a path, such as an empty one, or one escaping the
	// directory of a file store.
	ErrInvalidKey = errors.New("invalid secret key")
)
