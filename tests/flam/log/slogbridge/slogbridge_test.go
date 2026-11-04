package slogbridge

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/log/slogbridge"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// entry is an entry signaled to a fake logger.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     []flam.IBag
}

// entries is a flam.ILogger recording the entries signaled, whose other
// methods panic.
type entries struct {
	flam.ILogger

	mu       sync.Mutex
	signaled []entry
}

func (e *entries) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.signaled = append(e.signaled, entry{level, channel, message, ctx})
}

// all returns a copy of the entries signaled.
func (e *entries) all() []entry {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]entry(nil), e.signaled...)
}

// contextEntries is a flam.IContextLogger recording the entries signaled
// with a context, along with the contexts, whose other methods panic.
type contextEntries struct {
	flam.IContextLogger

	recorded entries
	mu       sync.Mutex
	ctxs     []context.Context
}

func (e *contextEntries) SignalContext(ctx context.Context, level flam.LogLevel, channel, message string, bags ...flam.IBag) {
	e.mu.Lock()
	e.ctxs = append(e.ctxs, ctx)
	e.mu.Unlock()
	e.recorded.Signal(level, channel, message, bags...)
}

// ctxKey is the key of a value of the contexts of the tests.
type ctxKey struct{}

// plain returns the bag as nested map[string]any values, as slogtest
// expects them.
func plain(bag flam.IBag) map[string]any {
	m := map[string]any{}
	for key, val := range bag.All() {
		if nested, ok := val.(container.Bag); ok {
			m[key] = plain(nested)
			continue
		}
		m[key] = val
	}
	return m
}

func Test_Handler_Conformance(t *testing.T) {
	t.Run("should conform to slogtest", func(t *testing.T) {
		var logger *entries
		slogtest.Run(t, func(*testing.T) slog.Handler {
			logger = &entries{}
			return slogbridge.NewHandler(logger, &slogbridge.HandlerOptions{TimeField: slog.TimeKey})
		}, func(t *testing.T) map[string]any {
			got := logger.all()
			if len(got) != 1 {
				t.Fatalf("entries = %d, want 1", len(got))
			}
			m := plain(got[0].ctx[0])
			m[slog.LevelKey] = got[0].level
			m[slog.MessageKey] = got[0].message
			return m
		})
	})
}

