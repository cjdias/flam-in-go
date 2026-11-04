package file

import (
	encjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/afero"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/file"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeConfig is a flam.IConfig reading Has, Get and Bag from a bag; its
// other methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

func (c fakeConfig) Bag(path string, def ...flam.IBag) flam.IBag { return c.bag.Bag(path, def...) }

// allBundles returns the bundles of a full application: the disk feature
// with the memory and local drivers, the configuration feature with the
// json and yaml parsers, and the file source driver.
func allBundles() []flam.IBundle {
	return []flam.IBundle{
		disk.NewBundle(), memory.NewBundle(), local.NewBundle(),
		config.NewBundle(), json.NewBundle(), yaml.NewBundle(),
		file.NewBundle(),
	}
}

// newApp returns a testutil.App providing a fake flam.IConfig holding the
// defaults of the configurable bundles and the values, with the bundles
// registered.
func newApp(t *testing.T, values container.Bag, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values.All() {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	// the config bundle provides the configuration: give it the defaults
	if app.HasBundle(config.NewBundle().Id()) {
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	} else {
		testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	}
	return app
}

// creator returns the single source creator of the app.
func creator(t *testing.T, app *testutil.App) flam.IConfigSourceCreator {
	t.Helper()
	var creators []flam.IConfigSourceCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.IConfigSourceCreator `group:"flam.config.sources.creator"`
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
	testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) {
		var err error
		d, err = f.Get(memory.Id)
		testutil.NoError(t, err)
	}))
	return d
}

// write writes the content to the path of the disk.
func write(t *testing.T, d flam.IDisk, path, content string) {
	t.Helper()
	testutil.NoError(t, afero.WriteFile(d, path, []byte(content), 0o600))
}

// create creates a source of id "file" from the fields with the creator of
// the app.
func create(t *testing.T, app *testutil.App, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "file", factory.ConfigFieldDriver: file.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creator(t, app).Create(cfg)
}

// setup returns an app of every bundle, its memory disk holding a
// config.json file, and the fields of a source reading it.
func setup(t *testing.T) (*testutil.App, flam.IDisk, container.Bag) {
	t.Helper()
	app := newApp(t, nil, allBundles()...)
	d := memDisk(t, app)
	write(t, d, "/config.json", `{"server": {"port": 8080}}`)
	return app, d, container.Bag{file.ConfigFieldDisk: memory.Id, file.ConfigFieldPath: "/config.json"}
}

