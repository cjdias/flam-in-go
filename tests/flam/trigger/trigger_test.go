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
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

const (
	patience = 2 * time.Second       // to wait for an expected event
	quiet    = 50 * time.Millisecond // to conclude that nothing happens
	delay    = time.Minute
)

// newFactory returns the ITriggerFactory provided by the bundle, reading
// the time from a fake clock, and the clock.
func newFactory(t *testing.T) (flam.ITriggerFactory, *testutil.Clock) {
	t.Helper()
	clock := testutil.NewClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, app.RegisterBundle(trigger.NewBundle()))

	var f flam.ITriggerFactory
	testutil.NoError(t, app.Invoke(func(provided flam.ITriggerFactory) { f = provided }))
	t.Cleanup(func() { _ = f.Close() })
	return f, clock
}

// creator is one of the creation methods of a factory.
type creator struct {
	name    string
	message string
	create  func(f flam.ITriggerFactory, id string, d time.Duration, cb flam.TriggerCallback, h ...flam.TriggerErrorHandler) (flam.ITrigger, error)
}

var creators = []creator{
	{"NewPulse", "unable to create pulse trigger", flam.ITriggerFactory.NewPulse},
	{"NewRecurring", "unable to create recurring trigger", flam.ITriggerFactory.NewRecurring},
}

// calls returns a callback sending on the returned channel each time it
// is called, then returning the result of fn, or nil when fn is nil.
func calls(fn func() error) (flam.TriggerCallback, chan struct{}) {
	ch := make(chan struct{}, 100)
	return func() error {
		ch <- struct{}{}
		if fn == nil {
			return nil
		}
		return fn()
	}, ch
}

// errorsTo returns a handler sending the errors it receives on the
// returned channel.
func errorsTo() (flam.TriggerErrorHandler, chan error) {
	ch := make(chan error, 100)
	return func(err error) { ch <- err }, ch
}

func receive[T any](t *testing.T, name string, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(patience):
		t.Fatalf("%s: nothing received", name)
		panic("unreachable")
	}
}

func staysQuiet[T any](t *testing.T, name string, ch <-chan T) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("%s: unexpected %v", name, v)
	case <-time.After(quiet):
	}
}

func eventually(t *testing.T, name string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(patience)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: condition never met", name)
		}
		runtime.Gosched()
	}
}

