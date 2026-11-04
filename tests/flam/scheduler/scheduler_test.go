package scheduler

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/scheduler"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// hourly returns the values configuring the jobs of the ids to run
// hourly.
func hourly(ids ...string) map[string]any {
	values := map[string]any{}
	for _, id := range ids {
		values[scheduler.JobConfigPath+"."+id] = container.Bag{scheduler.ConfigFieldSchedule: "@hourly"}
	}
	return values
}

// with returns the values with the fields of the job of the id added.
func with(values map[string]any, id string, fields container.Bag) map[string]any {
	path := scheduler.JobConfigPath + "." + id
	bag, _ := values[path].(container.Bag)
	if bag == nil {
		bag = container.Bag{}
	}
	for field, value := range fields {
		bag[field] = value
	}
	values[path] = bag
	return values
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should have the documented sentinel messages", func(t *testing.T) {
		for err, text := range map[error]string{
			scheduler.ErrInvalidJob:   "invalid scheduled job",
			scheduler.ErrDuplicateJob: "scheduled job id already in use",
			scheduler.ErrInvalidField: "invalid scheduled job field",
			scheduler.ErrLocker:       "unable to get scheduled job locker",
			scheduler.ErrRunning:      "scheduler running",
			scheduler.ErrClosed:       "scheduler closed",
			scheduler.ErrSchedule:     "unable to schedule job",
			scheduler.ErrJob:          "scheduled job failed",
			scheduler.ErrPanic:        "scheduled job panicked",
			scheduler.ErrExit:         "scheduled job exited",
			scheduler.ErrTimeout:      "scheduled job timed out",
			scheduler.ErrLock:         "unable to lock scheduled job",
		} {
			testutil.Equal(t, "sentinel", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})

	t.Run("should have the documented constants", func(t *testing.T) {
		for _, c := range [][2]string{
			{scheduler.ProcessId, "scheduler"},
			{scheduler.JobGroup, "flam.scheduler.jobs"},
			{scheduler.JobConfigPath, "flam.scheduler.jobs"},
			{scheduler.LogChannelConfigPath, "flam.scheduler.log.channel"},
			{scheduler.TriggerIdPrefix, "flam.scheduler.jobs."},
			{scheduler.DefaultLogChannel, "flam.scheduler"},
			{scheduler.ConfigFieldSchedule, "schedule"},
			{scheduler.ConfigFieldSeconds, "seconds"},
			{scheduler.ConfigFieldLocation, "location"},
			{scheduler.ConfigFieldActive, "active"},
			{scheduler.ConfigFieldTimeout, "timeout"},
			{scheduler.ConfigFieldSingleton, "singleton"},
			{scheduler.ConfigFieldLocker, "locker"},
			{scheduler.ConfigFieldLockTTL, "lock_ttl"},
			{scheduler.DefaultLocationConfigPath, "flam.scheduler.defaults.jobs.location"},
			{scheduler.DefaultTimeoutConfigPath, "flam.scheduler.defaults.jobs.timeout"},
			{scheduler.DefaultLockerConfigPath, "flam.scheduler.defaults.jobs.locker"},
			{scheduler.DefaultLockTTLConfigPath, "flam.scheduler.defaults.jobs.lock_ttl"},
			{scheduler.ErrorFieldID, "id"},
			{scheduler.ErrorFieldField, "field"},
			{scheduler.ErrorFieldLocker, "locker"},
			{scheduler.ErrorFieldTimeout, "timeout"},
			{scheduler.ErrorFieldPanic, "panic"},
			{scheduler.ErrorFieldStack, "stack"},
			{scheduler.ContextFieldJob, "job"},
			{scheduler.ContextFieldDuration, "duration"},
			{scheduler.ContextFieldError, "error"},
		} {
			testutil.Equal(t, "constant", c[0], c[1])
		}
		testutil.Equal(t, "LockKeyPrefix", scheduler.LockKeyPrefix, "flam.scheduler.jobs.")
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := scheduler.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.scheduler")
		if b == scheduler.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
	})

	t.Run("should register a single grouped provider", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, scheduler.NewBundle().Register(a))
		testutil.Equal(t, "calls", a.Calls(), []string{"provider/1"})
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		a := testutil.NewApp()
		a.Fail = func(string) error { return errBoom }
		err := scheduler.NewBundle().Register(a)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
		testutil.Context(t, asFlamError(t, err), map[string]any{
			bundle.ErrorFieldIndex: 0, bundle.ErrorFieldKind: "provider",
		})
	})

	t.Run("should provide no process without jobs", func(t *testing.T) {
		a, _ := newApp(t, options{})
		ps, err := processes(a)
		testutil.NoError(t, err)
		testutil.Equal(t, "processes", len(ps), 0)
	})

	t.Run("should provide the scheduler process, not running", func(t *testing.T) {
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{newJob("a", nil)}})
		testutil.Equal(t, "Id()", p.Id(), "scheduler")
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should need an ICronTriggerFactory in the container", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(start) }))
		testutil.NoError(t, a.Provide(func() flam.IScheduledJob { return newJob("a", nil) },
			dig.Group(scheduler.JobGroup)))
		testutil.NoError(t, a.RegisterBundle(scheduler.NewBundle()))
		_, err := processes(a)
		if err == nil || !strings.Contains(err.Error(), "flam.ICronTriggerFactory") {
			t.Fatalf("processes() = %v, want a missing flam.ICronTriggerFactory error", err)
		}
	})

	t.Run("should have the kennel run and close the scheduler", func(t *testing.T) {
		j := newJob("a", nil)
		a, clock := newApp(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
		testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
		var kennel flam.IKennel
		testutil.NoError(t, a.Invoke(func(k flam.IKennel) { kennel = k }))
		testutil.Equal(t, "Available()", kennel.Available(), []string{"scheduler"})

		result := make(chan error, 1)
		go func() { result <- runKennel(a) }()
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		receive(t, "run", j.runs)

		p, _ := kennel.Process(scheduler.ProcessId)
		testutil.NoError(t, p.Close())
		testutil.NoError(t, receive(t, "kennel run", result))
		timers(t, clock, 0)
	})
}