// mustCreate creates the source of the fields.
func mustCreate(t *testing.T, app *testutil.App, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	s, err := create(t, app, fields)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// withFields returns a copy of the bag with the fields set.
func withFields(bag container.Bag, fields container.Bag) container.Bag {
	result := bag.Clone().(container.Bag)
	for key, value := range fields.All() {
		result[key] = value
	}
	return result
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{file.Driver, "flam.config.sources.driver.file"},
			{file.ConfigFieldPriority, "priority"},
			{file.ConfigFieldReload, "reload"},
			{file.DefaultReloadConfigPath, "flam.config.defaults.sources.file.reload"},
			{file.ConfigFieldDisk, "disk"},
			{file.ConfigFieldPath, "path"},
			{file.ConfigFieldParser, "parser"},
			{file.DefaultPriorityConfigPath, "flam.config.defaults.sources.file.priority"},
			{file.DefaultDiskConfigPath, "flam.config.defaults.sources.file.disk"},
			{file.DefaultParserConfigPath, "flam.config.defaults.sources.file.parser"},
			{file.ErrorFieldID, "id"},
			{file.ErrorFieldField, "field"},
			{file.ErrorFieldDisk, "disk"},
			{file.ErrorFieldPath, "path"},
			{file.ErrorFieldParser, "parser"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			file.ErrInvalidField:   "invalid file config source field",
			file.ErrDisk:           "unable to get file config source disk",
			file.ErrParser:         "unable to get file config source parser",
			file.ErrMissingFactory: "factory not provided by the container",
			file.ErrRead:           "unable to read config file",
			file.ErrParse:          "unable to parse config file",
		} {
			testutil.Equal(t, "message", err.Error(), text)
			testutil.Equal(t, "unwrap", errors.Unwrap(err), nil)
		}
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := file.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.file")
		if b == file.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, file.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "defaults", bag.Get("flam.config.defaults.sources.file"), container.Bag{
			"priority": 0, "reload": true, "disk": "local", "parser": "",
		})
		testutil.Equal(t, "sources", bag.Has(config.SourceConfigPath), false)
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := file.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, file.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
	})

	t.Run("should not need the disk and configuration features to provide the creator", func(t *testing.T) {
		app := newApp(t, nil, file.NewBundle())
		creator(t, app)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := file.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creator(t, newApp(t, nil, file.NewBundle()))
	for driver, want := range map[string]bool{
		file.Driver:                      true,
		"":                               false,
		"file":                           false,
		"flam.config.sources.driver.env": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
	testutil.Equal(t, "no driver", c.Accept(container.Bag{}), false)
}

func Test_Creator_Create(t *testing.T) {
	t.Run("should read and parse the file", func(t *testing.T) {
		app, _, fields := setup(t)
		s := mustCreate(t, app, withFields(fields, container.Bag{file.ConfigFieldPriority: 5}))
		testutil.Equal(t, "Id()", s.Id(), "file")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
	})

	parsers := []struct {
		path    string
		content string
	}{
		{"/config.json", `{"a": 1}`},
		{"/config.JSON", `{"a": 1}`},
		{"/dir.d/config.yaml", "a: 1"},
		{"/config.yml", "a: 1"},
		{"/config.YML", "a: 1"},
	}
	for _, p := range parsers {
		t.Run("should name the parser after the extension of "+p.path, func(t *testing.T) {
			app, d, fields := setup(t)
			write(t, d, p.path, p.content)
			s := mustCreate(t, app, withFields(fields, container.Bag{file.ConfigFieldPath: p.path}))
			testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		})
	}

	t.Run("should use the configured parser whatever the extension", func(t *testing.T) {
		app, d, fields := setup(t)
		write(t, d, "/config", "a: 1")
		s := mustCreate(t, app, withFields(fields, container.Bag{
			file.ConfigFieldPath: "/config", file.ConfigFieldParser: yaml.Id,
		}))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		app := newApp(t, container.Bag{
			file.DefaultPriorityConfigPath: 3,
			file.DefaultDiskConfigPath:     memory.Id,
			file.DefaultParserConfigPath:   yaml.Id,
		}, allBundles()...)
		write(t, memDisk(t, app), "/config.txt", "a: 1")
		s := mustCreate(t, app, container.Bag{file.ConfigFieldPath: "/config.txt"})
		testutil.Equal(t, "Priority()", s.Priority(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should read from the local disk by default", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		testutil.NoError(t, os.WriteFile(path, []byte(`{"a": "local"}`), 0o600))
		app := newApp(t, nil, allBundles()...)
		s := mustCreate(t, app, container.Bag{file.ConfigFieldPath: path})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "local"}))
	})

	t.Run("should use the built-in defaults when the configuration has none", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		testutil.NoError(t, os.WriteFile(path, []byte(`{}`), 0o600))
		app := newApp(t, nil, allBundles()...)
		bag := container.Bag{}
		testutil.NoError(t, app.Decorate(func(flam.IConfig) flam.IConfig {
			for _, b := range []flam.IBundle{local.NewBundle(), json.NewBundle()} {
				testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
			}
			return fakeConfig{bag: bag}
		}))
		s := mustCreate(t, app, container.Bag{file.ConfigFieldPath: path})
		testutil.Equal(t, "Priority()", s.Priority(), 0)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should be created through the source factory", func(t *testing.T) {
		app := newApp(t, container.Bag{
			config.SourceConfigPath + ".settings": container.Bag{
				factory.ConfigFieldDriver: file.Driver,
				file.ConfigFieldDisk:      memory.Id,
				file.ConfigFieldPath:      "/settings.yaml",
			},
		}, allBundles()...)
		write(t, memDisk(t, app), "/settings.yaml", "server:\n  port: 8080")
		testutil.NoError(t, app.Invoke(func(f flam.IConfigSourceFactory) {
			s, err := f.Get("settings")
			testutil.NoError(t, err)
			testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
		}))
	})
}

