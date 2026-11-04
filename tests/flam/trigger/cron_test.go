package trigger

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// start is the time of the fake clocks of the cron tests, a Thursday.
var start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// cronMessage is the message of the errors of NewCron.
const cronMessage = "unable to create cron trigger"

// newCronFactory returns the ICronTriggerFactory provided by the bundle,
// reading the time from a fake clock at now, and the clock.
func newCronFactory(t *testing.T, now time.Time) (flam.ICronTriggerFactory, *testutil.Clock) {
	t.Helper()
	clock := testutil.NewClock(now)
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, app.RegisterBundle(trigger.NewBundle()))

	var f flam.ICronTriggerFactory
	testutil.NoError(t, app.Invoke(func(provided flam.ICronTriggerFactory) { f = provided }))
	t.Cleanup(func() { _ = f.Close() })
	return f, clock
}

// armed waits for the trigger to wait for its next fire, and returns its
// time.
func armed(t *testing.T, tr flam.ICronTrigger) time.Time {
	t.Helper()
	eventually(t, "armed", func() bool { return !tr.Next().IsZero() })
	return tr.Next()
}

// fireAt waits for the trigger to wait for its next fire, checks that it
// is at want, and advances the clock to it.
func fireAt(t *testing.T, clock *testutil.Clock, tr flam.ICronTrigger, want time.Time) {
	t.Helper()
	next := armed(t, tr)
	if !next.Equal(want) {
		t.Fatalf("Next() = %v, want %v", next, want)
	}
	clock.Advance(clock.Until(next))
}

// ---------------------------------------------------------------------------
// creation
// ---------------------------------------------------------------------------