func Test_Handler(t *testing.T) {
	t.Run("should signal the records on the channel, at the mapped level", func(t *testing.T) {
		logger := &entries{}
		l := slogbridge.NewLogger(logger, &slogbridge.HandlerOptions{Channel: "app", Level: slog.Level(-10)})
		l.Log(context.Background(), slog.Level(-5), "debug", "a", 1)
		l.Info("info")
		l.Log(context.Background(), slogbridge.LevelNotice, "notice")
		l.Warn("warning")
		l.Error("error")
		l.Log(context.Background(), slogbridge.LevelFatal, "fatal")
		testutil.Equal(t, "entries", logger.all(), []entry{
			{flam.LogDebug, "app", "debug", []flam.IBag{container.Bag{"a": int64(1)}}},
			{flam.LogInfo, "app", "info", []flam.IBag{container.Bag{}}},
			{flam.LogNotice, "app", "notice", []flam.IBag{container.Bag{}}},
			{flam.LogWarning, "app", "warning", []flam.IBag{container.Bag{}}},
			{flam.LogError, "app", "error", []flam.IBag{container.Bag{}}},
			{flam.LogFatal, "app", "fatal", []flam.IBag{container.Bag{}}},
		})
	})

	t.Run("should default to the slog channel, the info level and no time", func(t *testing.T) {
		for name, opts := range map[string]*slogbridge.HandlerOptions{"nil": nil, "zero": {}} {
			t.Run(name, func(t *testing.T) {
				logger := &entries{}
				h := slogbridge.NewHandler(logger, opts)
				testutil.Equal(t, "debug", h.Enabled(context.Background(), slog.LevelDebug), false)
				testutil.Equal(t, "info", h.Enabled(context.Background(), slog.LevelInfo), true)
				slog.New(h).Info("m")
				testutil.Equal(t, "entries", logger.all(), []entry{
					{flam.LogInfo, slogbridge.DefaultChannel, "m", []flam.IBag{container.Bag{}}},
				})
			})
		}
		testutil.Equal(t, "channel", slogbridge.DefaultChannel, "slog")
	})

	t.Run("should read the minimum level on every record", func(t *testing.T) {
		var level slog.LevelVar
		level.Set(slog.LevelError)
		logger := &entries{}
		l := slogbridge.NewLogger(logger, &slogbridge.HandlerOptions{Level: &level})
		l.Warn("dropped")
		level.Set(slog.LevelWarn)
		l.Warn("kept")
		testutil.Equal(t, "entries", len(logger.all()), 1)
		testutil.Equal(t, "message", logger.all()[0].message, "kept")
	})

	t.Run("should store the time of the record at the time field", func(t *testing.T) {
		logger := &entries{}
		h := slogbridge.NewHandler(logger, &slogbridge.HandlerOptions{TimeField: "at"})
		at := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
		testutil.NoError(t, h.Handle(context.Background(), slog.NewRecord(at, slog.LevelInfo, "m", 0)))
		testutil.Equal(t, "ctx", logger.all()[0].ctx, []flam.IBag{container.Bag{"at": at}})
	})

	t.Run("should nest the attributes in their groups", func(t *testing.T) {
		logger := &entries{}
		l := slogbridge.NewLogger(logger, nil).
			With("a", 1).
			WithGroup("g").
			With("b", "x", slog.Group("", slog.Int("c", 2)), slog.Group("empty")).
			WithGroup("").
			WithGroup("h")
		l.Info("m", "d", true, slog.Group("i", "e", 3*time.Second), slog.Group("i", "f", 1.5))
		l.Info("none")
		testutil.Equal(t, "entries", logger.all(), []entry{
			{flam.LogInfo, "slog", "m", []flam.IBag{container.Bag{
				"a": int64(1),
				"g": container.Bag{
					"b": "x",
					"c": int64(2),
					"h": container.Bag{"d": true, "i": container.Bag{"e": 3 * time.Second, "f": 1.5}},
				},
			}}},
			{flam.LogInfo, "slog", "none", []flam.IBag{container.Bag{"a": int64(1), "g": container.Bag{"b": "x", "c": int64(2)}}}},
		})
	})

	t.Run("should replace a value by a group of the same key", func(t *testing.T) {
		logger := &entries{}
		l := slogbridge.NewLogger(logger, nil).With("g", 1).WithGroup("g")
		l.Info("m", "a", 2)
		testutil.Equal(t, "ctx", logger.all()[0].ctx, []flam.IBag{container.Bag{"g": container.Bag{"a": int64(2)}}})
	})

	t.Run("should keep the handlers apart", func(t *testing.T) {
		logger := &entries{}
		base := slogbridge.NewLogger(logger, nil).WithGroup("g").With("a", 1)
		first, second := base.WithGroup("x"), base.WithGroup("y")
		first.Info("first", "b", 2)
		second.Info("second", "c", 3)
		base.With("d", 4).Info("third")
		base.Info("fourth")
		testutil.Equal(t, "entries", logger.all(), []entry{
			{flam.LogInfo, "slog", "first", []flam.IBag{container.Bag{"g": container.Bag{"a": int64(1), "x": container.Bag{"b": int64(2)}}}}},
			{flam.LogInfo, "slog", "second", []flam.IBag{container.Bag{"g": container.Bag{"a": int64(1), "y": container.Bag{"c": int64(3)}}}}},
			{flam.LogInfo, "slog", "third", []flam.IBag{container.Bag{"g": container.Bag{"a": int64(1), "d": int64(4)}}}},
			{flam.LogInfo, "slog", "fourth", []flam.IBag{container.Bag{"g": container.Bag{"a": int64(1)}}}},
		})
	})

	t.Run("should return the same handler for nothing to add", func(t *testing.T) {
		h := slogbridge.NewHandler(&entries{}, nil)
		if h.WithAttrs(nil) != h || h.WithAttrs([]slog.Attr{{}}) != h || h.WithGroup("") != h {
			t.Fatal("the handler changed for nothing to add")
		}
	})

	t.Run("should signal with the context when the logger takes one", func(t *testing.T) {
		logger := &contextEntries{}
		ctx := context.WithValue(context.Background(), ctxKey{}, "v")
		slogbridge.NewLogger(logger, nil).InfoContext(ctx, "m", "a", 1)
		testutil.Equal(t, "entries", logger.recorded.all(), []entry{
			{flam.LogInfo, "slog", "m", []flam.IBag{container.Bag{"a": int64(1)}}},
		})
		testutil.Equal(t, "contexts", len(logger.ctxs), 1)
		testutil.Equal(t, "context", logger.ctxs[0].Value(ctxKey{}), any("v"))
	})
}

