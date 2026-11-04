package loggers

import (
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/loggers"
	"github.com/cjdias/flam-in-go/pkg/flam/rest/loggers/standard"
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
func create(t *testing.T, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IRestLoggerAdaptor, error) {
	t.Helper()
	return createWith(t, standard.NewBundle(), fields, values, logger)
}

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// createWith returns the adaptor "main" as create does, with the driver
// bundle given.
func createWith(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, logger flam.ILogger) (flam.IRestLoggerAdaptor, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), driver}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = standard.Driver
	}
	testutil.NoError(t, bag.Set(rest.LoggerConfigPath+".main", fields))
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

	var a flam.IRestLoggerAdaptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IRestLoggerAdaptorFactory) { a, err = f.Get("main") }))
	return a, err
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := loggers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.loggers")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.rest.loggers", "bundle:flam.rest.loggers.standard", "provider/1",
		})
	})

	t.Run("should configure the standard adaptor as the default", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.rest.loggers.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.LoggerConfigPath + "." + standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
			rest.DefaultLoggerConfigPath:              standard.Id,
			standard.DefaultChannelConfigPath:         "flam.rest",
			standard.DefaultLevelStartConfigPath:      "info",
			standard.DefaultLevelErrorConfigPath:      "error",
			standard.DefaultLevelShutdownConfigPath:   "info",
			standard.DefaultLevelDoneConfigPath:       "info",
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

		a.Start("api", "127.0.0.1:8080")
		a.Error("api", errBoom)
		a.Error("api", nil)
		a.Shutdown("api")
		a.Done("api")

		want := []struct {
			level   flam.LogLevel
			message string
			ctx     map[string]any
		}{
			{flam.LogInfo, "rest server started", map[string]any{
				standard.ContextFieldServer: "api", standard.ContextFieldAddress: "127.0.0.1:8080",
			}},
			{flam.LogError, "rest server failed", map[string]any{
				standard.ContextFieldServer: "api", standard.ContextFieldError: errBoom,
			}},
			{flam.LogError, "rest server failed", map[string]any{standard.ContextFieldServer: "api"}},
			{flam.LogInfo, "rest server shutting down", map[string]any{standard.ContextFieldServer: "api"}},
			{flam.LogInfo, "rest server done", map[string]any{standard.ContextFieldServer: "api"}},
		}
		testutil.Equal(t, "count", len(logger.signaled), len(want))
		for i, w := range want {
			got := logger.signaled[i]
			testutil.Equal(t, w.message+" level", got.level, w.level)
			testutil.Equal(t, w.message+" channel", got.channel, "flam.rest")
			testutil.Equal(t, w.message+" message", got.message, w.message)
			testutil.Equal(t, w.message+" context", map[string]any(got.ctx.(container.Bag)), w.ctx)
		}
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		logger := &entries{}
		a, err := create(t, container.Bag{
			standard.ConfigFieldChannel:       "http",
			standard.ConfigFieldLevelStart:    "debug",
			standard.ConfigFieldLevelError:    2,
			standard.ConfigFieldLevelShutdown: flam.LogWarning,
		}, map[string]any{standard.DefaultLevelDoneConfigPath: "notice"}, logger)
		testutil.NoError(t, err)

		a.Start("api", ":8080")
		a.Error("api", errBoom)
		a.Shutdown("api")
		a.Done("api")
		testutil.Equal(t, "levels", []flam.LogLevel{
			logger.signaled[0].level, logger.signaled[1].level, logger.signaled[2].level, logger.signaled[3].level,
		}, []flam.LogLevel{flam.LogDebug, flam.LogError, flam.LogWarning, flam.LogNotice})
		testutil.Equal(t, "channel", logger.signaled[0].channel, "http")
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		logger := &entries{}
		a, err := createWith(t, bare{standard.NewBundle()}, container.Bag{}, nil, logger)
		testutil.NoError(t, err)

		a.Error("api", errBoom)
		a.Shutdown("api")
		testutil.Equal(t, "error level", logger.signaled[0].level, flam.LogError)
		testutil.Equal(t, "shutdown level", logger.signaled[1].level, flam.LogInfo)
		testutil.Equal(t, "channel", logger.signaled[0].channel, "flam.rest")
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, val := range map[string]any{
			standard.ConfigFieldChannel:       "",
			standard.ConfigFieldLevelStart:    "loud",
			standard.ConfigFieldLevelError:    99,
			standard.ConfigFieldLevelShutdown: true,
			standard.ConfigFieldLevelDone:     []any{},
		} {
			_, err := create(t, container.Bag{field: val}, nil, &entries{})
			testutil.Error(t, errors.Unwrap(err), standard.ErrInvalidField,
				"unable to create standard rest logger: invalid standard rest logger field",
				map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldField: field})
		}
	})

	t.Run("should fail without logger in the container", func(t *testing.T) {
		_, err := create(t, container.Bag{}, nil, nil)
		testutil.Error(t, errors.Unwrap(err), standard.ErrMissingLogger,
			"unable to create standard rest logger: logger not provided by the container",
			map[string]any{standard.ErrorFieldID: "main"})
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := create(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, &entries{})
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