func Test_Cron_Creation(t *testing.T) {
	cb, _ := calls(nil)
	hourly := flam.CronSchedule{Spec: "@hourly"}

	t.Run("should refuse the invalid arguments, in order", func(t *testing.T) {
		scenarios := []struct {
			name     string
			id       string
			schedule flam.CronSchedule
			callback flam.TriggerCallback
			sentinel error
			text     string
			ctx      map[string]any
		}{
			{"should refuse an empty id", "", hourly, cb,
				trigger.ErrInvalidID, "invalid trigger id", map[string]any{}},
			{"should check the id first", "", flam.CronSchedule{}, nil,
				trigger.ErrInvalidID, "invalid trigger id", map[string]any{}},
			{"should refuse a nil callback", "a", hourly, nil,
				trigger.ErrNilCallback, "nil trigger callback", map[string]any{"id": "a"}},
			{"should check the callback before the schedule", "a", flam.CronSchedule{}, nil,
				trigger.ErrNilCallback, "nil trigger callback", map[string]any{"id": "a"}},
		}
		for _, s := range scenarios {
			t.Run(s.name, func(t *testing.T) {
				f, clock := newCronFactory(t, start)
				tr, err := f.NewCron(s.id, s.schedule, s.callback)
				if tr != nil {
					t.Fatal("NewCron() returned a trigger with an error")
				}
				testutil.Error(t, err, s.sentinel, cronMessage+": "+s.text, s.ctx)
				testutil.Equal(t, "timers", clock.Timers(), 0)
			})
		}
	})

	t.Run("should refuse an invalid schedule", func(t *testing.T) {
		scenarios := map[string]flam.CronSchedule{
			"empty":                     {},
			"blank":                     {Spec: "   "},
			"too few fields":            {Spec: "* * * *"},
			"seconds without option":    {Spec: "0 * * * * *"},
			"too many fields":           {Spec: "0 0 * * * * *", Seconds: true},
			"out of range":              {Spec: "61 * * * *"},
			"unknown descriptor":        {Spec: "@fortnightly"},
			"invalid every":             {Spec: "@every soon"},
			"zone without expression":   {Spec: "CRON_TZ=UTC"},
			"unknown zone prefix":       {Spec: "CRON_TZ=Nowhere/City * * * * *"},
			"unknown TZ prefix":         {Spec: "TZ=Nowhere/City @hourly"},
			"unknown location":          {Spec: "* * * * *", Location: "Nowhere/City"},
			"invalid with unknown zone": {Spec: "nonsense", Location: "Nowhere/City"},
		}
		for name, schedule := range scenarios {
			t.Run(name, func(t *testing.T) {
				f, clock := newCronFactory(t, start)
				tr, err := f.NewCron("a", schedule, cb)
				if tr != nil {
					t.Fatal("NewCron() returned a trigger with an error")
				}
				if !errors.Is(err, trigger.ErrInvalidSchedule) {
					t.Fatalf("NewCron() = %v, want ErrInvalidSchedule", err)
				}
				prefix := cronMessage + ": invalid trigger schedule: "
				if !strings.HasPrefix(err.Error(), prefix) || len(err.Error()) == len(prefix) {
					t.Fatalf("Error() = %q, want %q and the parsing error", err.Error(), prefix)
				}
				testutil.Context(t, asFlamError(t, err), map[string]any{"id": "a", "schedule": schedule})
				testutil.Equal(t, "timers", clock.Timers(), 0)
			})
		}
	})

	t.Run("should check the schedule before the closed factory", func(t *testing.T) {
		f, _ := newCronFactory(t, start)
		testutil.NoError(t, f.Close())
		_, err := f.NewCron("a", flam.CronSchedule{}, cb)
		if !errors.Is(err, trigger.ErrInvalidSchedule) {
			t.Fatalf("NewCron() = %v, want ErrInvalidSchedule", err)
		}
	})

	t.Run("should refuse to create once the factory is closed", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		testutil.NoError(t, f.Close())
		_, err := f.NewCron("a", hourly, cb)
		testutil.Error(t, err, trigger.ErrClosed, cronMessage+": trigger factory closed",
			map[string]any{"id": "a"})
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should share the ids with the other kinds of trigger", func(t *testing.T) {
		f, _ := newCronFactory(t, start)
		pulse, err := f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)
		_, err = f.NewCron("a", hourly, cb)
		testutil.Error(t, err, trigger.ErrDuplicateID, cronMessage+": trigger id already in use",
			map[string]any{"id": "a"})
		testutil.Equal(t, "IsRunning()", pulse.IsRunning(), true)

		cron, err := f.NewCron("b", hourly, cb)
		testutil.NoError(t, err)
		_, err = f.NewRecurring("b", delay, cb)
		testutil.Error(t, err, trigger.ErrDuplicateID,
			"unable to create recurring trigger: trigger id already in use", map[string]any{"id": "b"})
		_, err = f.NewCron("b", hourly, cb)
		testutil.Error(t, err, trigger.ErrDuplicateID, cronMessage+": trigger id already in use",
			map[string]any{"id": "b"})
		testutil.Equal(t, "IsRunning()", cron.IsRunning(), true)
	})

	t.Run("should check the closed factory before the duplicate id", func(t *testing.T) {
		f, _ := newCronFactory(t, start)
		_, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		testutil.NoError(t, f.Close())
		_, err = f.NewCron("a", hourly, cb)
		if !errors.Is(err, trigger.ErrClosed) {
			t.Fatalf("NewCron() = %v, want ErrClosed", err)
		}
	})

	t.Run("should reuse the id of a closed trigger", func(t *testing.T) {
		f, _ := newCronFactory(t, start)
		first, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		testutil.NoError(t, first.Close())

		second, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		testutil.Equal(t, "IsRunning()", second.IsRunning(), true)
	})

	t.Run("should return a running trigger with its id and schedule", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		schedule := flam.CronSchedule{Spec: "*/15 * * * *", Seconds: true, Location: "UTC"}
		tr, err := f.NewCron("a", schedule, cb)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", tr.Id(), "a")
		testutil.Equal(t, "Schedule()", tr.Schedule(), schedule)
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
		testutil.Equal(t, "Next()", armed(t, tr), start.Add(15*time.Minute))
		testutil.Equal(t, "timers", clock.Timers(), 1)
	})
}

// ---------------------------------------------------------------------------
// schedules
// ---------------------------------------------------------------------------