// ---------------------------------------------------------------------------
// configuration
// ---------------------------------------------------------------------------

func Test_Configuration(t *testing.T) {
	const message = "unable to create scheduler process"
	path := scheduler.JobConfigPath + ".a"

	t.Run("should refuse an invalid job", func(t *testing.T) {
		scenarios := []struct {
			name     string
			jobs     []flam.IScheduledJob
			sentinel error
			id       string
		}{
			{"nil", []flam.IScheduledJob{(*job)(nil)}, scheduler.ErrInvalidJob, ""},
			{"empty id", []flam.IScheduledJob{newJob("", nil)}, scheduler.ErrInvalidJob, ""},
			{"dotted id", []flam.IScheduledJob{newJob("a.b", nil)}, scheduler.ErrInvalidJob, "a.b"},
			{"duplicate id", []flam.IScheduledJob{newJob("a", nil), newJob("a", nil)}, scheduler.ErrDuplicateJob, "a"},
		}
		for _, s := range scenarios {
			t.Run(s.name, func(t *testing.T) {
				err := provideErr(t, options{values: hourly("a"), jobs: s.jobs})
				if !errors.Is(err, s.sentinel) {
					t.Fatalf("error = %v, want %v", err, s.sentinel)
				}
				e := asFlamError(t, err)
				testutil.Equal(t, "Error()", e.Error(), message+": "+s.sentinel.Error())
				testutil.Context(t, e, map[string]any{"id": s.id})
			})
		}
	})

	t.Run("should refuse an invalid field", func(t *testing.T) {
		scenarios := []struct {
			name   string
			values map[string]any
			field  string
		}{
			{"configuration not a bag", map[string]any{path: "@hourly"}, path},
			{"missing configuration", map[string]any{}, "schedule"},
			{"missing schedule", map[string]any{path: container.Bag{"active": true}}, "schedule"},
			{"blank schedule", map[string]any{path: container.Bag{"schedule": "  "}}, "schedule"},
			{"schedule not a string", map[string]any{path: container.Bag{"schedule": 5}}, "schedule"},
			{"seconds not a bool", with(hourly("a"), "a", container.Bag{"seconds": "yes"}), "seconds"},
			{"location not a string", with(hourly("a"), "a", container.Bag{"location": 1}), "location"},
			{"active not a bool", with(hourly("a"), "a", container.Bag{"active": "no"}), "active"},
			{"timeout not a duration", with(hourly("a"), "a", container.Bag{"timeout": "soon"}), "timeout"},
			{"timeout of another type", with(hourly("a"), "a", container.Bag{"timeout": 5}), "timeout"},
			{"negative timeout", with(hourly("a"), "a", container.Bag{"timeout": "-1s"}), "timeout"},
			{"singleton not a bool", with(hourly("a"), "a", container.Bag{"singleton": 1}), "singleton"},
			{"first invalid field", with(hourly("a"), "a", container.Bag{"seconds": 1, "active": 1}), "seconds"},
			{"locker not a string", with(hourly("a"), "a", container.Bag{"singleton": true, "locker": 1}), "locker"},
			{"lock ttl not a duration", with(hourly("a"), "a", container.Bag{"singleton": true, "lock_ttl": "x"}), "lock_ttl"},
			{"lock ttl too short", with(hourly("a"), "a", container.Bag{"singleton": true, "lock_ttl": "999us"}), "lock_ttl"},
		}
		for _, s := range scenarios {
			t.Run(s.name, func(t *testing.T) {
				err := provideErr(t, options{values: s.values, jobs: []flam.IScheduledJob{newJob("a", nil)}})
				testutil.Error(t, errorOf(t, err), scheduler.ErrInvalidField,
					message+": invalid scheduled job field", map[string]any{"id": "a", "field": s.field})
			})
		}
	})

	t.Run("should refuse an invalid default", func(t *testing.T) {
		scenarios := []struct {
			path  string
			value any
			field string
		}{
			{scheduler.DefaultLocationConfigPath, 1, "location"},
			{scheduler.DefaultTimeoutConfigPath, "-5s", "timeout"},
			{scheduler.DefaultLockerConfigPath, 1, "locker"},
			{scheduler.DefaultLockTTLConfigPath, "0s", "lock_ttl"},
		}
		for _, s := range scenarios {
			t.Run(s.field, func(t *testing.T) {
				values := with(hourly("a"), "a", container.Bag{"singleton": true})
				values[s.path] = s.value
				err := provideErr(t, options{values: values, jobs: []flam.IScheduledJob{newJob("a", nil)}})
				testutil.Error(t, errorOf(t, err), scheduler.ErrInvalidField,
					message+": invalid scheduled job field", map[string]any{"id": "a", "field": s.field})
			})
		}
	})

	t.Run("should refuse a singleton job without locker factory", func(t *testing.T) {
		values := with(hourly("a"), "a", container.Bag{"singleton": true, "locker": "main"})
		err := provideErr(t, options{values: values, jobs: []flam.IScheduledJob{newJob("a", nil)}})
		testutil.Error(t, errorOf(t, err), scheduler.ErrLocker,
			message+": unable to get scheduled job locker", map[string]any{"id": "a", "locker": "main"})
	})

	t.Run("should refuse a singleton job whose locker cannot be taken", func(t *testing.T) {
		f := &lockers{lockers: map[string]flam.ILocker{}}
		values := with(hourly("a"), "a", container.Bag{"singleton": true, "locker": "main"})
		err := provideErr(t, options{values: values, jobs: []flam.IScheduledJob{newJob("a", nil)}, lockers: f})
		e := errorOf(t, err)
		if !errors.Is(e, scheduler.ErrLocker) || !errors.Is(e, errBoom) {
			t.Fatalf("error = %v, want ErrLocker and boom", e)
		}
		testutil.Equal(t, "Error()", e.Error(), message+": unable to get scheduled job locker: boom")
		testutil.Context(t, asFlamError(t, e), map[string]any{"id": "a", "locker": "main"})
		testutil.Equal(t, "gets", f.gets, []string{"main"})
	})

	t.Run("should not read the locker fields of a job not singleton", func(t *testing.T) {
		values := with(hourly("a"), "a", container.Bag{"locker": 1, "lock_ttl": "x"})
		newProcess(t, options{values: values, jobs: []flam.IScheduledJob{newJob("a", nil)}})
	})

	t.Run("should take the locker of the default path", func(t *testing.T) {
		f := &lockers{lockers: map[string]flam.ILocker{"main": newLocker()}}
		values := with(hourly("a"), "a", container.Bag{"singleton": true})
		values[scheduler.DefaultLockerConfigPath] = "main"
		newProcess(t, options{values: values, jobs: []flam.IScheduledJob{newJob("a", nil)}, lockers: f})
		testutil.Equal(t, "gets", f.gets, []string{"main"})
	})
}