func Test_Creator_InvalidFields(t *testing.T) {
	scenarios := []struct {
		name   string
		fields container.Bag
		field  string
	}{
		{"a string priority", container.Bag{file.ConfigFieldPriority: "1"}, "priority"},
		{"a float priority", container.Bag{file.ConfigFieldPriority: 1.0}, "priority"},
		{"a nil priority", container.Bag{file.ConfigFieldPriority: nil}, "priority"},
		{"a missing path", container.Bag{file.ConfigFieldPath: nil}, "path"},
		{"an empty path", container.Bag{file.ConfigFieldPath: ""}, "path"},
		{"a path that is not a string", container.Bag{file.ConfigFieldPath: 1}, "path"},
		{"a disk that is not a string", container.Bag{file.ConfigFieldDisk: 1}, "disk"},
		{"a parser that is not a string", container.Bag{file.ConfigFieldParser: []any{"json"}}, "parser"},
		{"no parser for a file with no extension", container.Bag{file.ConfigFieldPath: "/config"}, "parser"},
		{"no parser for a file ending with a dot", container.Bag{file.ConfigFieldPath: "/config."}, "parser"},
	}
	for _, s := range scenarios {
		t.Run("should refuse "+s.name, func(t *testing.T) {
			app, _, fields := setup(t)
			cfg := withFields(fields, s.fields)
			if s.fields.Has(file.ConfigFieldPath) && s.fields.Get(file.ConfigFieldPath) == nil {
				delete(cfg, file.ConfigFieldPath)
			}
			source, err := create(t, app, cfg)
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, file.ErrInvalidField,
				"unable to create file config source: invalid file config source field",
				map[string]any{"id": "file", "field": s.field})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		app := newApp(t, container.Bag{file.DefaultDiskConfigPath: 1}, allBundles()...)
		_, err := create(t, app, container.Bag{file.ConfigFieldPath: "/config.json"})
		testutil.Error(t, err, file.ErrInvalidField,
			"unable to create file config source: invalid file config source field",
			map[string]any{"id": "file", "field": "disk"})
	})
}

func Test_Creator_Dependencies(t *testing.T) {
	t.Run("should fail for an unknown disk", func(t *testing.T) {
		app, _, fields := setup(t)
		_, err := create(t, app, withFields(fields, container.Bag{file.ConfigFieldDisk: "other"}))
		testutil.Error(t, err, file.ErrDisk,
			"unable to create file config source: unable to get file config source disk: "+
				"unable to generate resource: unknown factory resource id",
			map[string]any{"id": "file", "disk": "other"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatal("the error does not wrap the factory error")
		}
	})

	t.Run("should fail for an unknown parser", func(t *testing.T) {
		app, d, fields := setup(t)
		write(t, d, "/config.toml", "a = 1")
		_, err := create(t, app, withFields(fields, container.Bag{file.ConfigFieldPath: "/config.toml"}))
		testutil.Error(t, err, file.ErrParser,
			"unable to create file config source: unable to get file config source parser: "+
				"unable to generate resource: unknown factory resource id",
			map[string]any{"id": "file", "parser": "toml"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatal("the error does not wrap the factory error")
		}
	})

	t.Run("should fail without a disk factory", func(t *testing.T) {
		app := newApp(t, nil, config.NewBundle(), json.NewBundle(), file.NewBundle())
		_, err := create(t, app, container.Bag{file.ConfigFieldPath: "/config.json"})
		testutil.Error(t, err, file.ErrDisk,
			"unable to create file config source: unable to get file config source disk: "+
				"factory not provided by the container",
			map[string]any{"id": "file", "disk": "local"})
		if !errors.Is(err, file.ErrMissingFactory) {
			t.Fatal("the error does not wrap ErrMissingFactory")
		}
	})

	t.Run("should fail without a parser factory", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle(), memory.NewBundle(), file.NewBundle())
		_, err := create(t, app, container.Bag{file.ConfigFieldDisk: memory.Id, file.ConfigFieldPath: "/config.json"})
		testutil.Error(t, err, file.ErrParser,
			"unable to create file config source: unable to get file config source parser: "+
				"factory not provided by the container",
			map[string]any{"id": "file", "parser": "json"})
		if !errors.Is(err, file.ErrMissingFactory) {
			t.Fatal("the error does not wrap ErrMissingFactory")
		}
	})
}