func Test_Cron_Schedules(t *testing.T) {
	t.Run("should fire at the times matched by the expression", func(t *testing.T) {
		scenarios := []struct {
			name     string
			schedule flam.CronSchedule
			want     []time.Time
		}{
			{"standard", flam.CronSchedule{Spec: "*/15 * * * *"}, []time.Time{
				start.Add(15 * time.Minute), start.Add(30 * time.Minute), start.Add(45 * time.Minute),
			}},
			{"weekdays", flam.CronSchedule{Spec: "30 2 * * 1-5"}, []time.Time{
				time.Date(2026, 1, 1, 2, 30, 0, 0, time.UTC), // Thursday
				time.Date(2026, 1, 2, 2, 30, 0, 0, time.UTC), // Friday
				time.Date(2026, 1, 5, 2, 30, 0, 0, time.UTC), // Monday
			}},
			{"trimmed", flam.CronSchedule{Spec: "  0 * * * *  "}, []time.Time{
				start.Add(time.Hour), start.Add(2 * time.Hour),
			}},
			{"seconds", flam.CronSchedule{Spec: "*/20 * * * * *", Seconds: true}, []time.Time{
				start.Add(20 * time.Second), start.Add(40 * time.Second), start.Add(time.Minute),
			}},
			{"standard with seconds allowed", flam.CronSchedule{Spec: "* * * * *", Seconds: true}, []time.Time{
				start.Add(time.Minute), start.Add(2 * time.Minute),
			}},
			{"hourly", flam.CronSchedule{Spec: "@hourly"}, []time.Time{
				start.Add(time.Hour), start.Add(2 * time.Hour),
			}},
			{"daily", flam.CronSchedule{Spec: "@daily"}, []time.Time{
				start.AddDate(0, 0, 1), start.AddDate(0, 0, 2),
			}},
			{"every", flam.CronSchedule{Spec: "@every 5m"}, []time.Time{
				start.Add(5 * time.Minute), start.Add(10 * time.Minute),
			}},
			{"every rounded to the second", flam.CronSchedule{Spec: "@every 1500ms"}, []time.Time{
				start.Add(time.Second), start.Add(2 * time.Second),
			}},
			{"every with a location", flam.CronSchedule{Spec: "@every 1h", Location: "Asia/Tokyo"}, []time.Time{
				start.Add(time.Hour), start.Add(2 * time.Hour),
			}},
		}
		for _, s := range scenarios {
			t.Run(s.name, func(t *testing.T) {
				f, clock := newCronFactory(t, start)
				cb, called := calls(nil)
				tr, err := f.NewCron("a", s.schedule, cb)
				testutil.NoError(t, err)

				for i, want := range s.want {
					next := armed(t, tr)
					if !next.Equal(want) {
						t.Fatalf("Next() %d = %v, want %v", i, next, want)
					}
					clock.Advance(clock.Until(next) - time.Nanosecond)
					staysQuiet(t, fmt.Sprintf("callback %d before its time", i), called)
					clock.Advance(time.Nanosecond)
					receive(t, fmt.Sprintf("callback %d", i), called)
				}
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
			})
		}
	})

	t.Run("should stop running when the schedule matches no time", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		cb, called := calls(nil)
		tr, err := f.NewCron("a", flam.CronSchedule{Spec: "0 0 30 2 *"}, cb) // February 30
		testutil.NoError(t, err)

		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
		testutil.Equal(t, "Next()", tr.Next(), time.Time{})
		testutil.Equal(t, "timers", clock.Timers(), 0)
		clock.Advance(10 * 365 * 24 * time.Hour)
		staysQuiet(t, "callback", called)

		_, err = f.NewCron("a", flam.CronSchedule{Spec: "@hourly"}, cb)
		testutil.NoError(t, err)
	})

	t.Run("should not fire twice for a time when the clock goes back", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		var once sync.Once
		cb, called := calls(func() error {
			once.Do(func() { clock.Advance(-30 * time.Second) })
			return nil
		})
		tr, err := f.NewCron("a", flam.CronSchedule{Spec: "* * * * *"}, cb)
		testutil.NoError(t, err)

		fireAt(t, clock, tr, start.Add(time.Minute))
		receive(t, "callback", called)
		testutil.Equal(t, "Next()", armed(t, tr), start.Add(2*time.Minute))
		testutil.Equal(t, "Now()", clock.Now(), start.Add(30*time.Second))
	})
}

// ---------------------------------------------------------------------------
// time zones
// ---------------------------------------------------------------------------