// errorOf returns the *flam.Error of the scheduler wrapped by the error of
// the container.
func errorOf(t *testing.T, err error) error {
	t.Helper()
	return asFlamError(t, err)
}

// ---------------------------------------------------------------------------
// run
// ---------------------------------------------------------------------------

func Test_Run(t *testing.T) {
	t.Run("should run each active job at the times of its schedule", func(t *testing.T) {
		a, b, c := newJob("a", nil), newJob("b", nil), newJob("c", nil)
		values := hourly("a", "b", "c")
		values = with(values, "b", container.Bag{"schedule": "*/20 * * * * *", "seconds": true})
		values = with(values, "c", container.Bag{"active": false})
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{a, b, c}})
		_, result := run(t, p)
		timers(t, clock, 2)
		testutil.Equal(t, "IsRunning()", p.IsRunning(), true)

		for i := range 3 {
			clock.Advance(20 * time.Second)
			receive(t, fmt.Sprintf("run %d of b", i), b.runs)
			timers(t, clock, 2)
		}
		clock.Advance(time.Hour - time.Minute)
		receive(t, "run of a", a.runs)
		staysQuiet(t, "run of c", c.runs)

		testutil.NoError(t, p.Close())
		testutil.NoError(t, receive(t, "result", result))
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should read the schedule in its location", func(t *testing.T) {
		j := newJob("a", nil)
		values := with(hourly("a"), "a", container.Bag{"schedule": "0 10 * * *"})
		values[scheduler.DefaultLocationConfigPath] = "Asia/Tokyo"
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{j}})
		run(t, p)
		timers(t, clock, 1)

		clock.Advance(time.Hour - time.Nanosecond) // 10:00 in Tokyo is 01:00 UTC
		staysQuiet(t, "early run", j.runs)
		clock.Advance(time.Nanosecond)
		receive(t, "run", j.runs)
	})

	t.Run("should signal the runs to the logger", func(t *testing.T) {
		j := newJob("a", nil)
		l := newLogger()
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}, logger: l})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)

		testutil.Equal(t, "started", l.next(t, flam.LogInfo, "scheduled job started", "a").ctx,
			container.Bag{"job": "a"})
		testutil.Equal(t, "done", l.next(t, flam.LogInfo, "scheduled job done", "a").ctx,
			container.Bag{"job": "a", "duration": time.Duration(0)})
	})

	t.Run("should signal on the configured channel", func(t *testing.T) {
		j := newJob("a", nil)
		l := newLogger()
		values := hourly("a")
		values[scheduler.LogChannelConfigPath] = "jobs"
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{j}, logger: l})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		testutil.Equal(t, "channel", receive(t, "entry", l.entries).channel, "jobs")
	})

	t.Run("should give the jobs a context cancelled when it stops", func(t *testing.T) {
		j := newJob("a", func(ctx context.Context) error { <-ctx.Done(); return nil })
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
		cancel, result := run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		ctx := receive(t, "run", j.runs)

		cancel()
		testutil.NoError(t, receive(t, "result", result))
		testutil.Equal(t, "Err()", ctx.Err(), context.Canceled)
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should run again once a run ended, unless closed", func(t *testing.T) {
		j := newJob("a", nil)
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
		cancel, result := run(t, p)
		timers(t, clock, 1)
		cancel()
		testutil.NoError(t, receive(t, "first result", result))

		_, result = run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		receive(t, "run", j.runs)
		testutil.NoError(t, p.Close())
		testutil.NoError(t, receive(t, "second result", result))

		err := p.Run(context.Background())
		testutil.Error(t, err, scheduler.ErrClosed, "unable to run scheduler: scheduler closed",
			map[string]any{"id": "scheduler"})
	})

	t.Run("should refuse a run while one is under way", func(t *testing.T) {
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{newJob("a", nil)}})
		run(t, p)
		timers(t, clock, 1)
		err := p.Run(context.Background())
		testutil.Error(t, err, scheduler.ErrRunning, "unable to run scheduler: scheduler running",
			map[string]any{"id": "scheduler"})
		testutil.Equal(t, "IsRunning()", p.IsRunning(), true)
	})

	t.Run("should fail when a trigger cannot be created, closing the others", func(t *testing.T) {
		values := with(hourly("a", "b"), "b", container.Bag{"schedule": "not a schedule"})
		p, clock := newProcess(t, options{values: values,
			jobs: []flam.IScheduledJob{newJob("a", nil), newJob("b", nil)}})
		err := p.Run(context.Background())
		if !errors.Is(err, scheduler.ErrSchedule) || !errors.Is(err, trigger.ErrInvalidSchedule) {
			t.Fatalf("Run() = %v, want ErrSchedule and ErrInvalidSchedule", err)
		}
		testutil.Context(t, asFlamError(t, err), map[string]any{"id": "b"})
		if !strings.HasPrefix(err.Error(), "unable to run scheduler: unable to schedule job: ") {
			t.Fatalf("Error() = %q", err.Error())
		}
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})
}

