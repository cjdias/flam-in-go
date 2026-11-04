package file

import (
	"errors"
	"fmt"
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
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/file"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// ts is the timestamp of the written entries.
var ts = time.Date(2026, 11, 2, 9, 43, 0, 0, time.UTC)

// newApp returns a testutil.App with the config, disk, log and serializer
// bundles, the bundles given, and a configuration holding their defaults,
// then the values, at paths. Without the disk bundles when noDisks.
func newApp(t *testing.T, values map[string]any, noDisks bool, bundles ...flam.IBundle) *testutil.App {
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
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return testutil.NewClock(ts) }))
	for _, b := range all {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// bare hides the defaults of a bundle.
type bare struct{ flam.IBundle }

// failingDisk is a flam.IDisk of id "failing" whose files fail to write
// and to close.
type failingDisk struct{ afero.Fs }

func (d *failingDisk) Id() string { return "failing" }

func (d *failingDisk) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	f, err := d.Fs.OpenFile(name, flag, perm)
	return &failingFile{File: f}, err
}

// failingFile is an afero.File failing to write and to close.
type failingFile struct{ afero.File }

func (f *failingFile) Write([]byte) (int, error)       { return 0, errBoom }
func (f *failingFile) WriteString(string) (int, error) { return 0, errBoom }
func (f *failingFile) Close() error                    { return errBoom }

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

// memDisk returns the memory disk of the app.
func memDisk(t *testing.T, app *testutil.App) flam.IDisk {
	t.Helper()
	var d flam.IDisk
	testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) (err error) {
		d, err = f.Get(memory.Id)
		return err
	}))
	return d
}

// streamConfig returns the configuration of a file stream of id "s" on the
// memory disk, at "log.txt", with the stringify serializer, holding the
// fields.
func streamConfig(fields container.Bag) container.Bag {
	bag := container.Bag{
		factory.ConfigFieldId:      "s",
		factory.ConfigFieldDriver:  file.Driver,
		file.ConfigFieldDisk:       memory.Id,
		file.ConfigFieldPath:       "log.txt",
		file.ConfigFieldSerializer: stringify.Id,
	}
	for k, v := range fields {
		bag[k] = v
	}
	return bag
}

// newStream returns the stream created from streamConfig, with the app.
func newStream(t *testing.T, fields container.Bag) (flam.ILogStream, *testutil.App) {
	t.Helper()
	app := newApp(t, nil, false, file.NewBundle())
	s, err := creator(t, app).Create(streamConfig(fields))
	testutil.NoError(t, err)
	return s, app
}

// content returns the content of the file of the memory disk.
func content(t *testing.T, app *testutil.App, path string) string {
	t.Helper()
	data, err := afero.ReadFile(memDisk(t, app), path)
	testutil.NoError(t, err)
	return string(data)
}

