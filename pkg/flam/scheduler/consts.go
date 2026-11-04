package scheduler

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.scheduler"

// ProcessId is the id of the scheduler process, in the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], configured at
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.ProcessConfigPath] by
// it.
const ProcessId = "scheduler"

// Configuration paths, container group and keys of the jobs.
const (
	// JobGroup is the container group the application provides its
	// [flam.IScheduledJob] values in, run by the scheduler.
	JobGroup = "flam.scheduler.jobs"

	// JobConfigPath is the configuration path holding the configuration
	// of the jobs of [JobGroup], by job id, with the fields named by the
	// ConfigField constants.
	JobConfigPath = "flam.scheduler.jobs"

	// LogChannelConfigPath is the configuration path of the channel of
	// the [flam.ILogger] the scheduler signals the runs of the jobs on. It
	// defaults to [DefaultLogChannel].
	LogChannelConfigPath = "flam.scheduler.log.channel"

	// TriggerIdPrefix is the prefix of the ids of the cron triggers of the
	// jobs, in the [flam.ICronTriggerFactory], followed by the job id.
	TriggerIdPrefix = "flam.scheduler.jobs."

	// LockKeyPrefix is the prefix of the keys of the locks of the
	// singleton jobs, followed by the job id.
	LockKeyPrefix = "flam.scheduler.jobs."

	// DefaultLogChannel is the logger channel used unless set at
	// [LogChannelConfigPath].
	DefaultLogChannel = "flam.scheduler"
)

// Fields of a job configuration, at [JobConfigPath].
const (
	// ConfigFieldSchedule is the field holding the cron expression of the
	// job, a non-empty string, as the Spec of a [flam.CronSchedule]. It is
	// required.
	ConfigFieldSchedule = "schedule"

	// ConfigFieldSeconds is the field holding whether the expression may
	// start with a seconds field, a bool, false unless set.
	ConfigFieldSeconds = "seconds"

	// ConfigFieldLocation is the field holding the name of the time zone
	// the expression is read in, a string, defaulting to the value at
	// [DefaultLocationConfigPath], or else to the location of the times of
	// the clock.
	ConfigFieldLocation = "location"

	// ConfigFieldActive is the field holding whether the job is
	// scheduled, a bool, true unless set.
	ConfigFieldActive = "active"

	// ConfigFieldTimeout is the field holding the time after which the
	// context of a run of the job is cancelled, a non-negative duration
	// such as "30s", defaulting to the value at
	// [DefaultTimeoutConfigPath], or else to 0, for no timeout.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldSingleton is the field holding whether a run of the job
	// takes place on a single instance of the application, a bool, false
	// unless set: the job then runs only when its lock, of key
	// [LockKeyPrefix] followed by its id, is acquired from the locker of
	// [ConfigFieldLocker], and is skipped otherwise.
	ConfigFieldSingleton = "singleton"

	// ConfigFieldLocker is the field holding the id of the locker of a
	// singleton job, in the [flam.ILockerFactory], a string, defaulting to
	// the value at [DefaultLockerConfigPath]. It is read for a singleton
	// job only.
	ConfigFieldLocker = "locker"

	// ConfigFieldLockTTL is the field holding the ttl of the lock of a
	// singleton job, a duration of 1ms or more, defaulting to the value at
	// [DefaultLockTTLConfigPath], or else to the timeout of the job when
	// set, or else to 1m. It is read for a singleton job only. It bounds
	// the time the lock of a crashed instance is kept, but a locker that
	// does not refresh its locks lets another instance run the job once it
	// elapsed, so a ttl shorter than the runs needs automatic refreshes.
	ConfigFieldLockTTL = "lock_ttl"
)

// Configuration paths of the values taken by the fields missing from a
// job configuration.
const (
	// DefaultLocationConfigPath is the path of the default of
	// [ConfigFieldLocation].
	DefaultLocationConfigPath = "flam.scheduler.defaults.jobs.location"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout].
	DefaultTimeoutConfigPath = "flam.scheduler.defaults.jobs.timeout"

	// DefaultLockerConfigPath is the path of the default of
	// [ConfigFieldLocker].
	DefaultLockerConfigPath = "flam.scheduler.defaults.jobs.locker"

	// DefaultLockTTLConfigPath is the path of the default of
	// [ConfigFieldLockTTL].
	DefaultLockTTLConfigPath = "flam.scheduler.defaults.jobs.lock_ttl"
)

// Context fields of the [*flam.Error] values returned by the provider of
// the scheduler process and by its Run method, and of the failures of the
// jobs.
const (
	// ErrorFieldID holds the id of the job, or [ProcessId] for an error of
	// the scheduler itself.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid job configuration
	// field, or the path of an invalid job configuration.
	ErrorFieldField = "field"

	// ErrorFieldLocker holds the id of the locker of a singleton job.
	ErrorFieldLocker = "locker"

	// ErrorFieldTimeout holds the timeout of a job that timed out.
	ErrorFieldTimeout = "timeout"

	// ErrorFieldPanic holds the value a job panicked with.
	ErrorFieldPanic = "panic"

	// ErrorFieldStack holds the stack trace of the panic of a job, as a
	// string.
	ErrorFieldStack = "stack"
)

// Context fields of the messages signalled to the [flam.ILogger].
const (
	// ContextFieldJob holds the id of the job.
	ContextFieldJob = "job"

	// ContextFieldDuration holds the duration of the run of the job, as
	// read from the clock, with its end or its failure.
	ContextFieldDuration = "duration"

	// ContextFieldError holds the failure of the job.
	ContextFieldError = "error"
)

// Defaults of the job fields.
const (
	defaultActive    = true
	defaultSeconds   = false
	defaultSingleton = false
	defaultLockTTL   = time.Minute

	// minLockTTL is the least ttl of a lock.
	minLockTTL = time.Millisecond
)