func asFlamError(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v (%T) is not a *flam.Error", err, err)
	}
	return e
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should have the documented sentinel messages", func(t *testing.T) {
		for err, text := range map[error]string{
			trigger.ErrInvalidID:     "invalid trigger id",
			trigger.ErrNilCallback:   "nil trigger callback",
			trigger.ErrInvalidDelay:  "invalid trigger delay",
			trigger.ErrClosed:        "trigger factory closed",
			trigger.ErrDuplicateID:   "trigger id already in use",
			trigger.ErrCallback:      "trigger callback failed",
			trigger.ErrCallbackPanic: "trigger callback panicked",
			trigger.ErrCallbackExit:  "trigger callback exited",
		} {
			testutil.Equal(t, "sentinel", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})

	t.Run("should have the documented context fields", func(t *testing.T) {
		testutil.Equal(t, "ErrorFieldID", trigger.ErrorFieldID, "id")
		testutil.Equal(t, "ErrorFieldDelay", trigger.ErrorFieldDelay, "delay")
		testutil.Equal(t, "ErrorFieldPanic", trigger.ErrorFieldPanic, "panic")
		testutil.Equal(t, "ErrorFieldStack", trigger.ErrorFieldStack, "stack")
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := trigger.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.trigger")
		if b == trigger.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
	})

	t.Run("should provide a single ITriggerFactory", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(time.Time{}) }))
		testutil.NoError(t, trigger.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "provider/0"})

		var first, second flam.ITriggerFactory
		testutil.NoError(t, app.Invoke(func(f flam.ITriggerFactory) { first = f }))
		testutil.NoError(t, app.Invoke(func(f flam.ITriggerFactory) { second = f }))
		if first == nil || first != second {
			t.Fatal("the container did not provide a single ITriggerFactory")
		}
	})

	t.Run("should need an IClock in the container", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(trigger.NewBundle()))
		err := app.Invoke(func(flam.ITriggerFactory) {})
		if err == nil || !strings.Contains(err.Error(), "flam.IClock") {
			t.Fatalf("Invoke() = %v, want a missing flam.IClock error", err)
		}
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		err := trigger.NewBundle().Register(app)
		if !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
		testutil.Context(t, asFlamError(t, err), map[string]any{
			bundle.ErrorFieldIndex: 0, bundle.ErrorFieldKind: "provider",
		})
	})

	t.Run("should close the factory and its running triggers", func(t *testing.T) {
		b := trigger.NewBundle()
		closable, ok := b.(flam.IClosableBundle)
		if !ok {
			t.Fatal("the bundle does not implement IClosableBundle")
		}
		app := testutil.NewApp()
		clock := testutil.NewClock(time.Time{})
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
		testutil.NoError(t, b.Register(app))
		var f flam.ITriggerFactory
		testutil.NoError(t, app.Invoke(func(provided flam.ITriggerFactory) { f = provided }))

		cb, called := calls(nil)
		tr, err := f.NewRecurring("a", delay, cb)
		testutil.NoError(t, err)

		testutil.NoError(t, closable.Close(app))
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
		clock.Advance(delay)
		staysQuiet(t, "callback", called)
		_, err = f.NewPulse("b", delay, cb)
		testutil.Error(t, err, trigger.ErrClosed,
			"unable to create pulse trigger: trigger factory closed", map[string]any{"id": "b"})

		testutil.NoError(t, closable.Close(app))
	})

	t.Run("should create and close the factory when never used", func(t *testing.T) {
		b := trigger.NewBundle()
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(time.Time{}) }))
		testutil.NoError(t, b.Register(app))
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))

		var f flam.ITriggerFactory
		testutil.NoError(t, app.Invoke(func(provided flam.ITriggerFactory) { f = provided }))
		cb, _ := calls(nil)
		_, err := f.NewPulse("a", delay, cb)
		testutil.Error(t, err, trigger.ErrClosed,
			"unable to create pulse trigger: trigger factory closed", map[string]any{"id": "a"})
	})

	t.Run("should fail to close when the factory cannot be created", func(t *testing.T) {
		b := trigger.NewBundle()
		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		if err := b.(flam.IClosableBundle).Close(app); err == nil {
			t.Fatal("Close() without IClock = nil, want an error")
		}
	})
}

// ---------------------------------------------------------------------------
// creation
// ---------------------------------------------------------------------------

