package scheduler

import (
	"strings"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// job is a job of the scheduler, with its configuration.
type job struct {
	id       string
	job      flam.IScheduledJob
	schedule flam.CronSchedule
	active   bool
	timeout  time.Duration

	// locker is the locker of a singleton job, nil otherwise, acquiring
	// the lock of key lockKey for lockTTL.
	locker  flam.ILocker
	lockKey string
	lockTTL time.Duration
}

// newJobs returns the jobs, in the order given, with their configuration
// read from config at JobConfigPath, the lockers of the singleton ones
// being taken from lockers, which may be nil.
func newJobs(
	config flam.IConfig,
	lockers flam.ILockerFactory,
	scheduled []flam.IScheduledJob,
) ([]*job, error) {
	jobs := make([]*job, 0, len(scheduled))
	seen := map[string]bool{}
	for _, s := range scheduled {
		if value.IsNil(s) {
			return nil, newErr(ErrInvalidJob, jobConfigMessage, "")
		}
		id := s.Id()
		switch {
		case id == "" || strings.Contains(id, "."):
			return nil, newErr(ErrInvalidJob, jobConfigMessage, id)
		case seen[id]:
			return nil, newErr(ErrDuplicateJob, jobConfigMessage, id)
		}
		seen[id] = true

		j, err := newJob(config, lockers, id, s)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// newJob returns the job of the id, with its configuration read from
// config.
func newJob(
	config flam.IConfig,
	lockers flam.ILockerFactory,
	id string,
	scheduled flam.IScheduledJob,
) (*job, error) {
	path := JobConfigPath + "." + id
	if raw := config.Get(path); raw != nil {
		if _, ok := raw.(flam.IBag); !ok {
			return nil, newErrInvalidField(jobConfigMessage, id, path)
		}
	}
	bag := config.Bag(path, container.Bag{})
	r := fieldReader{config: config, bag: bag, id: id}

	j := &job{id: id, job: scheduled}
	j.schedule.Spec = r.string(ConfigFieldSchedule, "", "")
	j.schedule.Seconds = r.bool(ConfigFieldSeconds, defaultSeconds)
	j.schedule.Location = r.string(ConfigFieldLocation, DefaultLocationConfigPath, "")
	j.active = r.bool(ConfigFieldActive, defaultActive)
	j.timeout = r.duration(ConfigFieldTimeout, DefaultTimeoutConfigPath, 0, 0)
	singleton := r.bool(ConfigFieldSingleton, defaultSingleton)
	if r.err == nil && strings.TrimSpace(j.schedule.Spec) == "" {
		r.err = newErrInvalidField(jobConfigMessage, id, ConfigFieldSchedule)
	}
	switch {
	case r.err != nil:
		return nil, r.err
	case !singleton:
		return j, nil
	}

	lockerId := r.string(ConfigFieldLocker, DefaultLockerConfigPath, "")
	ttl := defaultLockTTL
	if j.timeout > 0 {
		ttl = j.timeout
	}
	j.lockTTL = r.duration(ConfigFieldLockTTL, DefaultLockTTLConfigPath, ttl, minLockTTL)
	if r.err != nil {
		return nil, r.err
	}
	if lockers == nil {
		return nil, newErr(ErrLocker, jobConfigMessage, id).
			With(ErrorFieldLocker, lockerId)
	}
	locker, err := lockers.Get(lockerId)
	if err != nil {
		return nil, newErrJoined(ErrLocker, jobConfigMessage, id, err).
			With(ErrorFieldLocker, lockerId)
	}
	j.locker = locker
	j.lockKey = LockKeyPrefix + id
	return j, nil
}

// fieldReader reads the fields of the configuration of a job, keeping the
// first error.
type fieldReader struct {
	config flam.IConfig
	bag    flam.IBag
	id     string
	err    error
}

// string returns the string of the field, or of its default.
func (r *fieldReader) string(
	field string,
	defaultPath string,
	builtIn string,
) string {
	s, ok := settings.Field(r.config, r.bag, field, defaultPath, builtIn).(string)
	r.check(ok, field)
	return s
}

// bool returns the bool of the field, or its built-in default.
func (r *fieldReader) bool(
	field string,
	builtIn bool,
) bool {
	b, ok := settings.Field(r.config, r.bag, field, "", builtIn).(bool)
	r.check(ok, field)
	return b
}

// duration returns the duration of the field, or of its default, which
// must be least at least.
func (r *fieldReader) duration(
	field string,
	defaultPath string,
	builtIn time.Duration,
	least time.Duration,
) time.Duration {
	d, ok := settings.Duration(settings.Field(r.config, r.bag, field, defaultPath, builtIn))
	r.check(ok && d >= least, field)
	return d
}

// check records the invalid field error, unless ok or an error is already
// recorded.
func (r *fieldReader) check(
	ok bool,
	field string,
) {
	if !ok && r.err == nil {
		r.err = newErrInvalidField(jobConfigMessage, r.id, field)
	}
}

// jobConfigMessage is the message of the errors of the job
// configurations.
const jobConfigMessage = "unable to create scheduler process"