func Test_Creator_FileErrors(t *testing.T) {
	t.Run("should fail for a missing file", func(t *testing.T) {
		app, _, fields := setup(t)
		source, err := create(t, app, withFields(fields, container.Bag{file.ConfigFieldPath: "/missing.json"}))
		if source != nil {
			t.Fatal("Create() returned a source with an error")
		}
		testutil.Error(t, err, file.ErrRead,
			"unable to create file config source: unable to read config file: open /missing.json: file does not exist",
			map[string]any{"id": "file", "disk": "memory", "path": "/missing.json"})
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("the error does not wrap the disk error")
		}
	})

	t.Run("should fail for content the parser refuses", func(t *testing.T) {
		app, d, fields := setup(t)
		write(t, d, "/config.json", `[1]`)
		_, err := create(t, app, fields)
		testutil.Error(t, err, file.ErrParse,
			"unable to create file config source: unable to parse config file: "+
				"unable to parse json config: json config document is not an object",
			map[string]any{"id": "file", "path": "/config.json", "parser": "json"})
		if !errors.Is(err, json.ErrNotObject) {
			t.Fatal("the error does not wrap the parser error")
		}
	})

	t.Run("should fail for a directory", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		dir := filepath.Join(t.TempDir(), "config.json")
		testutil.NoError(t, os.Mkdir(dir, 0o700))
		_, err := create(t, app, container.Bag{file.ConfigFieldPath: dir})
		if !errors.Is(err, file.ErrParse) || !errors.Is(err, json.ErrRead) {
			t.Fatalf("Create() = %v, want ErrParse wrapping json.ErrRead", err)
		}
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

func Test_Source(t *testing.T) {
	t.Run("should close nothing, more than once", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		write(t, d, "/other.json", `{}`) // the disk is still usable
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		app, _, fields := setup(t)
		s := mustCreate(t, app, fields)
		testutil.NoError(t, s.Bag().Set("server.port", 1))
		testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
	})
}

func Test_Source_Reload(t *testing.T) {
	t.Run("should report no change when the file did not change", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		write(t, d, "/config.json", "{\n  \"server\": {\"port\": 8080}\n}")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should read the file again", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		write(t, d, "/config.json", `{"server": {"port": 9090}}`)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 9090)
	})

	t.Run("should keep its values when the file is missing", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		testutil.NoError(t, d.Remove("/config.json"))
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, file.ErrRead,
			"unable to reload file config source: unable to read config file: open /config.json: file does not exist",
			map[string]any{"id": "file", "disk": "memory", "path": "/config.json"})
		testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
	})

	t.Run("should keep its values when the file does not parse", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		write(t, d, "/config.json", `{`)
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, file.ErrParse) || !errors.Is(err, json.ErrInvalidDocument) {
			t.Fatalf("Reload() = %v, want ErrParse wrapping json.ErrInvalidDocument", err)
		}
		if !strings.HasPrefix(err.Error(), "unable to reload file config source: unable to parse config file: ") {
			t.Fatalf("Reload() error text = %q", err.Error())
		}
		testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		app, d, fields := setup(t)
		s := mustCreate(t, app, fields)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				// replaced atomically, as a reloaded file must be
				port := []int{8080, 9090}[i%2]
				tmp := fmt.Sprintf("/config.json.%d", i)
				write(t, d, tmp, fmt.Sprintf(`{"server": {"port": %d}}`, port))
				if err := d.Rename(tmp, "/config.json"); err != nil {
					t.Errorf("Rename() = %v", err)
				}
			})
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
			wg.Go(func() {
				if port := s.Bag().Int("server.port"); port != 8080 && port != 9090 {
					t.Errorf("server.port = %d, want 8080 or 9090", port)
				}
			})
		}
		wg.Wait()
	})
}

// ---------------------------------------------------------------------------
// reload: file stamps
// ---------------------------------------------------------------------------

// countingParser decodes JSON objects into bags, counting its calls.
type countingParser struct {
	calls *atomic.Int32
}

func (p *countingParser) Id() string   { return "counting" }
func (p *countingParser) Close() error { return nil }

func (p *countingParser) Parse(r io.Reader) (flam.IBag, error) {
	p.calls.Add(1)
	var values map[string]any
	if err := encjson.NewDecoder(r).Decode(&values); err != nil {
		return nil, err
	}
	return container.Bag(values), nil
}

