package flam

import (
	"context"
	"io"
	"time"

	"gorm.io/gorm"
)

// IJob is a job of an [IJobQueue], handed to the [IJobHandler] of its kind
// by a worker of the queue. It may be read from several goroutines, but
// must not be changed.
type IJob interface {
	// Id returns the id the queue gave the job when enqueuing it.
	Id() string

	// Queue returns the id of the queue of the job.
	Queue() string

	// Kind returns the kind of the job, naming its handler.
	Kind() string

	// Key returns the deduplication key the job was enqueued with, or ""
	// when it has none.
	Key() string

	// Payload returns the JSON encoding of the payload the job was
	// enqueued with.
	Payload() []byte

	// Decode decodes the JSON payload of the job into target, a pointer,
	// as [encoding/json.Unmarshal] does.
	Decode(target any) error

	// Attempt returns the number of the attempt under way, from 1: an
	// attempt whose worker crashed counts as one.
	Attempt() int

	// MaxAttempts returns the number of attempts the job is allowed,
	// after which a failing job is dead.
	MaxAttempts() int

	// EnqueuedAt returns the time the job was enqueued at.
	EnqueuedAt() time.Time
}

// IJobHandler handles the jobs of a kind, provided by the application in
// the [github.com/cjdias/flam-in-go/pkg/flam/jobs.HandlerGroup] container
// group. Returning nil completes the job, while returning an error, or
// panicking, fails the attempt: the job is retried after a backoff, until
// it runs out of attempts and is dead. As a job may run more than once,
// such as when its worker crashes before recording its completion, a
// handler must be idempotent.
type IJobHandler interface {
	// Kind returns the kind of the jobs the handler handles, unique among
	// the handlers.
	Kind() string

	// Handle handles the job, with a context done once the worker stops
	// and its stop timeout elapses.
	Handle(ctx context.Context, job IJob) error
}

// JobOptions are the options of the Enqueue method of an [IJobQueue], set
// by the [JobOption] values of
// [github.com/cjdias/flam-in-go/pkg/flam/jobs], such as
// [github.com/cjdias/flam-in-go/pkg/flam/jobs.WithDelay].
type JobOptions struct {
	// RunAt is the time before which the job does not run, the zero time
	// running it at once, unless Delay is set.
	RunAt time.Time

	// Delay is the delay from the enqueue time before which the job does
	// not run, when RunAt is the zero time.
	Delay time.Duration

	// MaxAttempts is the number of attempts the job is allowed, the
	// default of the queue when 0.
	MaxAttempts int

	// Key is the deduplication key of the job: enqueuing a job of a key
	// held by a job of the queue still stored does not add one, returning
	// the id of the stored job instead. Empty for none.
	Key string

	// Tx is the gorm transaction to insert the job with, so that the job
	// is enqueued only when the transaction commits, nil inserting it with
	// the connection of the queue. Enqueue then notifies no worker, as the
	// job is not visible before the commit: the caller calls the Notify
	// method of the queue once the transaction commits.
	Tx *gorm.DB
}

// JobOption sets an option of the Enqueue method of an [IJobQueue].
type JobOption func(options *JobOptions)

// IJobQueue is a queue of background jobs stored in a database, enqueued
// with Enqueue and run by the workers of the process returned by Worker,
// each job by the [IJobHandler] of its kind.
//
// A queue is held by an [IJobQueueFactory], which closes it when removing
// it. Closing it stops its worker, after which Enqueue fails with
// [github.com/cjdias/flam-in-go/pkg/flam/jobs.ErrClosed]. A queue is safe
// for concurrent use.
type IJobQueue interface {
	io.Closer
	IResource

	// Enqueue stores the job of the kind, with the JSON encoding of the
	// payload, and the options, returning its id, or the id of the job of
	// its deduplication key when one is stored. It fails with the error
	// wrapping [github.com/cjdias/flam-in-go/pkg/flam/jobs.ErrInvalidKind]
	// when the kind is empty,
	// [github.com/cjdias/flam-in-go/pkg/flam/jobs.ErrInvalidOption] when
	// the maximum number of attempts is negative,
	// [github.com/cjdias/flam-in-go/pkg/flam/jobs.ErrEncode] when the
	// payload cannot be encoded, or
	// [github.com/cjdias/flam-in-go/pkg/flam/jobs.ErrEnqueue] when the
	// database refuses it. Once stored, the job is notified to the
	// workers, as Notify does, unless it is stored with the transaction
	// of the caller, which must call Notify once it commits.
	Enqueue(ctx context.Context, kind string, payload any, opts ...JobOption) (string, error)

	// Notify wakes the idle worker of the queue, and publishes the wakeup
	// notification of the queue when it has a broker, so that the workers
	// poll at once rather than after their poll delay, such as after the
	// commit of a transaction jobs were enqueued with. The failure to
	// publish is logged, as the polls find the jobs anyway.
	Notify(ctx context.Context)

	// Worker returns the process running the workers of the queue,
	// provided in the
	// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.ProcessGroup]
	// group so that the kennel runs it, the same on every call.
	Worker() IWatchdogProcess
}

// IJobQueueCreator creates the job queues of a driver, for the
// [IJobQueueFactory].
type IJobQueueCreator = IResourceCreator[IJobQueue]

// IJobQueueFactory creates and holds the job queues, one per id.
type IJobQueueFactory = IFactory[IJobQueue]
