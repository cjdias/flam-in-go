package s3

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// creator when a field of the disk configuration, or its default, does
	// not hold a value of the expected type: a non-empty bucket, prefix,
	// region, endpoint and credential strings, a path style bool, an
	// access key and a secret key both set or both unset, and a session
	// token set only along with them.
	ErrInvalidField = errors.New("invalid s3 disk field")

	// ErrConfig is the error wrapped, along with the error of the SDK, by
	// the error returned from the creator when the AWS configuration
	// cannot be loaded, such as for a missing profile of the shared
	// configuration.
	ErrConfig = errors.New("unable to load s3 disk configuration")
)

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidField, message).
		With(ErrorFieldID, id).
		With(ErrorFieldField, field)
}

// newErrConfig returns the error wrapping ErrConfig, joined with the
// cause, with the message and the ErrorFieldID context field.
func newErrConfig(
	message string,
	id string,
	cause error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrConfig, cause), message).
		With(ErrorFieldID, id)
}
