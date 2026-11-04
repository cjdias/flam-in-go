package watchdog

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrNilProcess is the error wrapped by the error returned from Add
	// when the process is nil.
	ErrNilProcess = errors.New("nil watchdog process")

	// ErrInvalidID is the error wrapped by the error returned from Add
	// for an empty id, and when the kennel is created for a process of
	// [ProcessGroup] whose id is empty or holds a dot, which would make it
	// a nested path of the configuration.
	ErrInvalidID = errors.New("invalid watchdog process id")

	// ErrIDMismatch is the error wrapped by the error returned from Add
	// when the id of the process is not the id it is added under.
	ErrIDMismatch = errors.New("watchdog process id mismatch")

	// ErrDuplicateProcess is the error wrapped by the error returned from
	// Add when the kennel holds a process of the id already.
	ErrDuplicateProcess = errors.New("watchdog process id already in use")

	// ErrUnknownProcess is the error wrapped by the error returned from
	// Activate and Deactivate when the kennel holds no process of the id.
	ErrUnknownProcess = errors.New("unknown watchdog process")

	// ErrRunning is the error wrapped by the error returned from Add,
	// Activate and Deactivate while the kennel runs its processes, as the
	// run would not take the change into account.
	ErrRunning = errors.New("watchdog kennel running")

	// ErrClosed is the error wrapped by the error returned from Add, and
	// from the run of the kennel, once the kennel is closed.
	ErrClosed = errors.New("watchdog kennel closed")

	// ErrLogger is the error wrapped, along with the error of the
	// [flam.IWatchdogLoggerAdaptorFactory], by the error returned from Add
	// when the logger adaptor cannot be taken from the factory.
	ErrLogger = errors.New("unable to get watchdog process logger")

	// ErrInvalidField is the error wrapped by the error returned when the
	// kennel is created, and so by the container, when the configuration
	// of a process of [ProcessGroup] is not a bag, or a field of it, or
	// its default, does not hold a value of the expected type: an active
	// bool and a logger string, or when the value at
	// [RestartDelayConfigPath] or [RestartMaxDelayConfigPath] is not a
	// non-negative duration, or the maximum is set shorter than the delay.
	ErrInvalidField = errors.New("invalid watchdog process field")

	// ErrPanic is the error wrapped, along with the value it panicked
	// with, by the error reported to the logger adaptor of a process when
	// it panics. It is not returned from the run of the kennel, as a
	// process panicking is run again, until the kennel is closed.
	ErrPanic = errors.New("watchdog process panicked")

	// ErrClose is the error wrapped, along with the error of the process,
	// by the errors returned from the closing of the kennel when a process
	// fails to close.
	ErrClose = errors.New("unable to close watchdog process")
)

// newErr returns the error wrapping the sentinel with the message and the
// ErrorFieldID context field.
func newErr(
	sentinel error,
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(sentinel, message).
		With(ErrorFieldID, id)
}

// newErrJoined returns the error wrapping the sentinel, joined with the
// cause, with the message and the ErrorFieldID context field.
func newErrJoined(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	return newErr(fmt.Errorf("%w: %w", sentinel, cause), message, id)
}

// newErrPanic returns the error wrapping ErrPanic, joined with the value
// the process of the id panicked with when an error, with the message and
// the ErrorFieldID, ErrorFieldPanic and ErrorFieldStack context fields.
func newErrPanic(
	message string,
	id string,
	value any,
	stack []byte,
) *flam.Error {
	sentinel := fmt.Errorf("%w: %v", ErrPanic, value)
	if err, ok := value.(error); ok {
		sentinel = fmt.Errorf("%w: %w", ErrPanic, err)
	}
	return newErr(sentinel, message, id).
		With(ErrorFieldPanic, value).
		With(ErrorFieldStack, string(stack))
}
