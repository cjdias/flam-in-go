package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidJob is the error wrapped by the error returned from the
	// provider of the scheduler process, and so by the container, when a
	// job of [JobGroup] is nil, or its id is empty or holds a dot, which
	// would make it a nested path of the configuration.
	ErrInvalidJob = errors.New("invalid scheduled job")

	// ErrDuplicateJob is the error wrapped by the error returned from the
	// provider of the scheduler process when two jobs of [JobGroup] have
	// the same id.
	ErrDuplicateJob = errors.New("scheduled job id already in use")

	// ErrInvalidField is the error wrapped by the error returned from the
	// provider of the scheduler process when the configuration of a job
	// is not a bag, or a field of it, or its default, does not hold a
	// value of the expected type, or holds an invalid one: an empty
	// schedule, a negative timeout or a lock ttl under 1ms.
	ErrInvalidField = errors.New("invalid scheduled job field")

	// ErrLocker is the error wrapped by the error returned from the
	// provider of the scheduler process when the locker of a singleton job
	// cannot be taken from the [flam.ILockerFactory], along with its
	// error, or when the container holds no ILockerFactory.
	ErrLocker = errors.New("unable to get scheduled job locker")

	// ErrRunning is the error wrapped by the error returned from the Run
	// method of the scheduler process when a run is under way already.
	ErrRunning = errors.New("scheduler running")

	// ErrClosed is the error wrapped by the error returned from the Run
	// method of the scheduler process once it is closed.
	ErrClosed = errors.New("scheduler closed")

	// ErrSchedule is the error wrapped, along with the error of the
	// [flam.ICronTriggerFactory], by the error returned from the Run
	// method of the scheduler process when the trigger of a job cannot be
	// created, as its schedule is invalid.
	ErrSchedule = errors.New("unable to schedule job")

	// ErrJob is the error wrapped, along with the error the job returned,
	// by the failure of a job signalled to the logger.
	ErrJob = errors.New("scheduled job failed")

	// ErrPanic is the error wrapped, along with the value it panicked with
	// when an error, by the failure of a job signalled to the logger when
	// it panicked.
	ErrPanic = errors.New("scheduled job panicked")

	// ErrExit is the error wrapped by the failure of a job signalled to
	// the logger when it ended its goroutine with [runtime.Goexit].
	ErrExit = errors.New("scheduled job exited")

	// ErrTimeout is the error wrapped by the cause of the cancellation of
	// the context of a run of a job, as returned by [context.Cause], once
	// its timeout elapsed, and by the failure of the job signalled to the
	// logger then, along with the error the job returned, if any.
	ErrTimeout = errors.New("scheduled job timed out")

	// ErrLock is the error wrapped, along with the error of the
	// [flam.ILocker] or of the [flam.ILock], by the failure of a singleton
	// job signalled to the logger when its lock cannot be acquired or
	// released.
	ErrLock = errors.New("unable to lock scheduled job")
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

// newErrInvalidField returns the error wrapping ErrInvalidField with the
// message and the ErrorFieldID and ErrorFieldField context fields.
func newErrInvalidField(
	message string,
	id string,
	field string,
) *flam.Error {
	return newErr(ErrInvalidField, message, id).
		With(ErrorFieldField, field)
}

// newErrPanic returns the error wrapping ErrPanic, joined with the value
// the job of the id panicked with when an error, with the message and the
// ErrorFieldID, ErrorFieldPanic and ErrorFieldStack context fields.
func newErrPanic(
	message string,
	id string,
	value any,
	stack []byte,
) *flam.Error {
	sentinel := ErrPanic
	if err, ok := value.(error); ok {
		sentinel = fmt.Errorf("%w: %w", ErrPanic, err)
	}
	return newErr(sentinel, message, id).
		With(ErrorFieldPanic, value).
		With(ErrorFieldStack, string(stack))
}

// newErrTimeout returns the error wrapping ErrTimeout, joined with the
// error the job returned when not nil, with the message and the
// ErrorFieldID and ErrorFieldTimeout context fields.
func newErrTimeout(
	message string,
	id string,
	timeout time.Duration,
	err error,
) *flam.Error {
	sentinel := ErrTimeout
	if err != nil {
		sentinel = fmt.Errorf("%w: %w", ErrTimeout, err)
	}
	return newErr(sentinel, message, id).
		With(ErrorFieldTimeout, timeout)
}