// ---------------------------------------------------------------------------
// close
// ---------------------------------------------------------------------------

func Test_Close(t *testing.T) {
	t.Run("should return nil when not running, and refuse a later run", func(t *testing.T) {
		p, _ := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{newJob("a", nil)}})
		testutil.NoError(t, p.Close())
		testutil.NoError(t, p.Close())
		err := p.Run(context.Background())
		testutil.Error(t, err, scheduler.ErrClosed, "unable to run scheduler: scheduler closed",
			map[string]any{"id": "scheduler"})
	})

	t.Run("should cancel and wait for the runs under way", func(t *testing.T) {
		var ended atomic.Bool
		j := newJob("a", func(ctx context.Context) error {
			<-ctx.Done()
			time.Sleep(quiet)
			ended.Store(true)
			return ctx.Err()
		})
		l := newLogger()
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}, logger: l})
		_, result := run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		receive(t, "run", j.runs)

		testutil.NoError(t, p.Close())
		testutil.Equal(t, "ended", ended.Load(), true)
		testutil.NoError(t, receive(t, "result", result))
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		testutil.NoError(t, p.Close())

		l.next(t, flam.LogInfo, "scheduled job started", "a")
		if err := l.failure(t, "a"); !errors.Is(err, scheduler.ErrJob) || !errors.Is(err, context.Canceled) {
			t.Fatalf("failure = %v, want ErrJob and context.Canceled", err)
		}
	})

	t.Run("should not start a run once stopping", func(t *testing.T) {
		for i := range 20 {
			j := newJob("a", nil)
			p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
			_, result := run(t, p)
			timers(t, clock, 1)
			go clock.Advance(time.Hour)
			testutil.NoError(t, p.Close())
			testutil.NoError(t, receive(t, fmt.Sprintf("result %d", i), result))
			select {
			case ctx := <-j.runs:
				if ctx.Err() != nil {
					continue // started before the stop, and cancelled by it
				}
				t.Fatalf("run %d with a live context after the stop", i)
			default:
			}
		}
	})
}

