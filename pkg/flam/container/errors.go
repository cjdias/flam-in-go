package container

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidPath is the error wrapped by the errors returned from
	// [Bag.Set], [Bag.Delete] and [Bag.Merge] when the given path is
	// malformed, as described in [Bag], or is empty where the root bag
	// cannot be the target, and by [Bag.Merge] when a key of the merged bag
	// is not a valid path.
	ErrInvalidPath = errors.New("invalid path")

	// ErrPathConflict is the error wrapped by the errors returned from
	// [Bag.Set] and [Bag.Merge] when a path cannot be followed or created
	// because one of its segments already holds a value that is not a bag.
	ErrPathConflict = errors.New("path conflict")

	// ErrNil is the error wrapped by the error returned from [Bag.Set]
	// when called on a nil bag, which cannot be written to.
	ErrNil = errors.New("nil value")

	// ErrUnsupportedBag is the error wrapped by the error returned from
	// [Bag.Merge] when the merged flam.IBag is an implementation other than
	// Bag whose Get("") does not return a Bag or a map[string]any.
	ErrUnsupportedBag = errors.New("unsupported bag")
)

// newErrInvalidPath returns the error wrapping ErrInvalidPath with the
// message and the ErrorFieldPath context field.
func newErrInvalidPath(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidPath, message).
		With(ErrorFieldPath, path)
}

// newErrPathConflict returns the error wrapping ErrPathConflict with the
// message and the ErrorFieldPath context field.
func newErrPathConflict(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrPathConflict, message).
		With(ErrorFieldPath, path)
}

// newErrNil returns the error wrapping ErrNil with the message and the
// ErrorFieldPath context field.
func newErrNil(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrNil, message).
		With(ErrorFieldPath, path)
}

// newErrUnsupportedBag returns the error wrapping ErrUnsupportedBag with
// the message and the ErrorFieldPath context field.
func newErrUnsupportedBag(
	message string,
	path string,
) *flam.Error {
	return flam.NewWrappedError(ErrUnsupportedBag, message).
		With(ErrorFieldPath, path)
}
