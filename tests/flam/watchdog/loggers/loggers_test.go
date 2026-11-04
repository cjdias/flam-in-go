package loggers

import (
	"errors"
	"fmt"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers/standard"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// entry is a signaled log entry.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     flam.IBag
}

// entries is a flam.ILogger recording the signaled entries.
type entries struct {
	flam.ILogger

	signaled []entry
}

func (e *entries) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	e.signaled = append(e.signaled, entry{level, channel, message, ctx[0]})
}

// create returns the adaptor "main" of the fields, created by the factory
// of an app with the standard bundle and the values, and the logger when
// not nil.
func create(t *testing.T, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IWatchdogLoggerAdaptor, error) {
	t.Helper()
	return createWith(t, standard.NewBundle(), fields, values, logger)
}

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// createWith returns the adaptor "main" as create does, with the driver
// bundle given.
func createWith(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IWatchdogLoggerAdaptor, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), watchdog.NewBundle(), driver}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = standard.Driver
	}
	testutil.NoError(t, bag.Set(watchdog.LoggerConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	if logger != nil {
		testutil.NoError(t, app.Provide(func() flam.ILogger { return logger }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var a flam.IWatchdogLoggerAdaptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IWatchdogLoggerAdaptorFactory) { a, err = f.Get("main") }))
	return a, err
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := loggers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.watchdog.loggers")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.watchdog.loggers", "bundle:flam.watchdog.loggers.standard", "provider/1",
		})
	})

	t.Run("should configure the standard adaptor as the default", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.watchdog.loggers.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			watchdog.LoggerConfigPath + "." + standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
			watchdog.DefaultLoggerConfigPath:              standard.Id,
			standard.DefaultChannelConfigPath:             "flam.watchdog",
			standard.DefaultLevelStartConfigPath:          "info",
			standard.DefaultLevelErrorConfigPath:          "error",
			standard.DefaultLevelDoneConfigPath:           "info",
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Adaptor(t *testing.T) {
	t.Run("should signal each report at the level of its kind", func(t *testing.T) {
		logger := &entries{}
		a, err := create(t, container.Bag{}, nil, logger)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", a.Id(), "main")

		a.Start("api")
		a.Error("api", errBoom)
		a.Done("api")

		want := []struct {
			level   flam.LogLevel
			message string
		}{
			{flam.LogInfo, "watchdog process started"},
			{flam.LogError, "watchdog process failed"},
			{flam.LogInfo, "watchdog process done"},
		}
		testutil.Equal(t, "count", len(logger.signaled), len(want))
		for i, w := range want {
			got := logger.signaled[i]
			testutil.Equal(t, w.message+" level", got.level, w.level)
			testutil.Equal(t, w.message+" channel", got.channel, "flam.watchdog")
			testutil.Equal(t, w.message+" message", got.message, w.message)
			testutil.Equal(t, w.message+" process", got.ctx.Get(standard.ContextFieldProcess), any("api"))
			var wantErr any
			if w.level == flam.LogError {
				wantErr = errBoom
			}
			testutil.Equal(t, w.message+" error", got.ctx.Get(standard.ContextFieldError), wantErr)
			testutil.Equal(t, w.message+" panic", got.ctx.Has(standard.ContextFieldPanic), false)
			testutil.Equal(t, w.message+" stack", got.ctx.Has(standard.ContextFieldStack), false)
		}
	})

	t.Run("should signal the value and the stack of a panic", func(t *testing.T) {
		logger := &entries{}
		a, err := create(t, container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		panicked := flam.NewWrappedError(fmt.Errorf("%w: again", watchdog.ErrPanic), "unable to run watchdog process").
			With(watchdog.ErrorFieldPanic, "again").
			With(watchdog.ErrorFieldStack, "goroutine 7")
		a.Error("api", panicked)
		a.Error("api", fmt.Errorf("%w: plain", watchdog.ErrPanic))
		testutil.Equal(t, "panic", logger.signaled[0].ctx.Get(standard.ContextFieldPanic), any("again"))
		testutil.Equal(t, "stack", logger.signaled[0].ctx.Get(standard.ContextFieldStack), any("goroutine 7"))
		testutil.Equal(t, "no panic", logger.signaled[1].ctx.Has(standard.ContextFieldPanic), false)
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger := &entries{}
		a, err := create(t, container.Bag{
			standard.ConfigFieldChannel: "jobs", standard.ConfigFieldLevelStart: "debug", standard.ConfigFieldLevelError: 2,
		}, map[string]any{standard.DefaultLevelDoneConfigPath: "notice"}, logger)
		testutil.NoError(t, err)

		a.Start("api")
		a.Error("api", errBoom)
		a.Done("api")
		testutil.Equal(t, "levels", []flam.LogLevel{logger.signaled[0].level, logger.signaled[1].level, logger.signaled[2].level},
			[]flam.LogLevel{flam.LogDebug, flam.LogError, flam.LogNotice})
		testutil.Equal(t, "channel", logger.signaled[0].channel, "jobs")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger := &entries{}
		a, err := createWith(t, bare{standard.NewBundle()}, container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		a.Error("api", errBoom)
		testutil.Equal(t, "level", logger.signaled[0].level, flam.LogError)
		testutil.Equal(t, "channel", logger.signaled[0].channel, "flam.watchdog")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			standard.ConfigFieldChannel:    "",
			standard.ConfigFieldLevelStart: "loud",
			standard.ConfigFieldLevelError: 99,
			standard.ConfigFieldLevelDone:  []any{},
		} {
			_, err := create(t, container.Bag{field: val}, nil, &entries{})
			testutil.Error(t, errors.Unwrap(err), standard.ErrInvalidField,
				"unable to create standard watchdog logger: invalid standard watchdog logger field",
				map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldField: field})
		}
	})

	t.Run("should fail without logger in the container", func(t *testing.T) {
		_, err := create(t, container.Bag{}, nil, nil)
		testutil.Error(t, errors.Unwrap(err), standard.ErrMissingLogger,
			"unable to create standard watchdog logger: logger not provided by the container",
			map[string]any{standard.ErrorFieldID: "main"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := create(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, &entries{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
