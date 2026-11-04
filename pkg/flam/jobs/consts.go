package jobs

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.jobs"

// Configuration path and container groups of the queues and the handlers.
const (
	// QueueConfigPath is the configuration path holding one queue
	// configuration per queue id, as read by the queue factory.
	QueueConfigPath = "flam.jobs.queues"

	// QueueCreatorGroup is the container group holding the
	// [flam.IJobQueueCreator] values collected by the queue factory, among
	// which the one of the bundle returned by [NewBundle].
	QueueCreatorGroup = "flam.jobs.queues.creator"

	// HandlerGroup is the container group the application provides its
	// [flam.IJobHandler] values in, each worker running the jobs of their
	// kinds.
	HandlerGroup = "flam.jobs.handlers"
)

// Driver is the driver name of the queues created by the creator of the
// bundle returned by [NewBundle], held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration, which may also be left out.
const Driver = "flam.jobs.queues.driver.database"

// Fields of a queue configuration.
const (
	// ConfigFieldConnection is the field holding the id of the database
	// connection of the queue, in the connection factory.
	ConfigFieldConnection = "connection"

	// ConfigFieldProcess is the field holding the id of the worker process
	// of the queue, in the kennel, "jobs-" followed by the queue id unless
	// set.
	ConfigFieldProcess = "process"

	// ConfigFieldPoll is the field holding the delay between two polls of
	// an idle worker, a positive duration such as "1s".
	ConfigFieldPoll = "poll"

	// ConfigFieldConcurrency is the field holding the number of jobs a
	// worker runs at once, 1 or more.
	ConfigFieldConcurrency = "concurrency"

	// ConfigFieldLease is the field holding the time a worker holds a job
	// it claimed, a positive duration: once elapsed, the job is taken as
	// crashed and claimed again, so that it must exceed the longest run of
	// a job.
	ConfigFieldLease = "lease"

	// ConfigFieldBackoff is the field holding the delay before the first
	// retry of a failed job, a duration doubled at each following retry.
	ConfigFieldBackoff = "backoff"

	// ConfigFieldMaxBackoff is the field holding the maximum delay before
	// the retry of a failed job, a duration.
	ConfigFieldMaxBackoff = "max_backoff"

	// ConfigFieldMaxAttempts is the field holding the number of attempts
	// of the jobs enqueued without one, 1 or more.
	ConfigFieldMaxAttempts = "max_attempts"

	// ConfigFieldRetention is the field holding the time the completed
	// jobs are kept for, keeping their deduplication keys, a duration, 0
	// deleting a job once completed.
	ConfigFieldRetention = "retention"

	// ConfigFieldCleanup is the field holding the delay between two
	// deletions of the completed jobs past their retention, a positive
	// duration.
	ConfigFieldCleanup = "cleanup"

	// ConfigFieldStopTimeout is the field holding the time the jobs
	// running when the worker stops are given to end before their context
	// is done, a duration.
	ConfigFieldStopTimeout = "stop_timeout"

	// ConfigFieldLocking is the field holding how a worker locks the job
	// it claims, [LockingAuto], [LockingSkipLocked] or [LockingNone].
	ConfigFieldLocking = "locking"

	// ConfigFieldMigrate is the field holding whether the queue creates or
	// updates the [Table] table of its database when created, a bool.
	ConfigFieldMigrate = "migrate"

	// ConfigFieldBroker is the field holding the id of the broker, in the
	// broker factory, on which Enqueue publishes a wakeup notification
	// that the worker subscribes to, so that it runs a new job before its
	// next poll, "" for none.
	ConfigFieldBroker = "broker"

	// ConfigFieldTopic is the field holding the topic of the wakeup
	// notifications, "flam.jobs." followed by the queue id unless set.
	ConfigFieldTopic = "topic"
)

