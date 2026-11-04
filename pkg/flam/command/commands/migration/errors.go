package migration

import (
	"errors"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ErrInvalidAction is the error wrapped by the error returned from the
// migrate command when its first argument is not one of its actions:
// [ActionUp], [ActionDown] or [ActionStatus].
var ErrInvalidAction = errors.New("invalid migrate action")

// newErrAction returns the error wrapping ErrInvalidAction with the
// message and the ErrorFieldAction context field.
func newErrAction(
	message string,
	action string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidAction, message).
		With(ErrorFieldAction, action)
}

// newErrMigrator returns the error wrapping the error of a migrator, or
// of the context, with the message and the ErrorFieldAction and
// ErrorFieldID context fields.
func newErrMigrator(
	err error,
	message string,
	action string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(err, message).
		With(ErrorFieldAction, action).
		With(ErrorFieldID, id)
}
