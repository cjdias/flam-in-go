package rotatingfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/afero"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
	rotatingfile "github.com/cjdias/flam-in-go/pkg/flam/log/streams/rotating_file"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// ts is the time of the clock of the test applications, and the timestamp
// of the written entries.
var ts = time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC)

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// testApp is a testutil.App with the clock it provides.
type testApp struct {
	*testutil.App
	clock *testutil.Clock
}

// newApp returns a testApp with the config, disk, log and serializer
// bundles, the bundles given, and a configuration holding their defaults,
// then the values, at paths. Without the disk bundles when noDisks.
func newApp(t *testing.T, values map[string]any, noDisks bool, bundles ...flam.IBundle) *testApp {
	t.Helper()
	all := []flam.IBundle{config.NewBundle(), log.NewBundle(), stringify.NewBundle(), json.NewBundle()}
	if !noDisks {
		all = append(all, disk.NewBundle(), memory.NewBundle(), local.NewBundle())
	}
	all = append(all, bundles...)

	bag := container.Bag{}
	for _, b := range all {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := &testApp{App: testutil.NewApp(), clock: testutil.NewClock(ts)}
	testutil.NoError(t, app.Provide(func() flam.IClock { return app.clock }))
	for _, b := range all {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// creator returns the single stream creator of the app.
func creator(t *testing.T, app *testApp) flam.ILogStreamCreator {
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

// memDisk returns the memory disk of the app.
func memDisk(t *testing.T, app *testApp) flam.IDisk {
	t.Helper()
	var d flam.IDisk
	testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) (err error) {
		d, err = f.Get(memory.Id)
		return err
	}))
	return d
}

// streamConfig returns the configuration of a rotating file stream of id
// "s" on the memory disk, at "app-%s.log", with the stringify serializer,
// holding the fields.
func streamConfig(fields container.Bag) container.Bag {
	bag := container.Bag{
		factory.ConfigFieldId:              "s",
		factory.ConfigFieldDriver:          rotatingfile.Driver,
		rotatingfile.ConfigFieldDisk:       memory.Id,
		rotatingfile.ConfigFieldPath:       "app-%s.log",
		rotatingfile.ConfigFieldSerializer: stringify.Id,
	}
	for k, v := range fields {
		bag[k] = v
	}
	return bag
}

// newStream returns the stream created from streamConfig, with the app.
func newStream(t *testing.T, fields container.Bag) (flam.ILogStream, *testApp) {
	t.Helper()
	app := newApp(t, nil, false, rotatingfile.NewBundle())
	s, err := creator(t, app).Create(streamConfig(fields))
	testutil.NoError(t, err)
	return s, app
}

// content returns the content of the file of the memory disk.
func content(t *testing.T, app *testApp, path string) string {
	t.Helper()
	data, err := afero.ReadFile(memDisk(t, app), path)
	testutil.NoError(t, err)
	return string(data)
}

// line returns the stringify line of an info entry at the time.
func line(at time.Time, message string) string {
	return at.Format(stringify.DefaultFormat) + " [INFO] " + message + "\n"
}

// failingDisk is a flam.IDisk of id "failing" failing to open the files
// whose name holds fail, until fail is cleared.
type failingDisk struct {
	afero.Fs

	mu   sync.Mutex
	fail string
}

func (d *failingDisk) Id() string { return "failing" }

func (d *failingDisk) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.fail != "" && strings.Contains(name, d.fail) {
		return nil, errBoom
	}
	return d.Fs.OpenFile(name, flag, perm)
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", rotatingfile.NewBundle().Id(), "flam.log.streams.rotating_file")
	})

	t.Run("should set the defaults of the fields, and configure no stream", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, rotatingfile.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "level", bag.Get(rotatingfile.DefaultLevelConfigPath), "info")
		testutil.Equal(t, "serializer", bag.Get(rotatingfile.DefaultSerializerConfigPath), json.Id)
		testutil.Equal(t, "disk", bag.Get(rotatingfile.DefaultDiskConfigPath), local.Id)
		testutil.Equal(t, "streams", bag.Has(log.StreamConfigPath), false)
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		if err := rotatingfile.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": 1}); err == nil {
			t.Fatal("Config() = nil, want an error")
		}
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := rotatingfile.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creator(t, newApp(t, nil, false, rotatingfile.NewBundle()))
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": rotatingfile.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
	})

	t.Run("should create a stream of the configuration, opening the file of the day", func(t *testing.T) {
		s, app := newStream(t, container.Bag{
			rotatingfile.ConfigFieldLevel: "warning", rotatingfile.ConfigFieldChannels: []any{"b", "a"},
		})
		testutil.Equal(t, "Id()", s.Id(), "s")
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogWarning)
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string{"a", "b"})
		testutil.Equal(t, "file", content(t, app, "app-2026-11-02.log"), "")
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		app := newApp(t, map[string]any{
			rotatingfile.DefaultLevelConfigPath: "debug", rotatingfile.DefaultDiskConfigPath: memory.Id,
		}, false, rotatingfile.NewBundle())
		s, err := creator(t, app).Create(container.Bag{
			factory.ConfigFieldId: "s", rotatingfile.ConfigFieldPath: "app-%s.log",
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogDebug)
		testutil.NoError(t, s.Broadcast(ts, flam.LogDebug, "m"))
		testutil.Equal(t, "content", content(t, app, "app-2026-11-02.log"),
			`{"level":"DEBUG","message":"m","time":"2026-11-02T23:00:00.000+0000"}`+"\n")
	})

	t.Run("should take the built-in defaults", func(t *testing.T) {
		app := newApp(t, nil, false, bare{rotatingfile.NewBundle()})
		dir := t.TempDir()
		s, err := creator(t, app).Create(container.Bag{
			factory.ConfigFieldId: "s", rotatingfile.ConfigFieldPath: filepath.Join(dir, "app-%s.log"),
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		data, err := os.ReadFile(filepath.Join(dir, "app-2026-11-02.log"))
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(data),
			`{"level":"INFO","message":"m","time":"2026-11-02T23:00:00.000+0000"}`+"\n")
	})

	for name, fields := range map[string]container.Bag{
		"level":             {rotatingfile.ConfigFieldLevel: "verbose"},
		"channels":          {rotatingfile.ConfigFieldChannels: []any{""}},
		"serializer":        {rotatingfile.ConfigFieldSerializer: 1},
		"disk":              {rotatingfile.ConfigFieldDisk: ""},
		"path":              {rotatingfile.ConfigFieldPath: nil},
		"path without date": {rotatingfile.ConfigFieldPath: "app.log"},
		"path with 2 dates": {rotatingfile.ConfigFieldPath: "%s/app-%s.log"},
	} {
		t.Run("should refuse an invalid "+name, func(t *testing.T) {
			app := newApp(t, nil, false, rotatingfile.NewBundle())
			_, err := creator(t, app).Create(streamConfig(fields))
			testutil.Error(t, err, rotatingfile.ErrInvalidField,
				"unable to create rotating file log stream: invalid rotating file log stream field",
				map[string]any{rotatingfile.ErrorFieldID: "s", rotatingfile.ErrorFieldField: strings.Fields(name)[0]})
		})
	}

	t.Run("should fail for an unknown serializer", func(t *testing.T) {
		app := newApp(t, nil, false, rotatingfile.NewBundle())
		_, err := creator(t, app).Create(streamConfig(container.Bag{rotatingfile.ConfigFieldSerializer: "x"}))
		testutil.Error(t, err, rotatingfile.ErrSerializer, err.Error(),
			map[string]any{rotatingfile.ErrorFieldID: "s", rotatingfile.ErrorFieldSerializer: "x"})
	})

	t.Run("should fail for an unknown disk", func(t *testing.T) {
		app := newApp(t, nil, false, rotatingfile.NewBundle())
		_, err := creator(t, app).Create(streamConfig(container.Bag{rotatingfile.ConfigFieldDisk: "x"}))
		testutil.Error(t, err, rotatingfile.ErrDisk, err.Error(),
			map[string]any{rotatingfile.ErrorFieldID: "s", rotatingfile.ErrorFieldDisk: "x"})
	})

	t.Run("should fail without a disk factory", func(t *testing.T) {
		app := newApp(t, nil, true, rotatingfile.NewBundle())
		_, err := creator(t, app).Create(streamConfig(nil))
		testutil.Error(t, err, rotatingfile.ErrMissingFactory,
			"unable to create rotating file log stream: unable to get rotating file log stream disk: factory not provided by the container",
			map[string]any{rotatingfile.ErrorFieldID: "s", rotatingfile.ErrorFieldDisk: memory.Id})
	})

	t.Run("should fail when the file of the day cannot be opened", func(t *testing.T) {
		app := newApp(t, nil, false, rotatingfile.NewBundle())
		testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) error {
			return f.Store("failing", &failingDisk{Fs: afero.NewMemMapFs(), fail: "2026-11-02"})
		}))
		_, err := creator(t, app).Create(streamConfig(container.Bag{rotatingfile.ConfigFieldDisk: "failing"}))
		testutil.Error(t, err, rotatingfile.ErrOpen,
			"unable to create rotating file log stream: unable to open log file: boom",
			map[string]any{
				rotatingfile.ErrorFieldID: "s", rotatingfile.ErrorFieldDisk: "failing",
				rotatingfile.ErrorFieldPath: "app-2026-11-02.log",
			})
	})
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func Test_Stream(t *testing.T) {
	t.Run("should write each entry to the file of its day", func(t *testing.T) {
		s, app := newStream(t, nil)
		d1, d2, d4 := ts, ts.Add(time.Hour), ts.Add(49*time.Hour)
		testutil.NoError(t, s.Broadcast(d1, flam.LogInfo, "m1"))
		testutil.NoError(t, s.Broadcast(d1.Add(59*time.Minute), flam.LogInfo, "m2"))
		testutil.NoError(t, s.Broadcast(d2, flam.LogInfo, "m3"))
		testutil.NoError(t, s.Broadcast(d4, flam.LogInfo, "m4"))
		testutil.NoError(t, s.Broadcast(d1, flam.LogInfo, "late")) // back to its day
		testutil.NoError(t, s.Close())

		testutil.Equal(t, "day 1", content(t, app, "app-2026-11-02.log"),
			line(d1, "m1")+line(d1.Add(59*time.Minute), "m2")+line(d1, "late"))
		testutil.Equal(t, "day 2", content(t, app, "app-2026-11-03.log"), line(d2, "m3"))
		testutil.Equal(t, "day 4", content(t, app, "app-2026-11-05.log"), line(d4, "m4"))
	})

	t.Run("should write the entries of a day after midnight to the file of their day", func(t *testing.T) {
		s, app := newStream(t, nil)
		app.clock.Advance(2 * time.Hour) // flushed the next day
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		testutil.Equal(t, "day 1", content(t, app, "app-2026-11-02.log"), line(ts, "m"))
		if _, err := memDisk(t, app).Stat("app-2026-11-03.log"); err == nil {
			t.Error("the file of the next day was created")
		}
	})

	t.Run("should take the day in the location of the timestamp", func(t *testing.T) {
		s, app := newStream(t, nil)
		east := ts.In(time.FixedZone("", 2*3600)) // 01:00 on the next day
		testutil.NoError(t, s.Broadcast(east, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		testutil.Equal(t, "day 2", content(t, app, "app-2026-11-03.log"), line(east, "m"))
	})

	t.Run("should append to the existing file of the day", func(t *testing.T) {
		app := newApp(t, nil, false, rotatingfile.NewBundle())
		testutil.NoError(t, afero.WriteFile(memDisk(t, app), "app-2026-11-02.log", []byte("before\n"), 0o644))
		s, err := creator(t, app).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.Equal(t, "content", content(t, app, "app-2026-11-02.log"), "before\n"+line(ts, "m"))
	})

	t.Run("should keep the file of the day when the next one cannot be opened, then retry", func(t *testing.T) {
		app := newApp(t, nil, false, rotatingfile.NewBundle())
		failing := &failingDisk{Fs: afero.NewMemMapFs(), fail: "2026-11-03"}
		testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) error { return f.Store("failing", failing) }))
		s, err := creator(t, app).Create(streamConfig(container.Bag{rotatingfile.ConfigFieldDisk: "failing"}))
		testutil.NoError(t, err)

		next := ts.Add(time.Hour)
		err = s.Broadcast(next, flam.LogInfo, "lost")
		testutil.Error(t, err, rotatingfile.ErrOpen,
			"unable to write log entry: unable to rotate log file: unable to open log file: boom",
			map[string]any{log.ErrorFieldID: "s"})

		failing.mu.Lock()
		failing.fail = ""
		failing.mu.Unlock()
		testutil.NoError(t, s.Broadcast(next, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		data, err := afero.ReadFile(failing, "app-2026-11-03.log")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(data), line(next, "m"))
	})

	t.Run("should close the file of the day once", func(t *testing.T) {
		s, _ := newStream(t, nil)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		testutil.Error(t, s.Broadcast(ts, flam.LogInfo, "m"), log.ErrStreamClosed,
			"unable to write log entry: log stream closed", map[string]any{log.ErrorFieldID: "s"})
	})

	t.Run("should never interleave the lines of concurrent writes across days", func(t *testing.T) {
		s, app := newStream(t, nil)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				at := ts.Add(time.Duration(i%2) * time.Hour) // alternating days
				for range 50 {
					if err := s.Broadcast(at, flam.LogInfo, "m"); err != nil {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, s.Close())
		for _, at := range []time.Time{ts, ts.Add(time.Hour)} {
			text := content(t, app, "app-"+at.Format(rotatingfile.DateFormat)+".log")
			testutil.Equal(t, "lines", strings.Count(text, "\n"), 200)
			testutil.Equal(t, "entries", strings.Count(text, line(at, "m")), 200)
		}
	})
}