// countingCreator creates the countingParser of the "counting" driver.
type countingCreator struct {
	calls *atomic.Int32
}

func (c *countingCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "counting"
}

func (c *countingCreator) Create(flam.IBag) (flam.IConfigParser, error) {
	return &countingParser{calls: c.calls}, nil
}

// setupCounting returns an app whose memory disk holds /config.json, the
// fields of a source parsing it with a counting parser, and the counter.
func setupCounting(t *testing.T) (*testutil.App, flam.IDisk, container.Bag, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	app := newApp(t, container.Bag{
		config.ParserConfigPath + ".counting": container.Bag{factory.ConfigFieldDriver: "counting"},
	}, allBundles()...)
	testutil.NoError(t, app.Provide(func() flam.IConfigParserCreator { return &countingCreator{calls: calls} },
		dig.Group(config.ParserCreatorGroup)))
	d := memDisk(t, app)
	write(t, d, "/config.json", `{"a": 1}`)
	return app, d, container.Bag{
		file.ConfigFieldDisk: memory.Id, file.ConfigFieldPath: "/config.json", file.ConfigFieldParser: "counting",
	}, calls
}

// touch sets the modification time of the file.
func touch(t *testing.T, d flam.IDisk, path string, modTime time.Time) {
	t.Helper()
	testutil.NoError(t, d.Chtimes(path, modTime, modTime))
}

// modTime returns the modification time of the file.
func modTime(t *testing.T, d flam.IDisk, path string) time.Time {
	t.Helper()
	info, err := d.Stat(path)
	testutil.NoError(t, err)
	return info.ModTime()
}

// statFailDisk is a disk whose opened files cannot be stat.
type statFailDisk struct {
	afero.Fs
}

func (d *statFailDisk) Id() string { return "statfail" }

func (d *statFailDisk) Open(name string) (afero.File, error) {
	f, err := d.Fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &statFailFile{File: f}, nil
}

// statFailFile is a file whose Stat fails.
type statFailFile struct {
	afero.File
}

func (f *statFailFile) Stat() (os.FileInfo, error) { return nil, errors.New("stat failed") }

func Test_Source_Reload_Stamps(t *testing.T) {
	t.Run("should not parse an unchanged file again", func(t *testing.T) {
		app, _, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "calls after Create", calls.Load(), int32(1))
		for range 5 {
			changed, err := s.Reload()
			testutil.NoError(t, err)
			testutil.Equal(t, "changed", changed, false)
		}
		testutil.Equal(t, "calls after the reloads", calls.Load(), int32(1))
	})

	t.Run("should parse a file with a new modification time, even with the same content", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		touch(t, d, "/config.json", modTime(t, d, "/config.json").Add(time.Second))
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "calls", calls.Load(), int32(2))

		changed, err = s.Reload() // the new stamp is kept
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should parse a file with a new size, even with the same modification time", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		before := modTime(t, d, "/config.json")
		write(t, d, "/config.json", `{"a": 22}`)
		touch(t, d, "/config.json", before)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "a", s.Bag().Get("a"), 22.0)
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should miss a change keeping the modification time and size", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		before := modTime(t, d, "/config.json")
		write(t, d, "/config.json", `{"a": 2}`)
		touch(t, d, "/config.json", before)
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "a", s.Bag().Get("a"), 1.0)
		testutil.Equal(t, "calls", calls.Load(), int32(1))
	})

	t.Run("should parse a file replaced by a rename", func(t *testing.T) {
		app, d, fields, _ := setupCounting(t)
		s := mustCreate(t, app, fields)
		write(t, d, "/config.json.new", `{"a": 3}`)
		touch(t, d, "/config.json.new", modTime(t, d, "/config.json").Add(time.Second))
		testutil.NoError(t, d.Rename("/config.json.new", "/config.json"))
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "a", s.Bag().Get("a"), 3.0)
	})

	t.Run("should not record the stamp of a file it failed to parse", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		write(t, d, "/config.json", `{`)
		touch(t, d, "/config.json", modTime(t, d, "/config.json").Add(time.Second))
		for i := range 2 {
			if _, err := s.Reload(); !errors.Is(err, file.ErrParse) {
				t.Fatalf("Reload() %d = %v, want ErrParse", i, err)
			}
		}
		testutil.Equal(t, "calls", calls.Load(), int32(3))
	})

	t.Run("should parse a file restored after a failure", func(t *testing.T) {
		app, d, fields, _ := setupCounting(t)
		s := mustCreate(t, app, fields)
		before := modTime(t, d, "/config.json")
		testutil.NoError(t, d.Remove("/config.json"))
		if _, err := s.Reload(); !errors.Is(err, file.ErrRead) {
			t.Fatalf("Reload() = %v, want ErrRead", err)
		}
		write(t, d, "/config.json", `{"a": 1}`)
		touch(t, d, "/config.json", before)
		changed, err := s.Reload() // same stamp and content: nothing to do
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "a", s.Bag().Get("a"), 1.0)
	})

	t.Run("should fail when the opened file cannot be stat", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		broken := &statFailDisk{Fs: afero.NewMemMapFs()}
		write(t, broken, "/config.json", `{}`)
		var disks flam.IDiskFactory
		testutil.NoError(t, app.Invoke(func(f flam.IDiskFactory) { disks = f }))
		testutil.NoError(t, disks.Store("statfail", broken))
		_, err := create(t, app, container.Bag{file.ConfigFieldDisk: "statfail", file.ConfigFieldPath: "/config.json"})
		testutil.Error(t, err, file.ErrRead,
			"unable to create file config source: unable to read config file: stat failed",
			map[string]any{"id": "file", "disk": "statfail", "path": "/config.json"})
	})

	t.Run("should serialize the reloads", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t)
		s := mustCreate(t, app, fields)
		touch(t, d, "/config.json", modTime(t, d, "/config.json").Add(time.Second))
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
		}
		wg.Wait()
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})
}

