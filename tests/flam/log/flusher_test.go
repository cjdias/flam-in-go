package log

import (
	"errors"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Flusher_Boot(t *testing.T) {
	t.Run("should flush every second by default, and observe the period", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, unset, nil), triggers)
		testutil.NoError(t, a.boot())
		testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Second})
		testutil.Equal(t, "observing", a.observing(), true)
	})

	for name, value := range map[string]any{"not a period": "soon", "not a duration": 30} {
		t.Run("should flush every second for a "+name, func(t *testing.T) {
			triggers := &fakeTriggers{}
			a := newLogApp(t, defaults(t, value, nil), triggers)
			testutil.NoError(t, a.boot())
			testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Second})
		})
	}

	t.Run("should fail by default without a trigger factory", func(t *testing.T) {
		a := newLogApp(t, defaults(t, unset, nil), nil)
		testutil.Error(t, a.boot(), log.ErrMissingTriggerFactory,
			"unable to boot log flusher: trigger factory not provided by the container",
			map[string]any{log.ErrorFieldPeriod: time.Second})
	})

	for name, value := range map[string]any{
		"zero": time.Duration(0), "negative": -time.Second,
	} {
		t.Run("should not flush with a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			a := newLogApp(t, defaults(t, value, nil), triggers)
			testutil.NoError(t, a.boot())
			testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		})
	}

	for name, value := range map[string]any{"duration": 30 * time.Second, "string": "30s"} {
		t.Run("should flush the logger at a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			a := newLogApp(t, defaults(t, value, map[string][]string{"a": nil}), triggers)
			testutil.NoError(t, a.boot())

			created := triggers.all()
			testutil.Equal(t, "created", len(created), 1)
			testutil.Equal(t, "Id()", created[0].Id(), log.FlusherTriggerID)
			testutil.Equal(t, "running", triggers.running(), []time.Duration{30 * time.Second})

			a.logger(t).BroadcastInfo("m")
			created[0].fire()
			testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
		})
	}

	t.Run("should log a failed flush, written by the next flush", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "1s", map[string][]string{"a": {"write"}, "b": nil}), triggers)
		testutil.NoError(t, a.boot())
		a.logger(t).BroadcastInfo("m")
		created := triggers.all()[0]
		created.fire()
		testutil.Equal(t, "b", a.streams.get("b").messages(), []string{"m"})

		created.fire()
		got := a.streams.get("b").written()
		testutil.Equal(t, "b", len(got), 2)
		testutil.Equal(t, "message", got[1].message, "unable to flush log")
		testutil.Equal(t, "level", got[1].level, flam.LogError)
		testutil.Equal(t, "broadcast", got[1].broadcast, true)
		err, _ := got[1].ctx[0].Get(log.ErrorFieldError).(error)
		testutil.Error(t, err, errBoom, "unable to flush log entries to stream: boom",
			map[string]any{log.ErrorFieldID: "a"})
	})

	t.Run("should fail without a trigger factory for a positive period", func(t *testing.T) {
		a := newLogApp(t, defaults(t, "30s", nil), nil)
		testutil.Error(t, a.boot(), log.ErrMissingTriggerFactory,
			"unable to boot log flusher: trigger factory not provided by the container",
			map[string]any{log.ErrorFieldPeriod: 30 * time.Second})
		testutil.Equal(t, "observing", a.observing(), false)
	})

	t.Run("should boot without a trigger factory when not flushing", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, nil), nil)
		testutil.NoError(t, a.boot())
		testutil.Equal(t, "observing", a.observing(), true)
	})

	t.Run("should fail when the trigger creation fails", func(t *testing.T) {
		triggers := &fakeTriggers{fail: errBoom}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		err := a.boot()
		if !errors.Is(err, errBoom) || err.Error() != "unable to boot log flusher: boom" {
			t.Fatalf("Boot() = %v, want the trigger error", err)
		}
		testutil.Equal(t, "observing", a.observing(), false)
	})

	t.Run("should fail when the period is observed already", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.cfg.AddObserver(log.FlusherTriggerID, "other", func(_, _ any) {}))
		err := a.boot()
		if !errors.Is(err, config.ErrDuplicateObserver) {
			t.Fatalf("Boot() = %v, want ErrDuplicateObserver", err)
		}
		testutil.Equal(t, "created", len(triggers.all()), 0)
	})
}

func Test_Flusher_Period(t *testing.T) {
	t.Run("should restart the trigger when the period changes", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.boot())
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Minute})
		testutil.Equal(t, "created", len(triggers.all()), 2)
	})

	t.Run("should keep the trigger when the period is the same", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.boot())
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, 30*time.Second))
		testutil.Equal(t, "created", len(triggers.all()), 1)
	})

	t.Run("should replace a trigger that stopped running", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.boot())
		triggers.all()[0].stop()
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, 30*time.Second))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{30 * time.Second})
		testutil.Equal(t, "created", len(triggers.all()), 2)
	})

	for name, value := range map[string]any{"zero": "0s", "negative": "-1s"} {
		t.Run("should stop flushing on a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			a := newLogApp(t, defaults(t, "30s", nil), triggers)
			testutil.NoError(t, a.boot())
			testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, value))
			testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		})
	}

	t.Run("should start flushing on a positive period", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, nil, nil), triggers)
		testutil.NoError(t, a.boot())
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1s"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Second})
	})

	t.Run("should stop flushing when the trigger creation fails", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.boot())
		triggers.mu.Lock()
		triggers.fail = errBoom
		triggers.mu.Unlock()
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})

	t.Run("should stop flushing without a trigger factory", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, nil), nil)
		testutil.NoError(t, a.boot())
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.Equal(t, "observing", a.observing(), true)
	})
}