func Test_Creation_Checks(t *testing.T) {
	cb, _ := calls(nil)

	for _, c := range creators {
		t.Run(c.name, func(t *testing.T) {
			scenarios := []struct {
				name     string
				id       string
				delay    time.Duration
				callback flam.TriggerCallback
				sentinel error
				text     string
				ctx      map[string]any
			}{
				{"should refuse an empty id", "", delay, cb,
					trigger.ErrInvalidID, "invalid trigger id", map[string]any{}},
				{"should check the id first", "", 0, nil,
					trigger.ErrInvalidID, "invalid trigger id", map[string]any{}},
				{"should refuse a nil callback", "a", delay, nil,
					trigger.ErrNilCallback, "nil trigger callback", map[string]any{"id": "a"}},
				{"should check the callback before the delay", "a", 0, nil,
					trigger.ErrNilCallback, "nil trigger callback", map[string]any{"id": "a"}},
				{"should refuse a zero delay", "a", 0, cb,
					trigger.ErrInvalidDelay, "invalid trigger delay",
					map[string]any{"id": "a", "delay": time.Duration(0)}},
				{"should refuse a negative delay", "a", -time.Nanosecond, cb,
					trigger.ErrInvalidDelay, "invalid trigger delay",
					map[string]any{"id": "a", "delay": -time.Nanosecond}},
			}
			for _, s := range scenarios {
				t.Run(s.name, func(t *testing.T) {
					f, clock := newFactory(t)
					tr, err := c.create(f, s.id, s.delay, s.callback)
					if tr != nil {
						t.Fatalf("%s() returned a trigger with an error", c.name)
					}
					testutil.Error(t, err, s.sentinel, c.message+": "+s.text, s.ctx)
					testutil.Equal(t, "timers", clock.Timers(), 0)
				})
			}

			t.Run("should check the arguments before the closed factory", func(t *testing.T) {
				f, _ := newFactory(t)
				testutil.NoError(t, f.Close())
				_, err := c.create(f, "a", 0, cb)
				testutil.Error(t, err, trigger.ErrInvalidDelay, c.message+": invalid trigger delay",
					map[string]any{"id": "a", "delay": time.Duration(0)})
			})

			t.Run("should refuse to create once the factory is closed", func(t *testing.T) {
				f, clock := newFactory(t)
				testutil.NoError(t, f.Close())
				_, err := c.create(f, "a", delay, cb)
				testutil.Error(t, err, trigger.ErrClosed, c.message+": trigger factory closed",
					map[string]any{"id": "a"})
				testutil.Equal(t, "timers", clock.Timers(), 0)
			})

			t.Run("should refuse the id of a running trigger of either kind", func(t *testing.T) {
				for _, other := range creators {
					f, _ := newFactory(t)
					first, err := other.create(f, "a", delay, cb)
					testutil.NoError(t, err)

					_, err = c.create(f, "a", delay, cb)
					testutil.Error(t, err, trigger.ErrDuplicateID, c.message+": trigger id already in use",
						map[string]any{"id": "a"})
					testutil.Equal(t, "IsRunning()", first.IsRunning(), true)
				}
			})

			t.Run("should check the closed factory before the duplicate id", func(t *testing.T) {
				f, _ := newFactory(t)
				_, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				testutil.NoError(t, f.Close())
				_, err = c.create(f, "a", delay, cb)
				if !errors.Is(err, trigger.ErrClosed) {
					t.Fatalf("%s() = %v, want ErrClosed", c.name, err)
				}
			})

			t.Run("should reuse the id of a closed trigger", func(t *testing.T) {
				f, _ := newFactory(t)
				first, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				testutil.NoError(t, first.Close())

				second, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				testutil.Equal(t, "IsRunning()", second.IsRunning(), true)
			})

			t.Run("should not share ids between factories", func(t *testing.T) {
				f1, _ := newFactory(t)
				f2, _ := newFactory(t)
				_, err := c.create(f1, "a", delay, cb)
				testutil.NoError(t, err)
				_, err = c.create(f2, "a", delay, cb)
				testutil.NoError(t, err)
			})

			t.Run("should return a running trigger with its id and delay", func(t *testing.T) {
				f, clock := newFactory(t)
				tr, err := c.create(f, "a", 3*time.Second, cb)
				testutil.NoError(t, err)
				testutil.Equal(t, "Id()", tr.Id(), "a")
				testutil.Equal(t, "Delay()", tr.Delay(), 3*time.Second)
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
				testutil.Equal(t, "timers", clock.Timers(), 1)
			})

			t.Run("should accept the smallest delay", func(t *testing.T) {
				f, clock := newFactory(t)
				cb, called := calls(nil)
				_, err := c.create(f, "a", time.Nanosecond, cb)
				testutil.NoError(t, err)
				clock.Advance(time.Nanosecond)
				receive(t, "callback", called)
			})
		})
	}
}

// ---------------------------------------------------------------------------
// pulse
// ---------------------------------------------------------------------------

