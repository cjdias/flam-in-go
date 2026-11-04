package config

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeTriggers is a flam.ITriggerFactory recording the recurring triggers
// it creates, without running them, or failing with fail.
type fakeTriggers struct {
	mu      sync.Mutex
	fail    error
	created []*fakeTrigger
}

func (f *fakeTriggers) Close() error { return nil }

func (f *fakeTriggers) NewPulse(string, time.Duration, flam.TriggerCallback, ...flam.TriggerErrorHandler) (flam.ITrigger, error) {
	panic("unexpected NewPulse")
}

func (f *fakeTriggers) NewRecurring(
	id string,
	delay time.Duration,
	callback flam.TriggerCallback,
	onError ...flam.TriggerErrorHandler,
) (flam.ITrigger, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	t := &fakeTrigger{id: id, delay: delay, callback: callback, running: true}
	f.created = append(f.created, t)
	return t, nil
}

// all returns the triggers created so far.
func (f *fakeTriggers) all() []*fakeTrigger {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*fakeTrigger(nil), f.created...)
}

// running returns the delays of the running triggers.
func (f *fakeTriggers) running() []time.Duration {
	delays := []time.Duration{}
	for _, t := range f.all() {
		if t.IsRunning() {
			delays = append(delays, t.delay)
		}
	}
	return delays
}

// fakeTrigger is the flam.ITrigger created by fakeTriggers, whose Close
// fails with closeErr.
type fakeTrigger struct {
	id       string
	delay    time.Duration
	callback flam.TriggerCallback
	closeErr error

	mu      sync.Mutex
	running bool
}

func (t *fakeTrigger) Id() string           { return t.id }
func (t *fakeTrigger) Delay() time.Duration { return t.delay }
func (t *fakeTrigger) IsRunning() bool      { t.mu.Lock(); defer t.mu.Unlock(); return t.running }
func (t *fakeTrigger) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running = false
	return t.closeErr
}
func (t *fakeTrigger) fire() error { return t.callback() }
func (t *fakeTrigger) stop()       { t.mu.Lock(); defer t.mu.Unlock(); t.running = false }

// period returns defaults holding the reload period.
func period(value any) container.Bag {
	bag := container.Bag{}
	if value != nil {
		_ = bag.Set(config.ReloadPeriodConfigPath, value)
	}
	return bag
}

// newReloadApp returns a testutil.App with a real configuration holding the
// defaults, the trigger factory when not nil, and a registered config
// bundle, returned to be booted and closed.
func newReloadApp(
	t *testing.T,
	defaults container.Bag,
	triggers flam.ITriggerFactory,
) (*testutil.App, flam.IConfig, flam.IBundle) {
	t.Helper()
	app := testutil.NewApp()
	if triggers != nil {
		testutil.NoError(t, app.Provide(func() flam.ITriggerFactory { return triggers }))
	}
	b := config.NewBundle()
	testutil.NoError(t, app.RegisterBundle(b))
	return app, appConfig(t, app, defaults), b
}

// bootBundle boots the bundle.
func bootBundle(app flam.IApp, b flam.IBundle) error {
	return b.(flam.IBootableBundle).Boot(app)
}

// closeBundle closes the bundle.
func closeBundle(app flam.IApp, b flam.IBundle) error {
	return b.(flam.IClosableBundle).Close(app)
}

// observing reports whether the reloader observes the period.
func observing(cfg flam.IConfig) bool {
	return cfg.HasObserver(config.ReloaderTriggerID, config.ReloadPeriodConfigPath)
}

// ---------------------------------------------------------------------------
// reloader
// ---------------------------------------------------------------------------

