package jobs

import (
	"errors"
	"fmt"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

var (
	// ErrInvalidField is the error wrapped by the error returned from the
	// queue creator when a field of the queue configuration, or its
	// default, does not hold a value of the expected type and range.
	ErrInvalidField = errors.New("invalid job queue field")

	// ErrConnection is the error wrapped, along with the error of the
	// [flam.IDatabaseConnectionFactory], by the error returned from the
	// queue creator when the database connection cannot be taken from the
	// factory, or when the container provides no factory.
	ErrConnection = errors.New("unable to get job queue connection")

	// ErrBroker is the error wrapped, along with the error of the
	// [flam.IBrokerFactory], by the error returned from the queue creator
	// when the wakeup broker cannot be taken from the factory, or when the
	// container provides no factory.
	ErrBroker = errors.New("unable to get job queue broker")

	// ErrMigrate is the error wrapped, along with the error of gorm, by
	// the error returned from the queue creator, or from [Migrate], when
	// the [Table] table cannot be created or updated.
	ErrMigrate = errors.New("unable to migrate job table")

	// ErrNilHandler is the error wrapped by the error returned from the
	// queue creator when a handler of the [HandlerGroup] group is nil.
	ErrNilHandler = errors.New("nil job handler")

	// ErrDuplicateKind is the error wrapped by the error returned from the
	// queue creator when two handlers of the [HandlerGroup] group handle
	// the same kind.
	ErrDuplicateKind = errors.New("duplicate job handler kind")

	// ErrClosed is the error wrapped by the error returned from the
	// Enqueue method of a closed queue.
	ErrClosed = errors.New("job queue closed")

	// ErrInvalidKind is the error wrapped by the error returned from the
	// Enqueue method of a queue when the kind is empty.
	ErrInvalidKind = errors.New("invalid job kind")

	// ErrInvalidOption is the error wrapped by the error returned from the
	// Enqueue method of a queue when the maximum number of attempts is
	// negative.
	ErrInvalidOption = errors.New("invalid job option")

	// ErrEncode is the error wrapped, along with the error of the encoder,
	// by the error returned from the Enqueue method of a queue when the
	// payload cannot be encoded.
	ErrEncode = errors.New("unable to encode job payload")

	// ErrEnqueue is the error wrapped, along with the error of gorm, by
	// the error returned from the Enqueue method of a queue when the job
	// cannot be stored.
	ErrEnqueue = errors.New("unable to enqueue job")

	// ErrRunning is the error wrapped by the error returned from the Run
	// method of the worker of a queue when it runs already.
	ErrRunning = errors.New("job worker running")

	// ErrClaim is the error wrapped, along with the error of gorm, by the
	// error a worker logs when it cannot claim a job, polling again later.
	ErrClaim = errors.New("unable to claim job")

	// ErrRecord is the error wrapped, along with the error of gorm, by the
	// error a worker logs when it cannot record the outcome of a job,
	// which runs again once its lease expires.
	ErrRecord = errors.New("unable to record job outcome")

	// ErrLeaseLost is the error wrapped by the error a worker logs when
	// the job it ran was claimed by another worker, as its lease expired,
	// its outcome being dropped.
	ErrLeaseLost = errors.New("job lease lost")

	// ErrUnknownKind is the error wrapped by the error failing the attempt
	// of a job of a kind no handler handles.
	ErrUnknownKind = errors.New("unknown job kind")

	// ErrHandler is the error wrapped, along with the error of the
	// handler, by the error failing the attempt of a job whose handler
	// returned an error.
	ErrHandler = errors.New("job handler failed")

	// ErrHandlerPanic is the error wrapped, joined with the panic value
	// when it is an error, by the error failing the attempt of a job whose
	// handler panicked.
	ErrHandlerPanic = errors.New("job handler panicked")

	// ErrLeaseExpired is the error wrapped by the error making dead a job
	// claimed again, as its lease expired, once out of attempts.
	ErrLeaseExpired = errors.New("job lease expired")

	// ErrDead is the error wrapped by the error a worker logs when a job
	// runs out of attempts.
	ErrDead = errors.New("job dead")

	// ErrCleanup is the error wrapped, along with the error of gorm, by
	// the error a worker logs when it cannot delete the completed jobs
	// past their retention.
	ErrCleanup = errors.New("unable to clean up jobs")

	// ErrWakeup is the error wrapped, along with the error of the broker,
	// by the error logged when a wakeup notification cannot be published
	// or subscribed to, the worker then relying on its polls.
	ErrWakeup = errors.New("unable to use job wakeup notifications")
)

// newErr returns the error wrapping the sentinel, joined with the cause
// when not nil, with the message.
func newErr(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	if cause != nil {
		sentinel = fmt.Errorf("%w: %w", sentinel, cause)
	}
	return flam.NewWrappedError(sentinel, message)
}

// newErrQueue returns the error of newErr with the ErrorFieldID context
// field of the queue.
func newErrQueue(
	sentinel error,
	message string,
	id string,
	cause error,
) *flam.Error {
	return newErr(sentinel, message, cause).
		With(ErrorFieldID, id)
}

// newErrJob returns the error of newErrQueue with the ErrorFieldJob,
// ErrorFieldKind and ErrorFieldAttempt context fields of the job.
func newErrJob(
	sentinel error,
	message string,
	j *job,
	cause error,
) *flam.Error {
	return newErrQueue(sentinel, message, j.row.Queue, cause).
		With(ErrorFieldJob, j.Id()).
		With(ErrorFieldKind, j.row.Kind).
		With(ErrorFieldAttempt, j.row.Attempt)
}