// ---------------------------------------------------------------------------
// overlap
// ---------------------------------------------------------------------------

func Test_Overlap(t *testing.T) {
	t.Run("should skip the times matched while the job runs", func(t *testing.T) {
		release := make(chan struct{})
		var running, overlaps atomic.Int32
		j := newJob("a", func(context.Context) error {
			if running.Add(1) > 1 {
				overlaps.Add(1)
			}
			<-release
			running.Add(-1)
			return nil
		})
		values := with(hourly("a"), "a", container.Bag{"schedule": "* * * * *"})
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{j}})
		run(t, p)
		timers(t, clock, 1)

		clock.Advance(time.Minute)
		receive(t, "first run", j.runs)
		for range 3 {
			clock.Advance(time.Minute)
		}
		staysQuiet(t, "overlapping run", j.runs)
		release <- struct{}{}
		timers(t, clock, 1)
		clock.Advance(time.Minute)
		receive(t, "next run", j.runs)
		release <- struct{}{}
		testutil.Equal(t, "overlaps", overlaps.Load(), int32(0))
	})
}

// ---------------------------------------------------------------------------
// failures
// ---------------------------------------------------------------------------

func Test_Failures(t *testing.T) {
	// fail runs the job of the fn once, and returns the failure signalled.
	fail := func(t *testing.T, fn func(ctx context.Context) error) error {
		t.Helper()
		j := newJob("a", fn)
		l := newLogger()
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}, logger: l})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		l.next(t, flam.LogInfo, "scheduled job started", "a")
		return l.failure(t, "a")
	}

	t.Run("should signal a returned error, wrapped", func(t *testing.T) {
		err := fail(t, func(context.Context) error { return errBoom })
		testutil.Error(t, err, scheduler.ErrJob,
			"unable to run scheduled job: scheduled job failed: boom", map[string]any{"id": "a"})
		if !errors.Is(err, errBoom) {
			t.Fatal("the failure does not wrap the error of the job")
		}
	})

	t.Run("should signal a panic, with its value and stack", func(t *testing.T) {
		err := fail(t, func(context.Context) error { panic("boom") })
		testutil.Equal(t, "Error()", err.Error(), "unable to run scheduled job: scheduled job panicked")
		ctx := asFlamError(t, err).Context()
		testutil.Equal(t, "panic", ctx[scheduler.ErrorFieldPanic], "boom")
		if stack, _ := ctx[scheduler.ErrorFieldStack].(string); !strings.Contains(stack, "goroutine") {
			t.Fatalf("stack = %q", stack)
		}
		if !errors.Is(err, scheduler.ErrPanic) {
			t.Fatal("the failure does not wrap ErrPanic")
		}
	})

	t.Run("should wrap the value of a panic with an error", func(t *testing.T) {
		err := fail(t, func(context.Context) error { panic(errBoom) })
		if !errors.Is(err, scheduler.ErrPanic) || !errors.Is(err, errBoom) {
			t.Fatalf("failure = %v, want ErrPanic and boom", err)
		}
		testutil.Equal(t, "Error()", err.Error(), "unable to run scheduled job: scheduled job panicked: boom")
	})

	t.Run("should signal a runtime.Goexit", func(t *testing.T) {
		err := fail(t, func(context.Context) error { runtime.Goexit(); return nil })
		testutil.Error(t, err, scheduler.ErrExit, "unable to run scheduled job: scheduled job exited",
			map[string]any{"id": "a"})
	})

	t.Run("should not signal a nil error held by an error interface as a failure", func(t *testing.T) {
		j := newJob("a", func(context.Context) error { return (*flam.Error)(nil) })
		l := newLogger()
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}, logger: l})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		l.next(t, flam.LogInfo, "scheduled job started", "a")
		l.next(t, flam.LogInfo, "scheduled job done", "a")
	})

	t.Run("should keep running the job after its failures", func(t *testing.T) {
		var n atomic.Int32
		j := newJob("a", func(context.Context) error {
			if n.Add(1) == 1 {
				panic("boom")
			}
			return errBoom
		})
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
		run(t, p)
		for i := range 3 {
			timers(t, clock, 1)
			clock.Advance(time.Hour)
			receive(t, fmt.Sprintf("run %d", i), j.runs)
		}
	})

	t.Run("should run without logger", func(t *testing.T) {
		j := newJob("a", func(context.Context) error { return errBoom })
		p, clock := newProcess(t, options{values: hourly("a"), jobs: []flam.IScheduledJob{j}})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		receive(t, "run", j.runs)
		timers(t, clock, 1)
	})
}