func Test_Reloader_Boot(t *testing.T) {
	t.Run("should not reload without a period, but observe it", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period(nil), triggers)
		testutil.NoError(t, bootBundle(app, b))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		testutil.Equal(t, "observing", observing(cfg), true)
	})

	for name, value := range map[string]any{
		"zero": time.Duration(0), "negative": -time.Second, "not a period": "soon", "not a duration": 30,
	} {
		t.Run("should not reload with a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			app, _, b := newReloadApp(t, period(value), triggers)
			testutil.NoError(t, bootBundle(app, b))
			testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		})
	}

	for name, value := range map[string]any{"duration": 30 * time.Second, "string": "30s"} {
		t.Run("should reload the sources at a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			app, cfg, b := newReloadApp(t, period(value), triggers)
			testutil.NoError(t, cfg.AddSource(reloadable("s", 0, container.Bag{"v": 1}, container.Bag{"v": 2})))
			testutil.NoError(t, bootBundle(app, b))

			created := triggers.all()
			testutil.Equal(t, "created", len(created), 1)
			testutil.Equal(t, "Id()", created[0].Id(), config.ReloaderTriggerID)
			testutil.Equal(t, "running", triggers.running(), []time.Duration{30 * time.Second})

			testutil.NoError(t, created[0].fire())
			testutil.Equal(t, "v", cfg.Int("v"), 2)
		})
	}

	t.Run("should return the errors of the reload to the trigger", func(t *testing.T) {
		boom := errors.New("boom")
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("1s"), triggers)
		failing := reloadable("s", 0, container.Bag{})
		failing.reload = func(*reloadSource) (bool, error) { return false, boom }
		testutil.NoError(t, cfg.AddSource(failing))
		testutil.NoError(t, bootBundle(app, b))
		if err := triggers.all()[0].fire(); !errors.Is(err, boom) {
			t.Fatalf("callback = %v, want boom", err)
		}
	})

	t.Run("should fail without a trigger factory for a positive period", func(t *testing.T) {
		app, cfg, b := newReloadApp(t, period("30s"), nil)
		err := bootBundle(app, b)
		testutil.Error(t, err, config.ErrMissingTriggerFactory,
			"unable to boot config reloader: trigger factory not provided by the container",
			map[string]any{config.ErrorFieldPeriod: 30 * time.Second})
		testutil.Equal(t, "observing", observing(cfg), false)
	})

	t.Run("should boot without a trigger factory when not reloading", func(t *testing.T) {
		app, cfg, b := newReloadApp(t, period(nil), nil)
		testutil.NoError(t, bootBundle(app, b))
		testutil.Equal(t, "observing", observing(cfg), true)
		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "1s")) // dropped
	})

	t.Run("should fail when the trigger cannot be created", func(t *testing.T) {
		boom := errors.New("boom")
		app, cfg, b := newReloadApp(t, period("30s"), &fakeTriggers{fail: boom})
		err := bootBundle(app, b)
		if !errors.Is(err, boom) {
			t.Fatalf("Boot() = %v, want boom", err)
		}
		testutil.Equal(t, "Error()", err.Error(), "unable to boot config reloader: boom")
		testutil.Equal(t, "observing", observing(cfg), false)
	})

	t.Run("should fail when the observer id is in use", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, cfg.AddObserver(config.ReloaderTriggerID, "other", func(_, _ any) {}))
		err := bootBundle(app, b)
		testutil.Error(t, err, config.ErrDuplicateObserver,
			"unable to boot config reloader: unable to add config observer: config observer id already in use",
			map[string]any{})
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})

	t.Run("should not start when the sources fail to boot", func(t *testing.T) {
		triggers := &fakeTriggers{}
		defaults := period("30s")
		testutil.NoError(t, defaults.Set(config.SourceConfigPath, container.Bag{"a": container.Bag{"driver": "none"}}))
		app, cfg, b := newReloadApp(t, defaults, triggers)
		if err := bootBundle(app, b); err == nil {
			t.Fatal("Boot() = nil, want the source error")
		}
		testutil.Equal(t, "observing", observing(cfg), false)
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})
}

func Test_Reloader_Period(t *testing.T) {
	t.Run("should restart the trigger when the period changes", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "1m"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Minute})
		testutil.Equal(t, "created", len(triggers.all()), 2)
	})

	t.Run("should keep the trigger when the period is the same", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, 30*time.Second))
		testutil.Equal(t, "created", len(triggers.all()), 1)
		testutil.Equal(t, "running", triggers.running(), []time.Duration{30 * time.Second})
	})

	t.Run("should restart a trigger that stopped running", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))
		triggers.all()[0].stop()

		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, 30*time.Second))
		testutil.Equal(t, "created", len(triggers.all()), 2)
		testutil.Equal(t, "running", triggers.running(), []time.Duration{30 * time.Second})
	})

	for name, value := range map[string]any{"zero": "0s", "negative": "-1s", "not a period": "soon"} {
		t.Run("should stop reloading on a "+name+" period", func(t *testing.T) {
			triggers := &fakeTriggers{}
			app, cfg, b := newReloadApp(t, period("30s"), triggers)
			testutil.NoError(t, bootBundle(app, b))

			testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, value))
			testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		})
	}

	t.Run("should start reloading on a positive period", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period(nil), triggers)
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "5s"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{5 * time.Second})
	})

	t.Run("should stop reloading when the trigger cannot be created", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))

		triggers.mu.Lock()
		triggers.fail = errors.New("boom")
		triggers.mu.Unlock()
		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "1m"))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})

	t.Run("should restart from the reload of a source changing the period", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, cfg.AddSource(reloadable("s", 0, container.Bag{}, period("1m"))))
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, triggers.all()[0].fire())
		testutil.Equal(t, "running", triggers.running(), []time.Duration{time.Minute})
	})
}