// line returns the stringify line of an entry at ts.
func line(level flam.LogLevel, message string, pairs ...string) string {
	l := fmt.Sprintf("2026-11-02T09:43:00.000+0000 [%s] %s", strings.ToUpper(level.String()), message)
	for _, p := range pairs {
		l += " " + p
	}
	return l + "\n"
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id", func(t *testing.T) {
		testutil.Equal(t, "Id()", file.NewBundle().Id(), "flam.log.streams.file")
	})

	t.Run("should set the defaults of the fields, and configure no stream", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, file.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "level", bag.Get(file.DefaultLevelConfigPath), "info")
		testutil.Equal(t, "serializer", bag.Get(file.DefaultSerializerConfigPath), json.Id)
		testutil.Equal(t, "disk", bag.Get(file.DefaultDiskConfigPath), local.Id)
		testutil.Equal(t, "streams", bag.Has(log.StreamConfigPath), false)
	})

	t.Run("should fail to configure a bag refusing the defaults", func(t *testing.T) {
		bag := container.Bag{"flam": 1}
		if err := file.NewBundle().(flam.IConfigurableBundle).Config(bag); err == nil {
			t.Fatal("Config() = nil, want an error")
		}
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := file.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept only its driver", func(t *testing.T) {
		c := creator(t, newApp(t, nil, false, file.NewBundle()))
		testutil.Equal(t, "Accept(driver)", c.Accept(container.Bag{"driver": file.Driver}), true)
		testutil.Equal(t, "Accept(other)", c.Accept(container.Bag{"driver": "other"}), false)
	})

	t.Run("should create a stream of the configuration", func(t *testing.T) {
		s, _ := newStream(t, container.Bag{
			file.ConfigFieldLevel: "warning", file.ConfigFieldChannels: []any{"b", "a", "b"},
		})
		testutil.Equal(t, "Id()", s.Id(), "s")
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogWarning)
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string{"a", "b"})
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		app := newApp(t, map[string]any{
			file.DefaultLevelConfigPath: "debug", file.DefaultDiskConfigPath: memory.Id,
		}, false, file.NewBundle())
		s, err := creator(t, app).Create(container.Bag{
			factory.ConfigFieldId: "s", file.ConfigFieldPath: "log.txt",
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogDebug)
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string(nil))
		testutil.NoError(t, s.Broadcast(ts, flam.LogDebug, "m", container.Bag{"a": 1}))
		testutil.Equal(t, "content", content(t, app, "log.txt"),
			`{"a":1,"level":"DEBUG","message":"m","time":"2026-11-02T09:43:00.000+0000"}`+"\n")
	})

	t.Run("should take the built-in defaults", func(t *testing.T) {
		app := newApp(t, nil, false, bare{file.NewBundle()})
		path := filepath.Join(t.TempDir(), "log.txt")
		s, err := creator(t, app).Create(container.Bag{factory.ConfigFieldId: "s", file.ConfigFieldPath: path})
		testutil.NoError(t, err)
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		data, err := os.ReadFile(path)
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(data),
			`{"level":"INFO","message":"m","time":"2026-11-02T09:43:00.000+0000"}`+"\n")
	})

	for name, fields := range map[string]container.Bag{
		"level":         {file.ConfigFieldLevel: "verbose"},
		"channels":      {file.ConfigFieldChannels: "a"},
		"channel":       {file.ConfigFieldChannels: []any{"a", 1}},
		"empty channel": {file.ConfigFieldChannels: []string{"a", ""}},
		"serializer":    {file.ConfigFieldSerializer: ""},
		"disk":          {file.ConfigFieldDisk: 1},
		"path":          {file.ConfigFieldPath: ""},
	} {
		t.Run("should refuse an invalid "+name, func(t *testing.T) {
			app := newApp(t, nil, false, file.NewBundle())
			field := strings.Fields(name)[len(strings.Fields(name))-1]
			if field == "channel" {
				field = "channels"
			}
			_, err := creator(t, app).Create(streamConfig(fields))
			testutil.Error(t, err, file.ErrInvalidField,
				"unable to create file log stream: invalid file log stream field",
				map[string]any{file.ErrorFieldID: "s", file.ErrorFieldField: field})
		})
	}

	t.Run("should fail for an unknown serializer", func(t *testing.T) {
		app := newApp(t, nil, false, file.NewBundle())
		_, err := creator(t, app).Create(streamConfig(container.Bag{file.ConfigFieldSerializer: "x"}))
		testutil.Error(t, err, file.ErrSerializer, err.Error(),
			map[string]any{file.ErrorFieldID: "s", file.ErrorFieldSerializer: "x"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Errorf("error = %v, want it to wrap ErrUnknownID", err)
		}
	})

	t.Run("should fail for an unknown disk", func(t *testing.T) {
		app := newApp(t, nil, false, file.NewBundle())
		_, err := creator(t, app).Create(streamConfig(container.Bag{file.ConfigFieldDisk: "x"}))
		testutil.Error(t, err, file.ErrDisk, err.Error(),
			map[string]any{file.ErrorFieldID: "s", file.ErrorFieldDisk: "x"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Errorf("error = %v, want it to wrap ErrUnknownID", err)
		}
	})

	t.Run("should fail without a disk factory", func(t *testing.T) {
		app := newApp(t, nil, true, file.NewBundle())
		_, err := creator(t, app).Create(streamConfig(nil))
		testutil.Error(t, err, file.ErrMissingFactory,
			"unable to create file log stream: unable to get file log stream disk: factory not provided by the container",
			map[string]any{file.ErrorFieldID: "s", file.ErrorFieldDisk: memory.Id})
	})

	t.Run("should fail when the file cannot be opened", func(t *testing.T) {
		app := newApp(t, nil, false, file.NewBundle())
		path := filepath.Join(t.TempDir(), "missing", "log.txt")
		_, err := creator(t, app).Create(streamConfig(container.Bag{file.ConfigFieldDisk: local.Id, file.ConfigFieldPath: path}))
		testutil.Error(t, err, file.ErrOpen, err.Error(),
			map[string]any{file.ErrorFieldID: "s", file.ErrorFieldDisk: local.Id, file.ErrorFieldPath: path})
		if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("error = %v, want it to wrap os.ErrNotExist", err)
		}
	})

	t.Run("should be created by the stream factory", func(t *testing.T) {
		app := newApp(t, map[string]any{log.StreamConfigPath + ".s": streamConfig(nil)}, false, file.NewBundle())
		testutil.NoError(t, app.Invoke(func(f flam.ILogStreamFactory) {
			s, err := f.Get("s")
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", s.Id(), "s")
		}))
	})
}