func Test_Cron_TimeZones(t *testing.T) {
	scenarios := []struct {
		name     string
		now      time.Time
		schedule flam.CronSchedule
		want     time.Time
	}{
		{"CRON_TZ prefix", start,
			flam.CronSchedule{Spec: "CRON_TZ=Asia/Tokyo 0 10 * * *"},
			time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)},
		{"TZ prefix", start,
			flam.CronSchedule{Spec: "TZ=Asia/Tokyo 0 10 * * *"},
			time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)},
		{"prefix with a descriptor and seconds", start,
			flam.CronSchedule{Spec: "CRON_TZ=America/New_York @daily", Seconds: true},
			time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)},
		{"location", start,
			flam.CronSchedule{Spec: "0 0 * * *", Location: "America/New_York"},
			time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)},
		{"location in summer time", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			flam.CronSchedule{Spec: "0 0 * * *", Location: "America/New_York"},
			time.Date(2026, 7, 1, 4, 0, 0, 0, time.UTC)},
		{"prefix over location", start,
			flam.CronSchedule{Spec: "CRON_TZ=Asia/Tokyo 0 10 * * *", Location: "America/New_York"},
			time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)},
		{"UTC location with a clock elsewhere", time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("X", 2*3600)),
			flam.CronSchedule{Spec: "0 3 * * *", Location: "UTC"},
			time.Date(2026, 1, 1, 3, 0, 0, 0, time.UTC)},
		{"clock location by default", time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("X", 2*3600)),
			flam.CronSchedule{Spec: "0 3 * * *"},
			time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)},
	}
	for _, s := range scenarios {
		t.Run("should read the expression in the zone of the "+s.name, func(t *testing.T) {
			f, clock := newCronFactory(t, s.now)
			cb, called := calls(nil)
			tr, err := f.NewCron("a", s.schedule, cb)
			testutil.NoError(t, err)

			next := armed(t, tr)
			if !next.Equal(s.want) {
				t.Fatalf("Next() = %v, want %v", next, s.want)
			}
			testutil.Equal(t, "Next() location", next.Location(), s.now.Location())
			clock.Advance(clock.Until(next))
			receive(t, "callback", called)
		})
	}
}

// ---------------------------------------------------------------------------
// firing
// ---------------------------------------------------------------------------

func Test_Cron_Firing(t *testing.T) {
	minutely := flam.CronSchedule{Spec: "* * * * *"}

	t.Run("should keep firing after each kind of failure", func(t *testing.T) {
		failures := map[string]struct {
			fail     func() error
			sentinel error
		}{
			"error":  {func() error { return errors.New("boom") }, trigger.ErrCallback},
			"panic":  {func() error { panic("boom") }, trigger.ErrCallbackPanic},
			"goexit": {func() error { runtime.Goexit(); return nil }, trigger.ErrCallbackExit},
		}
		for name, failure := range failures {
			t.Run(name, func(t *testing.T) {
				f, clock := newCronFactory(t, start)
				handler, reported := errorsTo()
				cb, called := calls(failure.fail)
				tr, err := f.NewCron("a", minutely, cb, nil, handler)
				testutil.NoError(t, err)

				for i := range 3 {
					fireAt(t, clock, tr, start.Add(time.Duration(i+1)*time.Minute))
					receive(t, fmt.Sprintf("callback %d", i), called)
					reportedErr := receive(t, fmt.Sprintf("failure %d", i), reported)
					if !errors.Is(reportedErr, failure.sentinel) {
						t.Fatalf("failure %d = %v, want %v", i, reportedErr, failure.sentinel)
					}
					testutil.Equal(t, "failure id", asFlamError(t, reportedErr).Context()[trigger.ErrorFieldID], "a")
				}
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
			})
		}
	})

	t.Run("should skip the times matched while its callback runs", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		release := make(chan struct{})
		var running, overlaps atomic.Int32
		cb, called := calls(func() error {
			if running.Add(1) > 1 {
				overlaps.Add(1)
			}
			<-release
			running.Add(-1)
			return nil
		})
		tr, err := f.NewCron("a", minutely, cb)
		testutil.NoError(t, err)

		fireAt(t, clock, tr, start.Add(time.Minute))
		receive(t, "first callback", called)
		testutil.Equal(t, "Next() while the callback runs", tr.Next(), time.Time{})
		testutil.Equal(t, "timers while the callback runs", clock.Timers(), 0)
		for range 3 {
			clock.Advance(time.Minute)
		}
		staysQuiet(t, "overlapping callback", called)

		release <- struct{}{}
		fireAt(t, clock, tr, start.Add(5*time.Minute))
		receive(t, "next callback", called)
		release <- struct{}{}
		testutil.Equal(t, "overlaps", overlaps.Load(), int32(0))
	})
}

// ---------------------------------------------------------------------------
// close
// ---------------------------------------------------------------------------

