package trigger

import (
	"errors"
	"fmt"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidID is the error wrapped by the error returned from the
	// [flam.ITriggerFactory] creation methods when the id is empty.
	ErrInvalidID = errors.New("invalid trigger id")

	// ErrNilCallback is the error wrapped by the error returned from the
	// [flam.ITriggerFactory] creation methods when the callback is nil.
	ErrNilCallback = errors.New("nil trigger callback")

	// ErrInvalidDelay is the error wrapped by the error returned from the
	// [flam.ITriggerFactory] creation methods when the delay is not
	// positive.
	ErrInvalidDelay = errors.New("invalid trigger delay")

	// ErrInvalidSchedule is the error wrapped, along with the error of the
	// parsing, by the error returned from
	// [flam.ICronTriggerFactory.NewCron] when the schedule is not a valid
	// cron expression, or names a time zone that cannot be loaded.
	ErrInvalidSchedule = errors.New("invalid trigger schedule")

	// ErrClosed is the error wrapped by the error returned from the
	// [flam.ITriggerFactory] creation methods once the factory is closed.
	ErrClosed = errors.New("trigger factory closed")

	// ErrDuplicateID is the error wrapped by the error returned from the
	// [flam.ITriggerFactory] creation methods when a running trigger of
	// the factory uses the id.
	ErrDuplicateID = errors.New("trigger id already in use")

	// ErrCallback is the error wrapped by the error passed to the
	// [flam.TriggerErrorHandler] values when the callback returned an
	// error, which is wrapped too.
	ErrCallback = errors.New("trigger callback failed")

	// ErrCallbackPanic is the error wrapped by the error passed to the
	// [flam.TriggerErrorHandler] values when the callback panicked, along
	// with the panic value when it is an error.
	ErrCallbackPanic = errors.New("trigger callback panicked")

	// ErrCallbackExit is the error wrapped by the error passed to the
	// [flam.TriggerErrorHandler] values when the callback ended its
	// goroutine with [runtime.Goexit], as [testing.T.FailNow] does.
	ErrCallbackExit = errors.New("trigger callback exited")
)

// newErrInvalidID returns the error wrapping ErrInvalidID with the
// message.
func newErrInvalidID(
	message string,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidID, message)
}

// newErrNilCallback returns the error wrapping ErrNilCallback with the
// message and the ErrorFieldID context field.
func newErrNilCallback(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrNilCallback, message).
		With(ErrorFieldID, id)
}

// newErrInvalidDelay returns the error wrapping ErrInvalidDelay with the
// message and the ErrorFieldID and ErrorFieldDelay context fields.
func newErrInvalidDelay(
	message string,
	id string,
	delay time.Duration,
) *flam.Error {
	return flam.NewWrappedError(ErrInvalidDelay, message).
		With(ErrorFieldID, id).
		With(ErrorFieldDelay, delay)
}

// newErrInvalidSchedule returns the error wrapping ErrInvalidSchedule,
// joined with the error of the parsing, with the message and the
// ErrorFieldID and ErrorFieldSchedule context fields.
func newErrInvalidSchedule(
	message string,
	id string,
	schedule flam.CronSchedule,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrInvalidSchedule, err), message).
		With(ErrorFieldID, id).
		With(ErrorFieldSchedule, schedule)
}

// newErrClosed returns the error wrapping ErrClosed with the message and
// the ErrorFieldID context field.
func newErrClosed(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrClosed, message).
		With(ErrorFieldID, id)
}

// newErrDuplicateID returns the error wrapping ErrDuplicateID with the
// message and the ErrorFieldID context field.
func newErrDuplicateID(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrDuplicateID, message).
		With(ErrorFieldID, id)
}

// newErrCallback returns the error wrapping ErrCallback, joined with the
// error of the callback, with the message and the ErrorFieldID context
// field.
func newErrCallback(
	message string,
	id string,
	err error,
) *flam.Error {
	return flam.NewWrappedError(fmt.Errorf("%w: %w", ErrCallback, err), message).
		With(ErrorFieldID, id)
}

// newErrCallbackPanic returns the error wrapping ErrCallbackPanic, joined
// with the panic value when it is an error, with the message and the
// ErrorFieldID, ErrorFieldPanic and ErrorFieldStack context fields.
func newErrCallbackPanic(
	message string,
	id string,
	value any,
	stack []byte,
) *flam.Error {
	cause := ErrCallbackPanic
	if panicErr, isErr := value.(error); isErr {
		cause = fmt.Errorf("%w: %w", ErrCallbackPanic, panicErr)
	}
	return flam.NewWrappedError(cause, message).
		With(ErrorFieldID, id).
		With(ErrorFieldPanic, value).
		With(ErrorFieldStack, string(stack))
}

// newErrCallbackExit returns the error wrapping ErrCallbackExit with the
// message and the ErrorFieldID context field.
func newErrCallbackExit(
	message string,
	id string,
) *flam.Error {
	return flam.NewWrappedError(ErrCallbackExit, message).
		With(ErrorFieldID, id)
}