func Test_Pulse(t *testing.T) {
	t.Run("should not fire before its delay", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(nil)
		_, err := f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay - time.Nanosecond)
		staysQuiet(t, "callback", called)
	})

	t.Run("should fire once at its delay, then stop running", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(nil)
		tr, err := f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "callback", called)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
		testutil.Equal(t, "timers", clock.Timers(), 0)

		clock.Advance(10 * delay)
		staysQuiet(t, "second callback", called)
	})

	t.Run("should be running until its callback has returned", func(t *testing.T) {
		f, clock := newFactory(t)
		release := make(chan struct{})
		cb, called := calls(func() error { <-release; return nil })
		tr, err := f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "callback", called)
		testutil.Equal(t, "IsRunning() during the callback", tr.IsRunning(), true)
		close(release)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
	})

	t.Run("should be running until the handlers have returned", func(t *testing.T) {
		f, clock := newFactory(t)
		release := make(chan struct{})
		handled := make(chan struct{})
		tr, err := f.NewPulse("a", delay, func() error { return errors.New("boom") },
			func(error) { close(handled); <-release })
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "handler", handled)
		testutil.Equal(t, "IsRunning() during the handler", tr.IsRunning(), true)
		close(release)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
	})

	t.Run("should free its id once it fired", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(nil)
		tr, err := f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)
		clock.Advance(delay)
		receive(t, "callback", called)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })

		_, err = f.NewPulse("a", delay, cb)
		testutil.NoError(t, err)
	})

	t.Run("should free its id before it stops running", func(t *testing.T) {
		f, clock := newFactory(t)
		for i := range 300 {
			cb, called := calls(nil)
			tr, err := f.NewPulse("a", delay, cb)
			if err != nil {
				t.Fatalf("NewPulse() #%d = %v", i, err)
			}
			clock.Advance(delay)
			receive(t, "callback", called)
			for tr.IsRunning() { // no pause, to catch the moment it stops
			}
		}
	})

	t.Run("should stop once its callback failed", func(t *testing.T) {
		f, clock := newFactory(t)
		handler, failures := errorsTo()
		cb, called := calls(func() error { return errors.New("boom") })
		tr, err := f.NewPulse("a", delay, cb, handler)
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "callback", called)
		_ = receive(t, "failure", failures)
		eventually(t, "IsRunning() false", func() bool { return !tr.IsRunning() })
	})
}

// ---------------------------------------------------------------------------
// recurring
// ---------------------------------------------------------------------------