// Configuration paths of the values taken by the fields missing from a
// queue configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], unset unless configured.
	DefaultConnectionConfigPath = "flam.jobs.defaults.queues.connection"

	// DefaultPollConfigPath is the path of the default of
	// [ConfigFieldPoll], 1s unless configured.
	DefaultPollConfigPath = "flam.jobs.defaults.queues.poll"

	// DefaultConcurrencyConfigPath is the path of the default of
	// [ConfigFieldConcurrency], 1 unless configured.
	DefaultConcurrencyConfigPath = "flam.jobs.defaults.queues.concurrency"

	// DefaultLeaseConfigPath is the path of the default of
	// [ConfigFieldLease], 5m unless configured.
	DefaultLeaseConfigPath = "flam.jobs.defaults.queues.lease"

	// DefaultBackoffConfigPath is the path of the default of
	// [ConfigFieldBackoff], 1s unless configured.
	DefaultBackoffConfigPath = "flam.jobs.defaults.queues.backoff"

	// DefaultMaxBackoffConfigPath is the path of the default of
	// [ConfigFieldMaxBackoff], 1h unless configured.
	DefaultMaxBackoffConfigPath = "flam.jobs.defaults.queues.max_backoff"

	// DefaultMaxAttemptsConfigPath is the path of the default of
	// [ConfigFieldMaxAttempts], 5 unless configured.
	DefaultMaxAttemptsConfigPath = "flam.jobs.defaults.queues.max_attempts"

	// DefaultRetentionConfigPath is the path of the default of
	// [ConfigFieldRetention], 24h unless configured.
	DefaultRetentionConfigPath = "flam.jobs.defaults.queues.retention"

	// DefaultCleanupConfigPath is the path of the default of
	// [ConfigFieldCleanup], 1m unless configured.
	DefaultCleanupConfigPath = "flam.jobs.defaults.queues.cleanup"

	// DefaultStopTimeoutConfigPath is the path of the default of
	// [ConfigFieldStopTimeout], 30s unless configured.
	DefaultStopTimeoutConfigPath = "flam.jobs.defaults.queues.stop_timeout"

	// DefaultLockingConfigPath is the path of the default of
	// [ConfigFieldLocking], [LockingAuto] unless configured.
	DefaultLockingConfigPath = "flam.jobs.defaults.queues.locking"

	// DefaultMigrateConfigPath is the path of the default of
	// [ConfigFieldMigrate], true unless configured.
	DefaultMigrateConfigPath = "flam.jobs.defaults.queues.migrate"

	// DefaultBrokerConfigPath is the path of the default of
	// [ConfigFieldBroker], none unless configured.
	DefaultBrokerConfigPath = "flam.jobs.defaults.queues.broker"
)

// Values of the [ConfigFieldLocking] field.
const (
	// LockingAuto claims with [LockingSkipLocked] on PostgreSQL and MySQL,
	// and with [LockingNone] on the other databases, such as SQLite.
	LockingAuto = "auto"

	// LockingSkipLocked selects the job to claim with
	// "FOR UPDATE SKIP LOCKED", in the transaction claiming it, so that
	// concurrent workers select different jobs.
	LockingSkipLocked = "skip_locked"

	// LockingNone selects the job to claim without lock, the claim being a
	// conditional update of the job that fails when another worker
	// claimed it first, the worker then selecting another one.
	LockingNone = "none"
)

// Table is the name of the table holding the jobs of every queue.
const Table = "flam_jobs"

// States of a job, held by the state column of [Table].
const (
	// StatePending is the state of a job waiting to run.
	StatePending = "pending"

	// StateRunning is the state of a job claimed by a worker.
	StateRunning = "running"

	// StateDone is the state of a completed job, kept for the retention.
	StateDone = "done"

	// StateDead is the state of a job that ran out of attempts, kept until
	// deleted by the application.
	StateDead = "dead"
)

// LogChannel is the channel of the entries the workers log, when the
// container provides a [flam.ILogger].
const LogChannel = "flam.jobs"

// Context fields of the [*flam.Error] values returned by the jobs package,
// and of the entries the workers log.
const (
	// ErrorFieldID holds the id of a queue.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the connection of a queue.
	ErrorFieldConnection = "connection"

	// ErrorFieldBroker holds the id of the broker of a queue.
	ErrorFieldBroker = "broker"

	// ErrorFieldKind holds the kind of a job or of a handler.
	ErrorFieldKind = "kind"

	// ErrorFieldJob holds the id of a job.
	ErrorFieldJob = "job"

	// ErrorFieldAttempt holds the number of an attempt of a job.
	ErrorFieldAttempt = "attempt"

	// ErrorFieldPanic holds the value a handler panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the goroutine of a handler
	// when it panicked, as a string.
	ErrorFieldStack = "stack"
)

// Defaults of the fields of a queue configuration.
const (
	defaultPoll        = time.Second
	defaultConcurrency = 1
	defaultLease       = 5 * time.Minute
	defaultBackoff     = time.Second
	defaultMaxBackoff  = time.Hour
	defaultMaxAttempts = 5
	defaultRetention   = 24 * time.Hour
	defaultCleanup     = time.Minute
	defaultStopTimeout = 30 * time.Second
	defaultLocking     = LockingAuto
	defaultMigrate     = true

	// processPrefix prefixes the queue id in the default process id.
	processPrefix = "jobs-"

	// topicPrefix prefixes the queue id in the default wakeup topic.
	topicPrefix = "flam.jobs."
)