// ---------------------------------------------------------------------------
// timeouts
// ---------------------------------------------------------------------------

func Test_Timeout(t *testing.T) {
	// start runs the job of the fn, with the timeout set at the field, or
	// at its default path, once, returning the logger and the clock once
	// the run started, and the context of the run.
	begin := func(t *testing.T, fn func(ctx context.Context) error, values map[string]any) (*logger, *testutil.Clock, context.Context) {
		t.Helper()
		j := newJob("a", fn)
		l := newLogger()
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{j}, logger: l})
		run(t, p)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		ctx := receive(t, "run", j.runs)
		l.next(t, flam.LogInfo, "scheduled job started", "a")
		return l, clock, ctx
	}
	timeout := with(hourly("a"), "a", container.Bag{"timeout": "30s"})

	t.Run("should cancel the context once the timeout elapses", func(t *testing.T) {
		l, clock, ctx := begin(t, func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }, timeout)
		testutil.Equal(t, "timers during the run", clock.Timers(), 1)
		clock.Advance(30*time.Second - time.Nanosecond)
		staysQuiet(t, "early cancel", ctx.Done())
		clock.Advance(time.Nanosecond)

		err := l.failure(t, "a")
		testutil.Error(t, err, scheduler.ErrTimeout,
			"unable to run scheduled job: scheduled job timed out: unable to run scheduled job: scheduled job failed: context canceled",
			map[string]any{"id": "a", "timeout": 30 * time.Second})
		if !errors.Is(err, scheduler.ErrJob) || !errors.Is(err, context.Canceled) {
			t.Fatalf("failure = %v, want ErrJob and context.Canceled", err)
		}
		cause := context.Cause(ctx)
		testutil.Error(t, cause, scheduler.ErrTimeout, "unable to run scheduled job: scheduled job timed out",
			map[string]any{"id": "a", "timeout": 30 * time.Second})
	})

	t.Run("should signal a timeout though the job returned nil", func(t *testing.T) {
		values := hourly("a")
		values[scheduler.DefaultTimeoutConfigPath] = 30 * time.Second
		l, clock, _ := begin(t, func(ctx context.Context) error { <-ctx.Done(); return nil }, values)
		clock.Advance(30 * time.Second)

		err := l.failure(t, "a")
		testutil.Error(t, err, scheduler.ErrTimeout, "unable to run scheduled job: scheduled job timed out",
			map[string]any{"id": "a", "timeout": 30 * time.Second})
	})

	t.Run("should stop the timer of a job ending in time", func(t *testing.T) {
		l, clock, ctx := begin(t, nil, timeout)
		l.next(t, flam.LogInfo, "scheduled job done", "a")
		timers(t, clock, 1) // the trigger only
		clock.Advance(time.Minute)
		testutil.Equal(t, "cause", context.Cause(ctx), context.Canceled)
	})
}