func Test_Recurring(t *testing.T) {
	t.Run("should fire each time its delay elapses", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(nil)
		tr, err := f.NewRecurring("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay - time.Nanosecond)
		staysQuiet(t, "callback before the delay", called)
		for i := range 5 {
			clock.Advance(delay)
			receive(t, fmt.Sprintf("callback %d", i), called)
		}
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
	})

	t.Run("should keep firing after each kind of failure", func(t *testing.T) {
		failures := map[string]func() error{
			"error":  func() error { return errors.New("boom") },
			"panic":  func() error { panic("boom") },
			"goexit": func() error { runtime.Goexit(); return nil },
		}
		for name, fail := range failures {
			t.Run(name, func(t *testing.T) {
				f, clock := newFactory(t)
				handler, reported := errorsTo()
				cb, called := calls(fail)
				tr, err := f.NewRecurring("a", delay, cb, handler)
				testutil.NoError(t, err)

				for i := range 3 {
					clock.Advance(delay)
					receive(t, fmt.Sprintf("callback %d", i), called)
					_ = receive(t, fmt.Sprintf("failure %d", i), reported)
				}
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
			})
		}
	})

	t.Run("should never call its callback twice at once, dropping ticks", func(t *testing.T) {
		f, clock := newFactory(t)
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
		_, err := f.NewRecurring("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "first callback", called)
		for range 3 { // one tick is kept pending, the others are dropped
			clock.Advance(delay)
		}
		release <- struct{}{}
		receive(t, "pending tick callback", called)
		release <- struct{}{}
		staysQuiet(t, "dropped tick callback", called)
		testutil.Equal(t, "overlaps", overlaps.Load(), int32(0))
	})
}

// ---------------------------------------------------------------------------
// close
// ---------------------------------------------------------------------------

func Test_Close(t *testing.T) {
	for _, c := range creators {
		t.Run(c.name, func(t *testing.T) {
			t.Run("should stop the trigger at once", func(t *testing.T) {
				f, clock := newFactory(t)
				cb, called := calls(nil)
				tr, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)

				testutil.NoError(t, tr.Close())
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
				clock.Advance(10 * delay)
				staysQuiet(t, "callback", called)
				eventually(t, "timer stopped", func() bool { return clock.Timers() == 0 })
			})

			t.Run("should return nil when called again", func(t *testing.T) {
				f, _ := newFactory(t)
				cb, _ := calls(nil)
				tr, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				testutil.NoError(t, tr.Close())
				testutil.NoError(t, tr.Close())
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
			})

			t.Run("should not wait for a running callback, which is not called again", func(t *testing.T) {
				f, clock := newFactory(t)
				release := make(chan struct{})
				cb, called := calls(func() error { <-release; return nil })
				tr, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)

				clock.Advance(delay)
				receive(t, "callback", called)
				testutil.NoError(t, tr.Close()) // returns while the callback blocks
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
				testutil.Equal(t, "timers", clock.Timers(), 0) // stopped by Close itself
				close(release)
				clock.Advance(delay)
				staysQuiet(t, "callback after Close", called)
			})

			t.Run("should be callable from the callback", func(t *testing.T) {
				f, clock := newFactory(t)
				var tr flam.ITrigger
				ready := make(chan struct{})
				closed := make(chan error, 1)
				cb, called := calls(func() error { <-ready; closed <- tr.Close(); return nil })
				tr, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				close(ready)

				clock.Advance(delay)
				receive(t, "callback", called)
				testutil.NoError(t, receive(t, "Close() from the callback", closed))
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
				clock.Advance(delay)
				staysQuiet(t, "callback after Close", called)
			})

			t.Run("should not affect a later trigger reusing the id", func(t *testing.T) {
				f, clock := newFactory(t)
				cb, _ := calls(nil)
				first, err := c.create(f, "a", delay, cb)
				testutil.NoError(t, err)
				testutil.NoError(t, first.Close())

				cb2, called := calls(nil)
				second, err := c.create(f, "a", delay, cb2)
				testutil.NoError(t, err)
				testutil.NoError(t, first.Close())
				testutil.Equal(t, "IsRunning()", second.IsRunning(), true)
				_, err = c.create(f, "a", delay, cb)
				testutil.Error(t, err, trigger.ErrDuplicateID, c.message+": trigger id already in use",
					map[string]any{"id": "a"})
				clock.Advance(delay)
				receive(t, "callback of the later trigger", called)
			})
		})
	}
}

func Test_FactoryClose(t *testing.T) {
	t.Run("should close every running trigger", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(nil)
		var triggers []flam.ITrigger
		for _, c := range creators {
			for _, id := range []string{"a", "b"} {
				tr, err := c.create(f, c.name+id, delay, cb)
				testutil.NoError(t, err)
				triggers = append(triggers, tr)
			}
		}

		testutil.NoError(t, f.Close())
		for _, tr := range triggers {
			testutil.Equal(t, tr.Id()+" IsRunning()", tr.IsRunning(), false)
		}
		clock.Advance(10 * delay)
		staysQuiet(t, "callback", called)
		eventually(t, "timers stopped", func() bool { return clock.Timers() == 0 })
	})

	t.Run("should return nil when called again", func(t *testing.T) {
		f, _ := newFactory(t)
		testutil.NoError(t, f.Close())
		testutil.NoError(t, f.Close())
	})

	t.Run("should be callable from a callback", func(t *testing.T) {
		f, clock := newFactory(t)
		closed := make(chan error, 1)
		cb, called := calls(func() error { closed <- f.Close(); return nil })
		tr, err := f.NewRecurring("a", delay, cb)
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "callback", called)
		testutil.NoError(t, receive(t, "Close() from the callback", closed))
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), false)
	})
}