func Test_Level(t *testing.T) {
	t.Run("should map the slog levels to the flam levels", func(t *testing.T) {
		for level, want := range map[slog.Level]flam.LogLevel{
			slog.Level(-100): flam.LogDebug, slog.LevelDebug: flam.LogDebug, slog.Level(-1): flam.LogDebug,
			slog.LevelInfo: flam.LogInfo, slog.Level(1): flam.LogInfo,
			slogbridge.LevelNotice: flam.LogNotice, slog.Level(3): flam.LogNotice,
			slog.LevelWarn: flam.LogWarning, slog.Level(7): flam.LogWarning,
			slog.LevelError: flam.LogError, slog.Level(11): flam.LogError,
			slogbridge.LevelFatal: flam.LogFatal, slogbridge.LevelNone: flam.LogFatal,
		} {
			testutil.Equal(t, level.String(), slogbridge.FlamLevel(level), want)
		}
	})

	t.Run("should map the flam levels to the slog levels", func(t *testing.T) {
		for level, want := range map[flam.LogLevel]slog.Level{
			flam.LogDebug: slog.LevelDebug, flam.LogInfo: slog.LevelInfo, flam.LogNotice: slogbridge.LevelNotice,
			flam.LogWarning: slog.LevelWarn, flam.LogError: slog.LevelError, flam.LogFatal: slogbridge.LevelFatal,
			flam.LogNone: slogbridge.LevelNone, flam.LogLevel(42): slogbridge.LevelNone,
		} {
			testutil.Equal(t, level.String(), slogbridge.SlogLevel(level), want)
			if level != flam.LogNone && level != flam.LogLevel(42) {
				testutil.Equal(t, "back", slogbridge.FlamLevel(want), level)
			}
		}
	})
}

// newBridgeApp returns a testutil.App with the config bundle registered,
// the logger provided, and the slogbridge bundle registered, whose
// configuration holds the defaults.
func newBridgeApp(t *testing.T, defaults container.Bag, logger flam.ILogger) (*testutil.App, flam.IBundle) {
	t.Helper()
	app := testutil.NewApp()
	b := slogbridge.NewBundle()
	testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, app.Provide(func() flam.ILogger { return logger }))
	testutil.NoError(t, app.RegisterBundle(b))
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(defaults) }))
	return app, b
}