func Test_Reloader_Close(t *testing.T) {
	t.Run("should not restart the trigger from an observer call made once closed", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))
		// called first, closing the bundle before the reloader observer runs
		testutil.NoError(t, cfg.AddObserver("a", config.ReloadPeriodConfigPath, func(_, _ any) {
			testutil.NoError(t, closeBundle(app, b))
		}))
		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "1m"))
		testutil.Equal(t, "created", len(triggers.all()), 1)
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
	})

	t.Run("should neither observe nor reload when booted once closed", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, closeBundle(app, b))
		testutil.NoError(t, bootBundle(app, b))
		testutil.Equal(t, "observing", observing(cfg), false)
		testutil.Equal(t, "created", len(triggers.all()), 0)
	})

	t.Run("should stop the trigger and the observer", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, closeBundle(app, b))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		testutil.Equal(t, "observing", observing(cfg), false)

		testutil.NoError(t, cfg.Set(config.ReloadPeriodConfigPath, "1m"))
		testutil.Equal(t, "created", len(triggers.all()), 1)
		testutil.NoError(t, closeBundle(app, b))
	})

	t.Run("should stop the reloader booted by another bundle of the application", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))

		testutil.NoError(t, closeBundle(app, config.NewBundle()))
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		testutil.Equal(t, "observing", observing(cfg), false)
	})

	t.Run("should not remove the observer when never booted", func(t *testing.T) {
		app, cfg, b := newReloadApp(t, period("30s"), &fakeTriggers{})
		testutil.NoError(t, cfg.AddObserver(config.ReloaderTriggerID, config.ReloadPeriodConfigPath, func(_, _ any) {}))

		testutil.NoError(t, closeBundle(app, b))
		testutil.Equal(t, "observing", observing(cfg), true)
	})

	t.Run("should not restart from a reload under way", func(t *testing.T) {
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, cfg.AddSource(reloadable("s", 0, container.Bag{}, period("1m"))))
		testutil.NoError(t, bootBundle(app, b))
		testutil.NoError(t, cfg.AddObserver("closer", config.ReloadPeriodConfigPath, func(_, _ any) {
			testutil.NoError(t, closeBundle(app, b))
		}))

		testutil.NoError(t, triggers.all()[0].fire())
		testutil.Equal(t, "running", triggers.running(), []time.Duration{})
		testutil.Equal(t, "created", len(triggers.all()), 1)
	})

	t.Run("should return the error of the trigger, and close the factories", func(t *testing.T) {
		boom := errors.New("boom")
		triggers := &fakeTriggers{}
		app, cfg, b := newReloadApp(t, period("30s"), triggers)
		testutil.NoError(t, bootBundle(app, b))
		triggers.all()[0].closeErr = boom

		if err := closeBundle(app, b); !errors.Is(err, boom) {
			t.Fatalf("Close() = %v, want boom", err)
		}
		testutil.Equal(t, "observing", observing(cfg), false)
		testutil.NoError(t, app.Invoke(func(s flam.IConfigSourceFactory) {
			if err := s.Store("failing", &failingSource{}); !errors.Is(err, factory.ErrClosed) {
				t.Fatalf("source Store() after Close = %v, want factory.ErrClosed", err)
			}
		}))
	})
}

func Test_Reloader_Trigger(t *testing.T) {
	t.Run("should reload with the triggers of the trigger bundle", func(t *testing.T) {
		clk := testutil.NewClock(time.Time{})
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return clk }))
		tb := trigger.NewBundle()
		testutil.NoError(t, app.RegisterBundle(tb))
		b := config.NewBundle()
		testutil.NoError(t, app.RegisterBundle(b))
		cfg := appConfig(t, app, period("1s"))

		testutil.NoError(t, cfg.AddSource(reloadable("s", 0, container.Bag{"v": 1}, container.Bag{"v": 2})))
		changed := make(chan any, 1)
		testutil.NoError(t, cfg.AddObserver("v", "v", func(_, newValue any) { changed <- newValue }))
		testutil.NoError(t, bootBundle(app, b))

		clk.Advance(time.Second)
		select {
		case v := <-changed:
			testutil.Equal(t, "v", v, 2)
		case <-time.After(5 * time.Second):
			t.Fatal("the sources were not reloaded")
		}

		testutil.NoError(t, closeBundle(app, b))
		testutil.NoError(t, tb.(flam.IClosableBundle).Close(app))
	})
}