// ---------------------------------------------------------------------------
// failures
// ---------------------------------------------------------------------------

type wrapped struct{ error }

func Test_Failures(t *testing.T) {
	// fire creates a pulse with the callback and the handler, fires it, and
	// returns the error the handler received.
	fire := func(t *testing.T, cb flam.TriggerCallback) error {
		t.Helper()
		f, clock := newFactory(t)
		handler, reported := errorsTo()
		_, err := f.NewPulse("a", delay, cb, handler)
		testutil.NoError(t, err)
		clock.Advance(delay)
		return receive(t, "failure", reported)
	}

	t.Run("should report a returned error, wrapped", func(t *testing.T) {
		boom := errors.New("boom")
		err := fire(t, func() error { return boom })
		testutil.Error(t, err, trigger.ErrCallback,
			"unable to run trigger callback: trigger callback failed: boom", map[string]any{"id": "a"})
		if !errors.Is(err, boom) {
			t.Fatal("the reported error does not wrap the callback error")
		}
	})

	t.Run("should let errors.As find a returned *flam.Error", func(t *testing.T) {
		returned := flam.NewError("custom", map[string]any{"k": "v"})
		err := fire(t, func() error { return wrapped{returned} })
		var w wrapped
		if !errors.As(err, &w) || w.error != returned {
			t.Fatal("errors.As did not find the returned error")
		}
	})

	t.Run("should report a panic, with its value and stack", func(t *testing.T) {
		err := fire(t, func() error { panic("boom") })
		e := asFlamError(t, err)
		if !errors.Is(err, trigger.ErrCallbackPanic) {
			t.Fatalf("%v does not wrap ErrCallbackPanic", err)
		}
		testutil.Equal(t, "Error()", err.Error(), "unable to run trigger callback: trigger callback panicked")
		ctx := e.Context()
		testutil.Equal(t, "id", ctx[trigger.ErrorFieldID], "a")
		testutil.Equal(t, "panic", ctx[trigger.ErrorFieldPanic], "boom")
		stack, _ := ctx[trigger.ErrorFieldStack].(string)
		if !strings.Contains(stack, "trigger_test.go") {
			t.Fatalf("stack does not show the panicking callback:\n%s", stack)
		}
	})

	t.Run("should wrap the value of a panic with an error", func(t *testing.T) {
		boom := errors.New("boom")
		err := fire(t, func() error { panic(boom) })
		if !errors.Is(err, trigger.ErrCallbackPanic) || !errors.Is(err, boom) {
			t.Fatalf("%v does not wrap both ErrCallbackPanic and the panic error", err)
		}
		testutil.Equal(t, "Error()", err.Error(),
			"unable to run trigger callback: trigger callback panicked: boom")
	})

	t.Run("should report a panic with a nil value", func(t *testing.T) {
		err := fire(t, func() error { panic(nil) })
		var nilErr *runtime.PanicNilError
		if !errors.Is(err, trigger.ErrCallbackPanic) || !errors.As(err, &nilErr) {
			t.Fatalf("%v does not wrap ErrCallbackPanic and a *runtime.PanicNilError", err)
		}
	})

	t.Run("should report a runtime.Goexit", func(t *testing.T) {
		err := fire(t, func() error { runtime.Goexit(); return nil })
		testutil.Error(t, err, trigger.ErrCallbackExit,
			"unable to run trigger callback: trigger callback exited", map[string]any{"id": "a"})
	})

	t.Run("should not report a nil error held by an error interface", func(t *testing.T) {
		f, clock := newFactory(t)
		handler, reported := errorsTo()
		cb, called := calls(func() error { var e *flam.Error; return e })
		_, err := f.NewPulse("a", delay, cb, handler)
		testutil.NoError(t, err)
		clock.Advance(delay)
		receive(t, "callback", called)
		staysQuiet(t, "failure", reported)
	})

	t.Run("should drop the failures without handlers", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, called := calls(func() error { panic("boom") })
		tr, err := f.NewRecurring("a", delay, cb)
		testutil.NoError(t, err)
		for i := range 2 {
			clock.Advance(delay)
			receive(t, fmt.Sprintf("callback %d", i), called)
		}
		testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
	})
}

