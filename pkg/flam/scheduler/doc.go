// Package scheduler is the scheduler feature: the bundle returned from
// [NewBundle] adds to the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog] a process running the
// [github.com/cjdias/flam-in-go/pkg/flam.IScheduledJob] values the
// application provides in the [JobGroup] container group, each at the
// times matched by the cron expression configured for its id at
// [JobConfigPath], such as:
//
//	flam:
//	  scheduler:
//	    jobs:
//	      report: {schedule: "CRON_TZ=Europe/Lisbon 0 6 * * 1-5", timeout: 10m}
//	      cleanup: {schedule: "@every 15m", singleton: true, locker: redis}
//	      legacy: {schedule: "@hourly", active: false}
//
// Each active job is run by a cron trigger of the
// [github.com/cjdias/flam-in-go/pkg/flam.ICronTriggerFactory] of
// [github.com/cjdias/flam-in-go/pkg/flam/trigger], so its timing is read
// from the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the
// container, and follows the overlap policy of the triggers: a job is
// never run twice at once, the times matched while it runs being skipped.
//
// A run of a job is given a context cancelled once its timeout elapses,
// if any, and when the scheduler stops. A singleton job runs on a single
// instance of the application at a time: each run first tries to acquire
// its lock from the
// [github.com/cjdias/flam-in-go/pkg/flam.ILocker] configured for it, and
// is skipped when another instance holds it, the lock being released once
// the run ends. As the instances fire at the same times, their clocks
// should be synchronised, a run shorter than their skew letting a late
// instance acquire the released lock and run the job again.
//
// The scheduler recovers the panics of the jobs, and signals, when the
// container holds a [github.com/cjdias/flam-in-go/pkg/flam.ILogger], the
// start and the end of each run at the info level, its failure, an error,
// a panic, a timeout or a lock failure, at the error level, and its skip
// at the debug level, with the job id, the duration of the run and the
// failure in the context. Closing the scheduler, as the kennel does when
// it closes, stops the triggers, cancels the context of the runs under
// way, and waits for them to end.
package scheduler
