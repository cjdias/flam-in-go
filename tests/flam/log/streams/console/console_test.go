package console

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/console"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// ts is the timestamp of the written entries.
var ts = time.Date(2026, 11, 2, 9, 43, 0, 0, time.UTC)

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// newApp returns a testutil.App with the config, log and serializer
// bundles, the bundle given, and a configuration holding their defaults,
// then the values, at paths.
func newApp(t *testing.T, values map[string]any, b flam.IBundle) *testutil.App {
	t.Helper()
	all := []flam.IBundle{config.NewBundle(), log.NewBundle(), stringify.NewBundle(), json.NewBundle(), b}
	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	for _, b := range all {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// creator returns the single stream creator of the app.
func creator(t *testing.T, app *testutil.App) flam.ILogStreamCreator {
	t.Helper()
	var creators []flam.ILogStreamCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ILogStreamCreator `group:"flam.log.streams.creator"`
	}) {
		creators = args.Creators
	}))
	testutil.Equal(t, "creators", len(creators), 1)
	return creators[0]
}

// capture replaces the output, os.Stdout or os.Stderr, with a temporary
// file until the end of the test, and returns a function reading it.
func capture(t *testing.T, output **os.File) func() string {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "output"))
	testutil.NoError(t, err)
	previous := *output
	*output = f
	t.Cleanup(func() {
		*output = previous
		_ = f.Close()
	})
	return func() string {
		data, err := os.ReadFile(f.Name())
		testutil.NoError(t, err)
		return string(data)
	}
}

// streamConfig returns the configuration of a console stream of id "s"
// holding the fields.
func streamConfig(fields container.Bag) container.Bag {
	bag := container.Bag{factory.ConfigFieldId: "s", factory.ConfigFieldDriver: console.Driver}
	for k, v := range fields {
		bag[k] = v
	}
	return bag
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", console.NewBundle().Id(), "flam.log.streams.console")
	})

	t.Run("should set the defaults of the fields, and configure no stream", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, console.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "level", bag.Get(console.DefaultLevelConfigPath), "info")
		testutil.Equal(t, "serializer", bag.Get(console.DefaultSerializerConfigPath), stringify.Id)
		testutil.Equal(t, "output", bag.Get(console.DefaultOutputConfigPath), console.OutputStdout)
		testutil.Equal(t, "streams", bag.Has(log.StreamConfigPath), false)
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		if err := console.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}); err == nil {
			t.Fatal("Config() = nil, want an error")
		}
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := console.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creator(t, newApp(t, nil, console.NewBundle()))
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": console.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
	})

	t.Run("should write to the standard output by default", func(t *testing.T) {
		read := capture(t, &os.Stdout)
		s, err := creator(t, newApp(t, nil, console.NewBundle())).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "s")
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Broadcast(ts, flam.LogDebug, "hidden"))
		testutil.Equal(t, "stdout", read(), "2026-11-02T09:43:00.000+0000 [INFO] m\n")
	})

	t.Run("should write to the standard error", func(t *testing.T) {
		read := capture(t, &os.Stderr)
		s, err := creator(t, newApp(t, nil, console.NewBundle())).Create(streamConfig(container.Bag{
			console.ConfigFieldOutput: console.OutputStderr, console.ConfigFieldSerializer: json.Id,
			console.ConfigFieldChannels: []string{"a"}, console.ConfigFieldLevel: flam.LogDebug,
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Signal(ts, flam.LogDebug, "a", "m"))
		testutil.Equal(t, "stderr", read(),
			`{"channel":"a","level":"DEBUG","message":"m","time":"2026-11-02T09:43:00.000+0000"}`+"\n")
	})

	t.Run("should take the output whatever its case", func(t *testing.T) {
		read := capture(t, &os.Stderr)
		s, err := creator(t, newApp(t, nil, console.NewBundle())).Create(streamConfig(container.Bag{
			console.ConfigFieldOutput: "STDERR", console.ConfigFieldLevel: "3",
		}))
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogWarning)
		testutil.NoError(t, s.Broadcast(ts, flam.LogWarning, "m"))
		testutil.Equal(t, "stderr", read(), "2026-11-02T09:43:00.000+0000 [WARNING] m\n")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		read := capture(t, &os.Stderr)
		app := newApp(t, map[string]any{
			console.DefaultLevelConfigPath: 2, console.DefaultSerializerConfigPath: json.Id,
			console.DefaultOutputConfigPath: console.OutputStderr,
		}, console.NewBundle())
		s, err := creator(t, app).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogError)
		testutil.NoError(t, s.Broadcast(ts, flam.LogError, "m"))
		testutil.Equal(t, "stderr", read(), `{"level":"ERROR","message":"m","time":"2026-11-02T09:43:00.000+0000"}`+"\n")
	})

	t.Run("should take the built-in defaults", func(t *testing.T) {
		read := capture(t, &os.Stdout)
		s, err := creator(t, newApp(t, nil, bare{console.NewBundle()})).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.Equal(t, "stdout", read(), "2026-11-02T09:43:00.000+0000 [INFO] m\n")
	})

	for name, fields := range map[string]container.Bag{
		"level":      {console.ConfigFieldLevel: 9},
		"channels":   {console.ConfigFieldChannels: 1},
		"serializer": {console.ConfigFieldSerializer: ""},
		"output":     {console.ConfigFieldOutput: "file"},
		"output ":    {console.ConfigFieldOutput: 1},
	} {
		t.Run("should refuse an invalid "+name, func(t *testing.T) {
			_, err := creator(t, newApp(t, nil, console.NewBundle())).Create(streamConfig(fields))
			testutil.Error(t, err, console.ErrInvalidField,
				"unable to create console log stream: invalid console log stream field",
				map[string]any{console.ErrorFieldID: "s", console.ErrorFieldField: strings.TrimSpace(name)})
		})
	}

	t.Run("should fail for an unknown serializer", func(t *testing.T) {
		_, err := creator(t, newApp(t, nil, console.NewBundle())).Create(
			streamConfig(container.Bag{console.ConfigFieldSerializer: "x"}))
		testutil.Error(t, err, console.ErrSerializer, err.Error(),
			map[string]any{console.ErrorFieldID: "s", console.ErrorFieldSerializer: "x"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Errorf("error = %v, want it to wrap ErrUnknownID", err)
		}
	})
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func Test_Stream(t *testing.T) {
	t.Run("should leave the output open when closed", func(t *testing.T) {
		read := capture(t, &os.Stdout)
		s, err := creator(t, newApp(t, nil, console.NewBundle())).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Close())
		testutil.Error(t, s.Broadcast(ts, flam.LogInfo, "m"), log.ErrStreamClosed,
			"unable to write log entry: log stream closed", map[string]any{log.ErrorFieldID: "s"})
		_, err = os.Stdout.WriteString("open\n")
		testutil.NoError(t, err)
		testutil.Equal(t, "stdout", read(), "open\n")
	})
}