func Test_Handlers(t *testing.T) {
	t.Run("should call every handler, in order, skipping nil ones", func(t *testing.T) {
		f, clock := newFactory(t)
		var mu sync.Mutex
		var order []int
		done := make(chan struct{})
		handler := func(i int) flam.TriggerErrorHandler {
			return func(error) {
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				if i == 3 {
					close(done)
				}
			}
		}
		_, err := f.NewPulse("a", delay, func() error { return errors.New("boom") },
			handler(1), nil, handler(2), handler(3))
		testutil.NoError(t, err)

		clock.Advance(delay)
		receive(t, "last handler", done)
		mu.Lock()
		defer mu.Unlock()
		testutil.Equal(t, "order", order, []int{1, 2, 3})
	})

	t.Run("should give every handler the same error", func(t *testing.T) {
		f, clock := newFactory(t)
		first, firstErrs := errorsTo()
		second, secondErrs := errorsTo()
		_, err := f.NewPulse("a", delay, func() error { return errors.New("boom") }, first, second)
		testutil.NoError(t, err)

		clock.Advance(delay)
		if receive(t, "first", firstErrs) != receive(t, "second", secondErrs) {
			t.Fatal("the handlers received different errors")
		}
	})

	t.Run("should skip a handler that panics or exits", func(t *testing.T) {
		for name, bad := range map[string]flam.TriggerErrorHandler{
			"panic":  func(error) { panic("handler") },
			"goexit": func(error) { runtime.Goexit() },
		} {
			t.Run(name, func(t *testing.T) {
				f, clock := newFactory(t)
				handler, reported := errorsTo()
				cb, called := calls(func() error { return errors.New("boom") })
				tr, err := f.NewRecurring("a", delay, cb, bad, handler, bad)
				testutil.NoError(t, err)

				for i := range 2 {
					clock.Advance(delay)
					receive(t, fmt.Sprintf("callback %d", i), called)
					_ = receive(t, fmt.Sprintf("failure %d", i), reported)
				}
				testutil.Equal(t, "IsRunning()", tr.IsRunning(), true)
			})
		}
	})

	t.Run("should keep the handlers given at creation", func(t *testing.T) {
		f, clock := newFactory(t)
		handler, reported := errorsTo()
		other, otherReported := errorsTo()
		handlers := []flam.TriggerErrorHandler{handler}
		_, err := f.NewPulse("a", delay, func() error { return errors.New("boom") }, handlers...)
		testutil.NoError(t, err)
		handlers[0] = other

		clock.Advance(delay)
		_ = receive(t, "original handler", reported)
		staysQuiet(t, "replacing handler", otherReported)
	})
}

// ---------------------------------------------------------------------------
// concurrency
// ---------------------------------------------------------------------------

func Test_Concurrency(t *testing.T) {
	t.Run("should be safe for concurrent use", func(t *testing.T) {
		f, clock := newFactory(t)
		cb, _ := calls(nil)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				c := creators[i%2]
				tr, err := c.create(f, fmt.Sprint(i), delay, cb)
				if err != nil {
					t.Error(err)
					return
				}
				_ = tr.IsRunning()
				_ = tr.Delay()
				_ = tr.Close()
			})
		}
		wg.Go(func() { clock.Advance(delay) })
		wg.Wait()
		testutil.NoError(t, f.Close())
	})
}