// fileSource creates a source of a file holding a port, of priority 4,
// with the extra fields, in an app with the reload default when not nil.
func fileSource(t *testing.T, reloadDefault any, extra container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	values := container.Bag{}
	if reloadDefault != nil {
		values[file.DefaultReloadConfigPath] = reloadDefault
	}
	app := newApp(t, values, allBundles()...)
	write(t, memDisk(t, app), "/config.json", `{"server": {"port": 8080}}`)
	fields := container.Bag{
		file.ConfigFieldDisk: memory.Id, file.ConfigFieldPath: "/config.json", file.ConfigFieldPriority: 4,
	}
	return create(t, app, withFields(fields, extra))
}

// ---------------------------------------------------------------------------
// reload activation
// ---------------------------------------------------------------------------

// isReloadable reports whether the source implements
// flam.IConfigReloadableSource.
func isReloadable(s flam.IConfigSource) bool {
	_, ok := s.(flam.IConfigReloadableSource)
	return ok
}

func Test_ReloadActivation(t *testing.T) {
	t.Run("should be reloadable by default", func(t *testing.T) {
		s, err := fileSource(t, nil, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should be reloadable when activated", func(t *testing.T) {
		s, err := fileSource(t, nil, container.Bag{file.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should not be reloadable when deactivated, still being a source", func(t *testing.T) {
		s, err := fileSource(t, nil, container.Bag{file.ConfigFieldReload: false})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)
		testutil.Equal(t, "Id()", s.Id(), "file")
		testutil.Equal(t, "Priority()", s.Priority(), 4)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
		testutil.NoError(t, s.Bag().Set("x", 1)) // a copy
		testutil.Equal(t, "Bag() after a change of a copy", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should take the default stored in the configuration", func(t *testing.T) {
		s, err := fileSource(t, false, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)

		s, err = fileSource(t, false, container.Bag{file.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable with the field", isReloadable(s), true)
	})

	for _, value := range []any{"false", 0, nil} {
		t.Run(fmt.Sprintf("should refuse the reload %#v", value), func(t *testing.T) {
			s, err := fileSource(t, nil, container.Bag{file.ConfigFieldReload: value})
			if s != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, file.ErrInvalidField,
				"unable to create file config source: invalid file config source field",
				map[string]any{"id": "file", "field": "reload"})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		_, err := fileSource(t, "no", nil)
		testutil.Error(t, err, file.ErrInvalidField,
			"unable to create file config source: invalid file config source field",
			map[string]any{"id": "file", "field": "reload"})
	})
}