// ---------------------------------------------------------------------------
// singleton
// ---------------------------------------------------------------------------

func Test_Singleton(t *testing.T) {
	// singleton returns a process running the job, a singleton of the
	// fields added, with the locker "main", and the logger and the clock.
	singleton := func(t *testing.T, j *job, l *locker, fields container.Bag, values map[string]any) (*logger, *testutil.Clock) {
		t.Helper()
		if values == nil {
			values = hourly("a")
		}
		fields["singleton"] = true
		fields["locker"] = "main"
		values = with(values, "a", fields)
		log := newLogger()
		f := &lockers{lockers: map[string]flam.ILocker{"main": l}}
		p, clock := newProcess(t, options{values: values, jobs: []flam.IScheduledJob{j}, logger: log, lockers: f})
		run(t, p)
		timers(t, clock, 1)
		return log, clock
	}

	t.Run("should run the job holding its lock", func(t *testing.T) {
		l := newLocker()
		var held atomic.Bool
		j := newJob("a", func(context.Context) error {
			l.mu.Lock()
			defer l.mu.Unlock()
			held.Store(l.held["flam.scheduler.jobs.a"])
			return nil
		})
		log, clock := singleton(t, j, l, container.Bag{}, nil)
		clock.Advance(time.Hour)
		log.next(t, flam.LogInfo, "scheduled job started", "a")
		log.next(t, flam.LogInfo, "scheduled job done", "a")
		eventually(t, "released", func() bool { _, released, _ := l.calls(); return len(released) == 1 })

		testutil.Equal(t, "held during the run", held.Load(), true)
		tries, released, releaseErrs := l.calls()
		testutil.Equal(t, "tries", tries, []tryCall{{"flam.scheduler.jobs.a", time.Minute}})
		testutil.Equal(t, "released", released, []string{"flam.scheduler.jobs.a"})
		testutil.Equal(t, "release context", releaseErrs, []error{nil})
	})

	t.Run("should skip the run when another instance holds the lock", func(t *testing.T) {
		l := newLocker()
		l.hold("flam.scheduler.jobs.a", true)
		j := newJob("a", nil)
		log, clock := singleton(t, j, l, container.Bag{}, nil)
		clock.Advance(time.Hour)
		testutil.Equal(t, "skipped", log.next(t, flam.LogDebug, "scheduled job skipped", "a").ctx,
			container.Bag{"job": "a"})
		staysQuiet(t, "run", j.runs)

		l.hold("flam.scheduler.jobs.a", false)
		timers(t, clock, 1)
		clock.Advance(time.Hour)
		receive(t, "run", j.runs)
	})

	t.Run("should signal a failure to acquire the lock", func(t *testing.T) {
		l := newLocker()
		l.tryErr = errBoom
		j := newJob("a", nil)
		log, clock := singleton(t, j, l, container.Bag{}, nil)
		clock.Advance(time.Hour)
		err := log.failure(t, "a")
		testutil.Error(t, err, scheduler.ErrLock,
			"unable to run scheduled job: unable to lock scheduled job: boom", map[string]any{"id": "a"})
		staysQuiet(t, "run", j.runs)
	})

	t.Run("should signal a failure to release the lock", func(t *testing.T) {
		l := newLocker()
		l.releaseErr = errBoom
		j := newJob("a", nil)
		log, clock := singleton(t, j, l, container.Bag{}, nil)
		clock.Advance(time.Hour)
		log.next(t, flam.LogInfo, "scheduled job started", "a")
		log.next(t, flam.LogInfo, "scheduled job done", "a")
		err := log.failure(t, "a")
		testutil.Error(t, err, scheduler.ErrLock,
			"unable to run scheduled job: unable to lock scheduled job: boom", map[string]any{"id": "a"})
	})

	t.Run("should release the lock of a job that panicked", func(t *testing.T) {
		l := newLocker()
		j := newJob("a", func(context.Context) error { panic("boom") })
		log, clock := singleton(t, j, l, container.Bag{}, nil)
		clock.Advance(time.Hour)
		log.next(t, flam.LogInfo, "scheduled job started", "a")
		if err := log.failure(t, "a"); !errors.Is(err, scheduler.ErrPanic) {
			t.Fatalf("failure = %v, want ErrPanic", err)
		}
		eventually(t, "released", func() bool { _, released, _ := l.calls(); return len(released) == 1 })
	})

	t.Run("should take the lock ttl of its field, its default, or the timeout", func(t *testing.T) {
		scenarios := []struct {
			name   string
			fields container.Bag
			values map[string]any
			ttl    time.Duration
		}{
			{"field", container.Bag{"lock_ttl": "10s", "timeout": "5m"}, nil, 10 * time.Second},
			{"default path", container.Bag{"timeout": "5m"},
				map[string]any{scheduler.DefaultLockTTLConfigPath: "20s"}, 20 * time.Second},
			{"timeout", container.Bag{"timeout": "5m"}, nil, 5 * time.Minute},
			{"built-in", container.Bag{}, nil, time.Minute},
		}
		for _, s := range scenarios {
			t.Run(s.name, func(t *testing.T) {
				values := hourly("a")
				for path, value := range s.values {
					values[path] = value
				}
				l := newLocker()
				j := newJob("a", nil)
				_, clock := singleton(t, j, l, s.fields, values)
				clock.Advance(time.Hour)
				receive(t, "run", j.runs)
				tries, _, _ := l.calls()
				testutil.Equal(t, "tries", tries, []tryCall{{"flam.scheduler.jobs.a", s.ttl}})
			})
		}
	})
}

// ---------------------------------------------------------------------------
// kennel
// ---------------------------------------------------------------------------

// runKennel runs the kennel of the application as the watchdog bundle
// does when the application runs.
func runKennel(a flam.IApp) error {
	return watchdog.NewBundle().(flam.IRunnableBundle).Run(a)
}