func Test_Cron_Close(t *testing.T) {
	hourly := flam.CronSchedule{Spec: "@hourly"}

	t.Run("should stop the trigger at once", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		cb, called := calls(nil)
		tr, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		armed(t, tr)

		testutil.NoError(t, tr.Close())
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
		testutil.Equal(t, "Next()", tr.Next(), time.Time{})
		testutil.Equal(t, "timers", clock.Timers(), 0)
		clock.Advance(10 * time.Hour)
		staysQuiet(t, "callback", called)
	})

	t.Run("should return nil when called again", func(t *testing.T) {
		f, _ := newCronFactory(t, start)
		cb, _ := calls(nil)
		tr, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		testutil.NoError(t, tr.Close())
		testutil.NoError(t, tr.Close())
	})

	t.Run("should stop a trigger closed before it armed its timer", func(t *testing.T) {
		for i := range 50 {
			f, clock := newCronFactory(t, start)
			cb, called := calls(nil)
			tr, err := f.NewCron(fmt.Sprintf("a%d", i), hourly, cb)
			testutil.NoError(t, err)
			testutil.NoError(t, tr.Close())
			eventually(t, "timers", func() bool { return clock.Timers() == 0 })
			clock.Advance(time.Hour)
			staysQuiet(t, "callback", called)
		}
	})

	t.Run("should not wait for a running callback, which is not called again", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		release := make(chan struct{})
		defer close(release)
		cb, called := calls(func() error { <-release; return nil })
		tr, err := f.NewCron("a", flam.CronSchedule{Spec: "* * * * *"}, cb)
		testutil.NoError(t, err)

		fireAt(t, clock, tr, start.Add(time.Minute))
		receive(t, "callback", called)
		testutil.NoError(t, tr.Close())
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
		release <- struct{}{}
		clock.Advance(time.Hour)
		staysQuiet(t, "second callback", called)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should be callable from the callback", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		var tr flam.ICronTrigger
		ready := make(chan struct{})
		cb, called := calls(func() error { <-ready; return tr.Close() })
		tr, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		close(ready)

		fireAt(t, clock, tr, start.Add(time.Hour))
		receive(t, "callback", called)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
		clock.Advance(10 * time.Hour)
		staysQuiet(t, "second callback", called)
		testutil.Equal(t, "timers", clock.Timers(), 0)

		_, err = f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
	})

	t.Run("should not affect a later trigger reusing the id", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		cb, _ := calls(nil)
		first, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		testutil.NoError(t, first.Close())

		cb2, called := calls(nil)
		second, err := f.NewCron("a", hourly, cb2)
		testutil.NoError(t, err)
		testutil.NoError(t, first.Close())
		testutil.Equal(t, "IsRunning()", second.IsRunning(), true)

		_, err = f.NewCron("a", hourly, cb)
		testutil.Error(t, err, trigger.ErrDuplicateID, cronMessage+": trigger id already in use",
			map[string]any{"id": "a"})
		fireAt(t, clock, second, start.Add(time.Hour))
		receive(t, "callback", called)
	})

	t.Run("should be closed with its factory", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		cb, called := calls(nil)
		tr, err := f.NewCron("a", hourly, cb)
		testutil.NoError(t, err)
		armed(t, tr)

		testutil.NoError(t, f.Close())
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
		testutil.Equal(t, "timers", clock.Timers(), 0)
		clock.Advance(10 * time.Hour)
		staysQuiet(t, "callback", called)
	})
}

// ---------------------------------------------------------------------------
// concurrency
// ---------------------------------------------------------------------------

func Test_Cron_Concurrency(t *testing.T) {
	t.Run("should be safe for concurrent use", func(t *testing.T) {
		f, clock := newCronFactory(t, start)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				cb, _ := calls(nil)
				tr, err := f.NewCron(fmt.Sprintf("c%d", i), flam.CronSchedule{Spec: "* * * * * *", Seconds: true}, cb)
				if err != nil {
					t.Errorf("NewCron() = %v", err)
					return
				}
				for range 20 {
					_ = tr.Next()
					_ = tr.IsRunning()
					clock.Advance(time.Second)
				}
				_ = tr.Close()
			})
		}
		wg.Wait()
		testutil.NoError(t, f.Close())
		eventually(t, "timers", func() bool { return clock.Timers() == 0 })
	})
}