// ---------------------------------------------------------------------------
// stream
// ---------------------------------------------------------------------------

func Test_Stream(t *testing.T) {
	t.Run("should append the entries of its level or more severe", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldLevel: "notice"})
		for _, level := range []flam.LogLevel{
			flam.LogNone, flam.LogFatal, flam.LogError, flam.LogWarning, flam.LogNotice, flam.LogInfo, flam.LogDebug,
		} {
			testutil.NoError(t, s.Broadcast(ts, level, "m"))
		}
		testutil.Equal(t, "content", content(t, app, "log.txt"),
			line(flam.LogFatal, "m")+line(flam.LogError, "m")+line(flam.LogWarning, "m")+line(flam.LogNotice, "m"))
	})

	t.Run("should keep no entry at level none", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldLevel: "none"})
		testutil.NoError(t, s.Broadcast(ts, flam.LogFatal, "m"))
		testutil.Equal(t, "content", content(t, app, "log.txt"), "")
	})

	t.Run("should append to the existing file", func(t *testing.T) {
		app := newApp(t, nil, false, file.NewBundle())
		testutil.NoError(t, afero.WriteFile(memDisk(t, app), "log.txt", []byte("before\n"), 0o644))
		s, err := creator(t, app).Create(streamConfig(nil))
		testutil.NoError(t, err)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.Equal(t, "content", content(t, app, "log.txt"), "before\n"+line(flam.LogInfo, "m"))
	})

	t.Run("should keep the signaled entries of its channels, with their channel", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldChannels: []any{"a"}})
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "a", "m1"))
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "b", "m2"))
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "", "m3"))
		testutil.Equal(t, "content", content(t, app, "log.txt"), line(flam.LogInfo, "m1", "channel=a"))
	})

	t.Run("should keep the entries of any channel when listening to all", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldChannels: []string{log.AllChannels}})
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "a", "m1"))
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "", "m2"))
		testutil.Equal(t, "content", content(t, app, "log.txt"),
			line(flam.LogInfo, "m1", "channel=a")+line(flam.LogInfo, "m2", `channel=""`))
	})

	t.Run("should merge the contexts, the later winning, over the channel", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldChannels: []string{"ch"}})
		var nilBag container.Bag
		first, second := container.Bag{"a": 1, "b": 1, "channel": "x"}, otherBag{container.Bag{"b": 2}}
		testutil.NoError(t, s.Signal(ts, flam.LogInfo, "ch", "m", first, nil, nilBag, second))
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m", first))
		testutil.Equal(t, "content", content(t, app, "log.txt"),
			line(flam.LogInfo, "m", "a=1", "b=2", "channel=ch")+line(flam.LogInfo, "m", "a=1", "b=1", "channel=x"))
		testutil.Equal(t, "first", first, container.Bag{"a": 1, "b": 1, "channel": "x"})
	})

	t.Run("should set its level", func(t *testing.T) {
		s, app := newStream(t, nil)
		testutil.NoError(t, s.SetLevel(flam.LogError))
		testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogError)
		testutil.NoError(t, s.Broadcast(ts, flam.LogWarning, "m1"))
		testutil.NoError(t, s.Broadcast(ts, flam.LogError, "m2"))
		testutil.Equal(t, "content", content(t, app, "log.txt"), line(flam.LogError, "m2"))
		testutil.NoError(t, s.SetLevel(flam.LogNone))
		testutil.NoError(t, s.SetLevel(flam.LogDebug))
	})

	for _, level := range []flam.LogLevel{-1, 7} {
		t.Run(fmt.Sprintf("should refuse the level %d", level), func(t *testing.T) {
			s, _ := newStream(t, nil)
			testutil.Error(t, s.SetLevel(level), log.ErrInvalidLevel,
				"unable to set log stream level: invalid log level",
				map[string]any{log.ErrorFieldID: "s", log.ErrorFieldLevel: level})
			testutil.Equal(t, "GetLevel()", s.GetLevel(), flam.LogInfo)
		})
	}

	t.Run("should manage its channels", func(t *testing.T) {
		s, _ := newStream(t, nil)
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string(nil))
		for _, c := range []string{"b", "a", "c", "a"} {
			testutil.NoError(t, s.AddChannel(c))
		}
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string{"a", "b", "c"})
		testutil.Equal(t, "HasChannel(a)", s.HasChannel("a"), true)
		testutil.Equal(t, "HasChannel(x)", s.HasChannel("x"), false)

		testutil.NoError(t, s.RemoveChannel("b"))
		testutil.NoError(t, s.RemoveChannel("x"))
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string{"a", "c"})

		list := s.ListChannels()
		list[0] = "z"
		testutil.Equal(t, "copy", s.ListChannels(), []string{"a", "c"})

		testutil.NoError(t, s.RemoveAllChannels())
		testutil.Equal(t, "ListChannels()", s.ListChannels(), []string(nil))
	})

	t.Run("should refuse an empty channel", func(t *testing.T) {
		s, _ := newStream(t, nil)
		testutil.Error(t, s.AddChannel(""), log.ErrInvalidChannel,
			"unable to add log stream channel: invalid log channel",
			map[string]any{log.ErrorFieldID: "s", log.ErrorFieldChannel: ""})
	})

	t.Run("should close its file once, and refuse the writes afterwards", func(t *testing.T) {
		s, app := newStream(t, nil)
		testutil.NoError(t, s.Broadcast(ts, flam.LogInfo, "m"))
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		for _, err := range []error{s.Broadcast(ts, flam.LogInfo, "m"), s.Signal(ts, flam.LogInfo, "a", "m")} {
			testutil.Error(t, err, log.ErrStreamClosed, "unable to write log entry: log stream closed",
				map[string]any{log.ErrorFieldID: "s"})
		}
		testutil.Equal(t, "content", content(t, app, "log.txt"), line(flam.LogInfo, "m"))
	})

	t.Run("should return the errors of the file", func(t *testing.T) {
		app := newApp(t, nil, false, file.NewBundle())
		testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) error {
			return f.Store("failing", &failingDisk{Fs: afero.NewMemMapFs()})
		}))
		s, err := creator(t, app).Create(streamConfig(container.Bag{file.ConfigFieldDisk: "failing"}))
		testutil.NoError(t, err)
		testutil.Error(t, s.Broadcast(ts, flam.LogInfo, "m"), errBoom, "unable to write log entry: boom",
			map[string]any{log.ErrorFieldID: "s"})
		testutil.Equal(t, "Close()", s.Close(), errBoom)
	})

	t.Run("should never interleave the lines of concurrent writes", func(t *testing.T) {
		s, app := newStream(t, container.Bag{file.ConfigFieldChannels: []string{log.AllChannels}})
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				for range 50 {
					if err := s.Signal(ts, flam.LogInfo, "c", fmt.Sprint("m", i)); err != nil {
						t.Error(err)
					}
					_ = s.ListChannels()
				}
			})
		}
		wg.Wait()
		lines := strings.Split(strings.TrimSuffix(content(t, app, "log.txt"), "\n"), "\n")
		testutil.Equal(t, "lines", len(lines), 400)
		for _, l := range lines {
			if !strings.HasPrefix(l, "2026-11-02T09:43:00.000+0000 [INFO] m") {
				t.Fatalf("line = %q", l)
			}
		}
	})
}

// otherBag is a flam.IBag that is not a container.Bag.
type otherBag struct {
	flam.IBag
}