func Test_Flusher_Errors(t *testing.T) {
	t.Run("should log a change of the period failing to restart the trigger", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", map[string][]string{"a": nil}), triggers)
		testutil.NoError(t, a.boot())
		triggers.mu.Lock()
		triggers.fail = errBoom
		triggers.mu.Unlock()
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.NoError(t, a.logger(t).Flush())

		got := a.streams.get("a").written()
		testutil.Equal(t, "entries", len(got), 1)
		testutil.Equal(t, "message", got[0].message, "unable to restart log flusher")
		testutil.Equal(t, "level", got[0].level, flam.LogError)
		err, _ := got[0].ctx[0].Get(log.ErrorFieldError).(error)
		if !errors.Is(err, errBoom) {
			t.Fatalf("error = %v, want boom", err)
		}
	})

	t.Run("should neither flush nor log from the trigger once closed", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", map[string][]string{"b": nil}), triggers, func(app *testutil.App) {
			// the streams stay open, to tell what the trigger writes
			testutil.NoError(t, app.Decorate(func(f flam.ILogStreamFactory) flam.ILogStreamFactory {
				return &noClose{ILogStreamFactory: f}
			}))
		})
		testutil.NoError(t, a.boot())
		trigger := triggers.all()[0]
		testutil.NoError(t, a.close())

		l := a.logger(t)
		l.BroadcastInfo("after")
		trigger.fire() // fired before Close, run afterwards
		testutil.Equal(t, "flushed", a.streams.get("b").messages(), []string{})
		trigger.onError[0](errBoom)
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "logged", a.streams.get("b").messages(), []string{"after"})
	})
}

// noClose is a flam.ILogStreamFactory whose Close does nothing.
type noClose struct{ flam.ILogStreamFactory }

func (f *noClose) Close() error { return nil }

func Test_Flusher_Close(t *testing.T) {
	t.Run("should not restart the trigger from an observer call made once closed", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.boot())
		// called first, closing the bundle before the flusher observer runs
		testutil.NoError(t, a.cfg.AddObserver("a", log.FlusherPeriodConfigPath, func(_, _ any) {
			testutil.NoError(t, a.close())
		}))
		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.Equal(t, "created", len(triggers.all()), 1)
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})

	t.Run("should stop the trigger, stop observing, and flush", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", map[string][]string{"a": nil}), triggers)
		testutil.NoError(t, a.boot())
		a.logger(t).BroadcastInfo("m")
		testutil.NoError(t, a.close())
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		testutil.Equal(t, "observing", a.observing(), false)
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
		testutil.Equal(t, "closed", a.streams.get("a").isClosed(), true)

		testutil.NoError(t, a.cfg.Set(log.FlusherPeriodConfigPath, "1m"))
		testutil.Equal(t, "created", len(triggers.all()), 1)
	})

	t.Run("should return the errors of the trigger and of the flush", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", map[string][]string{"a": {"write"}}), triggers)
		testutil.NoError(t, a.boot())
		closeErr := errors.New("close")
		triggers.all()[0].closeErr = closeErr
		a.logger(t).BroadcastInfo("m")

		err := a.close()
		if !errors.Is(err, closeErr) || !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want the trigger and the flush errors", err)
		}
		var flushErr *flam.Error
		for _, e := range joined(t, err) {
			if errors.As(e, &flushErr) && errors.Is(e, errBoom) {
				break
			}
		}
		testutil.Equal(t, "Error()", flushErr.Error(),
			"unable to flush log on close: unable to flush log entries to stream: boom")
		testutil.Equal(t, "observing", a.observing(), false)
	})

	t.Run("should flush when never booted", func(t *testing.T) {
		a := newLogApp(t, defaults(t, "30s", map[string][]string{"a": nil}), &fakeTriggers{})
		_, err := a.factory(t).Get("a")
		testutil.NoError(t, err)
		a.logger(t).BroadcastInfo("m")
		testutil.NoError(t, a.close())
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
	})

	t.Run("should not start flushing when booted once closed", func(t *testing.T) {
		triggers := &fakeTriggers{}
		a := newLogApp(t, defaults(t, "30s", nil), triggers)
		testutil.NoError(t, a.close())
		testutil.NoError(t, a.boot())
		testutil.Equal(t, "created", len(triggers.all()), 0)
		testutil.Equal(t, "observing", a.observing(), false)
	})
}