// slogLogger returns the slog logger of the container.
func slogLogger(t *testing.T, app *testutil.App) *slog.Logger {
	t.Helper()
	var l *slog.Logger
	testutil.NoError(t, app.Invoke(func(logger *slog.Logger) { l = logger }))
	return l
}

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and paths", func(t *testing.T) {
		testutil.Equal(t, "Id()", slogbridge.NewBundle().Id(), "flam.log.slog")
		testutil.Equal(t, "channel", slogbridge.ChannelConfigPath, "flam.log.slog.channel")
		testutil.Equal(t, "level", slogbridge.LevelConfigPath, "flam.log.slog.level")
		testutil.Equal(t, "time", slogbridge.TimeFieldConfigPath, "flam.log.slog.time_field")
		testutil.Equal(t, "default", slogbridge.SetDefaultConfigPath, "flam.log.slog.set_default")
	})

	t.Run("should register its provider", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, slogbridge.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		boom := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return boom }
		if err := slogbridge.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})

	t.Run("should provide the logger of the defaults", func(t *testing.T) {
		logger := &entries{}
		app, _ := newBridgeApp(t, container.Bag{}, logger)
		l := slogLogger(t, app)
		if l != slogLogger(t, app) {
			t.Fatal("the container did not provide a single logger")
		}
		l.Debug("dropped")
		l.Info("m")
		testutil.Equal(t, "entries", logger.all(), []entry{{flam.LogInfo, "slog", "m", []flam.IBag{container.Bag{}}}})
	})

	t.Run("should provide the logger of the configuration", func(t *testing.T) {
		logger := &entries{}
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(slogbridge.ChannelConfigPath, "app"))
		testutil.NoError(t, bag.Set(slogbridge.LevelConfigPath, "warning"))
		testutil.NoError(t, bag.Set(slogbridge.TimeFieldConfigPath, "at"))
		app, _ := newBridgeApp(t, bag, logger)
		l := slogLogger(t, app)
		l.Info("dropped")
		at := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
		testutil.NoError(t, l.Handler().Handle(context.Background(), slog.NewRecord(at, slog.LevelWarn, "m", 0)))
		testutil.Equal(t, "entries", logger.all(), []entry{{flam.LogWarning, "app", "m", []flam.IBag{container.Bag{"at": at}}}})
	})

	t.Run("should take the info level for an invalid level, and disable the logger with none", func(t *testing.T) {
		for level, want := range map[string]bool{"bogus": true, "none": false} {
			bag := container.Bag{}
			testutil.NoError(t, bag.Set(slogbridge.LevelConfigPath, level))
			app, _ := newBridgeApp(t, bag, &entries{})
			l := slogLogger(t, app)
			testutil.Equal(t, level, l.Enabled(context.Background(), slog.LevelInfo), want)
			testutil.Equal(t, level+" debug", l.Enabled(context.Background(), slog.LevelDebug), false)
			testutil.Equal(t, level+" fatal", l.Enabled(context.Background(), slogbridge.LevelFatal), want)
		}
	})

	t.Run("should leave the default logger of slog by default", func(t *testing.T) {
		previous := slog.Default()
		app, b := newBridgeApp(t, container.Bag{}, &entries{})
		testutil.NoError(t, b.(flam.IBootableBundle).Boot(app))
		if slog.Default() != previous {
			t.Fatal("the default logger changed")
		}
	})

	t.Run("should set the default logger of slog when configured", func(t *testing.T) {
		previous := slog.Default()
		t.Cleanup(func() { slog.SetDefault(previous) })
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(slogbridge.SetDefaultConfigPath, true))
		app, b := newBridgeApp(t, bag, &entries{})
		testutil.NoError(t, b.(flam.IBootableBundle).Boot(app))
		if slog.Default() != slogLogger(t, app) {
			t.Fatal("the default logger is not the one of the container")
		}
	})

	t.Run("should fail to boot without logger", func(t *testing.T) {
		app := testutil.NewApp()
		b := slogbridge.NewBundle()
		testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, app.RegisterBundle(b))
		if err := b.(flam.IBootableBundle).Boot(app); err == nil {
			t.Fatal("Boot() = nil, want the error of the missing logger")
		}
	})
}
