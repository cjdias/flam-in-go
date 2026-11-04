package dir

import (
	encjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
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
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/dir"
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
// json and yaml parsers, and the dir source driver.
func allBundles() []flam.IBundle {
	return []flam.IBundle{
		disk.NewBundle(), memory.NewBundle(), local.NewBundle(),
		config.NewBundle(), json.NewBundle(), yaml.NewBundle(),
		dir.NewBundle(),
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

// diskFactory returns the disk factory of the app.
func diskFactory(t *testing.T, app *testutil.App) flam.IDiskFactory {
	t.Helper()
	var f flam.IDiskFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IDiskFactory) { f = provided }))
	return f
}

// memDisk returns the memory disk of the app.
func memDisk(t *testing.T, app *testutil.App) flam.IDisk {
	t.Helper()
	d, err := diskFactory(t, app).Get(memory.Id)
	testutil.NoError(t, err)
	return d
}

// write writes the files, by path, to the disk.
func write(t *testing.T, d afero.Fs, files map[string]string) {
	t.Helper()
	for path, content := range files {
		testutil.NoError(t, d.MkdirAll(filepath.Dir(path), 0o700))
		testutil.NoError(t, afero.WriteFile(d, path, []byte(content), 0o600))
	}
}

// create creates a source of id "dir" from the fields with the creator of
// the app.
func create(t *testing.T, app *testutil.App, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "dir", factory.ConfigFieldDriver: dir.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creator(t, app).Create(cfg)
}

// mustCreate creates the source of the fields.
func mustCreate(t *testing.T, app *testutil.App, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	s, err := create(t, app, fields)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// setup returns an app of every bundle, its memory disk holding the files,
// and the fields of a source reading the /conf directory of that disk.
func setup(t *testing.T, files map[string]string) (*testutil.App, flam.IDisk, container.Bag) {
	t.Helper()
	app := newApp(t, nil, allBundles()...)
	d := memDisk(t, app)
	testutil.NoError(t, d.MkdirAll("/conf", 0o700))
	write(t, d, files)
	return app, d, container.Bag{dir.ConfigFieldDisk: memory.Id, dir.ConfigFieldPath: "/conf"}
}

// withFields returns a copy of the bag with the fields set.
func withFields(bag container.Bag, fields container.Bag) container.Bag {
	result := bag.Clone().(container.Bag)
	for key, value := range fields.All() {
		result[key] = value
	}
	return result
}

// brokenDisk is a disk whose directories cannot be listed.
type brokenDisk struct {
	afero.Fs
}

func (d *brokenDisk) Id() string { return "broken" }

func (d *brokenDisk) Open(name string) (afero.File, error) {
	f, err := d.Fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &brokenFile{File: f}, nil
}

// brokenFile is a file whose Readdir fails.
type brokenFile struct {
	afero.File
}

func (f *brokenFile) Readdir(int) ([]os.FileInfo, error) { return nil, errors.New("readdir failed") }

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{dir.Driver, "flam.config.sources.driver.dir"},
			{dir.ConfigFieldPriority, "priority"},
			{dir.ConfigFieldReload, "reload"},
			{dir.DefaultReloadConfigPath, "flam.config.defaults.sources.dir.reload"},
			{dir.ConfigFieldDisk, "disk"},
			{dir.ConfigFieldPath, "path"},
			{dir.ConfigFieldRecursive, "recursive"},
			{dir.ConfigFieldParser, "parser"},
			{dir.DefaultPriorityConfigPath, "flam.config.defaults.sources.dir.priority"},
			{dir.DefaultDiskConfigPath, "flam.config.defaults.sources.dir.disk"},
			{dir.DefaultRecursiveConfigPath, "flam.config.defaults.sources.dir.recursive"},
			{dir.DefaultParserConfigPath, "flam.config.defaults.sources.dir.parser"},
			{dir.ErrorFieldID, "id"},
			{dir.ErrorFieldField, "field"},
			{dir.ErrorFieldDisk, "disk"},
			{dir.ErrorFieldPath, "path"},
			{dir.ErrorFieldParser, "parser"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			dir.ErrInvalidField:   "invalid dir config source field",
			dir.ErrDisk:           "unable to get dir config source disk",
			dir.ErrParser:         "unable to get dir config source parser",
			dir.ErrMissingFactory: "factory not provided by the container",
			dir.ErrNotDirectory:   "not a directory",
			dir.ErrRead:           "unable to read config directory",
			dir.ErrParse:          "unable to parse config file",
			dir.ErrMerge:          "unable to merge config file",
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
		b := dir.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.dir")
		if b == dir.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, dir.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "defaults", bag.Get("flam.config.defaults.sources.dir"), container.Bag{
			"priority": 0, "reload": true, "disk": "local", "recursive": false, "parser": "",
		})
		testutil.Equal(t, "sources", bag.Has(config.SourceConfigPath), false)
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := dir.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, dir.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
	})

	t.Run("should not need the disk and configuration features to provide the creator", func(t *testing.T) {
		creator(t, newApp(t, nil, dir.NewBundle()))
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := dir.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creator(t, newApp(t, nil, dir.NewBundle()))
	for driver, want := range map[string]bool{
		dir.Driver:                        true,
		"":                                false,
		"dir":                             false,
		"flam.config.sources.driver.file": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
	testutil.Equal(t, "no driver", c.Accept(container.Bag{}), false)
}

func Test_Creator_Create(t *testing.T) {
	t.Run("should give an empty bag for an empty directory", func(t *testing.T) {
		app, _, fields := setup(t, nil)
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldPriority: 5}))
		testutil.Equal(t, "Id()", s.Id(), "dir")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		app := newApp(t, container.Bag{
			dir.DefaultPriorityConfigPath:  3,
			dir.DefaultDiskConfigPath:      memory.Id,
			dir.DefaultRecursiveConfigPath: true,
			dir.DefaultParserConfigPath:    yaml.Id,
		}, allBundles()...)
		write(t, memDisk(t, app), map[string]string{"/conf/sub/settings": "a: 1"})
		s := mustCreate(t, app, container.Bag{dir.ConfigFieldPath: "/conf"})
		testutil.Equal(t, "Priority()", s.Priority(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should read from the local disk by default", func(t *testing.T) {
		root := t.TempDir()
		write(t, afero.NewOsFs(), map[string]string{filepath.Join(root, "config.json"): `{"a": "local"}`})
		s := mustCreate(t, newApp(t, nil, allBundles()...), container.Bag{dir.ConfigFieldPath: root})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "local"}))
	})

	t.Run("should use the built-in defaults when the configuration has none", func(t *testing.T) {
		root := t.TempDir()
		write(t, afero.NewOsFs(), map[string]string{filepath.Join(root, "sub", "config.json"): `{"a": 1}`})
		app := newApp(t, nil, allBundles()...)
		testutil.NoError(t, app.Decorate(func(flam.IConfig) flam.IConfig {
			bag := container.Bag{}
			for _, b := range []flam.IBundle{local.NewBundle(), json.NewBundle()} {
				testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
			}
			return fakeConfig{bag: bag}
		}))
		s := mustCreate(t, app, container.Bag{dir.ConfigFieldPath: root})
		testutil.Equal(t, "Priority()", s.Priority(), 0)
		testutil.Equal(t, "Bag() not recursive", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should be created through the source factory", func(t *testing.T) {
		app := newApp(t, container.Bag{
			config.SourceConfigPath + ".settings": container.Bag{
				factory.ConfigFieldDriver: dir.Driver,
				dir.ConfigFieldDisk:       memory.Id,
				dir.ConfigFieldPath:       "/conf",
			},
		}, allBundles()...)
		write(t, memDisk(t, app), map[string]string{"/conf/server.yaml": "server:\n  port: 8080"})
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
		{"a string priority", container.Bag{dir.ConfigFieldPriority: "1"}, "priority"},
		{"a nil priority", container.Bag{dir.ConfigFieldPriority: nil}, "priority"},
		{"a missing path", container.Bag{dir.ConfigFieldPath: nil}, "path"},
		{"an empty path", container.Bag{dir.ConfigFieldPath: ""}, "path"},
		{"a path that is not a string", container.Bag{dir.ConfigFieldPath: 1}, "path"},
		{"a string recursive", container.Bag{dir.ConfigFieldRecursive: "true"}, "recursive"},
		{"an int recursive", container.Bag{dir.ConfigFieldRecursive: 1}, "recursive"},
		{"a nil recursive", container.Bag{dir.ConfigFieldRecursive: nil}, "recursive"},
		{"a disk that is not a string", container.Bag{dir.ConfigFieldDisk: 1}, "disk"},
		{"a parser that is not a string", container.Bag{dir.ConfigFieldParser: []any{"json"}}, "parser"},
	}
	for _, s := range scenarios {
		t.Run("should refuse "+s.name, func(t *testing.T) {
			app, _, fields := setup(t, nil)
			cfg := withFields(fields, s.fields)
			if s.name == "a missing path" {
				delete(cfg, dir.ConfigFieldPath)
			}
			source, err := create(t, app, cfg)
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, dir.ErrInvalidField,
				"unable to create dir config source: invalid dir config source field",
				map[string]any{"id": "dir", "field": s.field})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		app := newApp(t, container.Bag{dir.DefaultRecursiveConfigPath: "yes"}, allBundles()...)
		_, err := create(t, app, container.Bag{dir.ConfigFieldPath: "/conf"})
		testutil.Error(t, err, dir.ErrInvalidField,
			"unable to create dir config source: invalid dir config source field",
			map[string]any{"id": "dir", "field": "recursive"})
	})
}

func Test_Creator_Dependencies(t *testing.T) {
	t.Run("should fail for an unknown disk", func(t *testing.T) {
		app, _, fields := setup(t, nil)
		_, err := create(t, app, withFields(fields, container.Bag{dir.ConfigFieldDisk: "other"}))
		testutil.Error(t, err, dir.ErrDisk,
			"unable to create dir config source: unable to get dir config source disk: "+
				"unable to generate resource: unknown factory resource id",
			map[string]any{"id": "dir", "disk": "other"})
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatal("the error does not wrap the factory error")
		}
	})

	t.Run("should fail for an unknown configured parser", func(t *testing.T) {
		app, _, fields := setup(t, nil)
		_, err := create(t, app, withFields(fields, container.Bag{dir.ConfigFieldParser: "toml"}))
		testutil.Error(t, err, dir.ErrParser,
			"unable to create dir config source: unable to get dir config source parser: "+
				"unable to generate resource: unknown factory resource id",
			map[string]any{"id": "dir", "parser": "toml"})
	})

	t.Run("should fail without a disk factory", func(t *testing.T) {
		app := newApp(t, nil, config.NewBundle(), json.NewBundle(), dir.NewBundle())
		_, err := create(t, app, container.Bag{dir.ConfigFieldPath: "/conf"})
		testutil.Error(t, err, dir.ErrDisk,
			"unable to create dir config source: unable to get dir config source disk: "+
				"factory not provided by the container",
			map[string]any{"id": "dir", "disk": "local"})
		if !errors.Is(err, dir.ErrMissingFactory) {
			t.Fatal("the error does not wrap ErrMissingFactory")
		}
	})

	t.Run("should fail without a parser factory, even to infer the parsers", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle(), memory.NewBundle(), dir.NewBundle())
		_, err := create(t, app, container.Bag{dir.ConfigFieldDisk: memory.Id, dir.ConfigFieldPath: "/conf"})
		testutil.Error(t, err, dir.ErrParser,
			"unable to create dir config source: unable to get dir config source parser: "+
				"factory not provided by the container",
			map[string]any{"id": "dir", "parser": ""})
		if !errors.Is(err, dir.ErrMissingFactory) {
			t.Fatal("the error does not wrap ErrMissingFactory")
		}
	})
}

// ---------------------------------------------------------------------------
// files
// ---------------------------------------------------------------------------

func Test_Files(t *testing.T) {
	t.Run("should merge the files in name order, a later file overriding", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/10-local.yaml": "server:\n  port: 9090\nlocal: true",
			"/conf/00-base.json":  `{"server": {"host": "db", "port": 8080}, "list": [1, 2]}`,
			"/conf/20-list.yml":   "list: [3]",
		})
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{
			"server": container.Bag{"host": "db", "port": 9090},
			"local":  true,
			"list":   []any{3},
		}))
	})

	t.Run("should expand the dotted keys of the files, as Merge does", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.json": `{"server.port": 8080}`})
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"server": container.Bag{"port": 8080}}))
	})

	t.Run("should not read the sub directories unless recursive", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/a.yaml":     "a: 1",
			"/conf/sub/b.yaml": "b: 1",
		})
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should read the sub directories at their place in name order when recursive", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/a.yaml":        "x: a\ny: a\nz: a",
			"/conf/b/c.yaml":      "x: b/c\ny: b/c",
			"/conf/b/deep/e.yaml": "x: b/deep/e",
			"/conf/d.yaml":        "y: d",
		})
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldRecursive: true}))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"x": "b/deep/e", "y": "d", "z": "a"}))
	})

	t.Run("should infer the parser of each file, skipping the others", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/a.json":     `{"json": 1}`,
			"/conf/b.JSON":     `{"upper": 1}`,
			"/conf/c.yaml":     "yaml: 1",
			"/conf/d.yml":      "yml: 1",
			"/conf/README":     "not a config",
			"/conf/notes.txt":  "not a config",
			"/conf/x.toml":     "a = 1",
			"/conf/trailing.":  "not a config",
			"/conf/swap.yaml~": "not a config",
		})
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"json": 1, "upper": 1, "yaml": 1, "yml": 1}))
	})

	t.Run("should parse every file with the configured parser", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/settings":     "a: 1",
			"/conf/other.txt":    "b: 1",
			"/conf/valid.json":   `{"c": 1}`,
			"/conf/sub/ignored":  "d: 1",
			"/conf/.hidden.yaml": "e: 1",
		})
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldParser: yaml.Id}))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1, "b": 1, "c": 1}))
	})

	t.Run("should skip the hidden files and directories", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{
			"/conf/.hidden.yaml":     "hidden: 1",
			"/conf/..data/conf.yaml": "data: 1",
			"/conf/.git/config.yaml": "git: 1",
			"/conf/sub/.inner.yaml":  "inner: 1",
			"/conf/sub/visible.yaml": "visible: 1",
			"/conf/visible.json":     `{"top": 1}`,
		})
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldRecursive: true}))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"visible": 1, "top": 1}))
	})
}

func Test_Files_Links(t *testing.T) {
	newLocal := func(t *testing.T, files map[string]string) (*testutil.App, string) {
		t.Helper()
		root := t.TempDir()
		osFiles := map[string]string{}
		for path, content := range files {
			osFiles[filepath.Join(root, path)] = content
		}
		write(t, afero.NewOsFs(), osFiles)
		return newApp(t, nil, allBundles()...), root
	}

	t.Run("should read a link to a file", func(t *testing.T) {
		app, root := newLocal(t, map[string]string{"target/real.yaml": "a: 1"})
		conf := filepath.Join(root, "conf")
		testutil.NoError(t, os.Mkdir(conf, 0o700))
		testutil.NoError(t, os.Symlink(filepath.Join(root, "target", "real.yaml"), filepath.Join(conf, "link.yaml")))
		s := mustCreate(t, app, container.Bag{dir.ConfigFieldPath: conf})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should not follow a link to a directory, so not loop on a cycle", func(t *testing.T) {
		app, root := newLocal(t, map[string]string{"conf/a.yaml": "a: 1", "other/b.yaml": "b: 1"})
		conf := filepath.Join(root, "conf")
		testutil.NoError(t, os.Symlink(conf, filepath.Join(conf, "loop")))
		testutil.NoError(t, os.Symlink(filepath.Join(root, "other"), filepath.Join(conf, "other")))
		s := mustCreate(t, app, container.Bag{dir.ConfigFieldPath: conf, dir.ConfigFieldRecursive: true})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
	})

	t.Run("should fail for a dangling link", func(t *testing.T) {
		app, root := newLocal(t, map[string]string{"conf/a.yaml": "a: 1"})
		conf := filepath.Join(root, "conf")
		link := filepath.Join(conf, "dangling.yaml")
		testutil.NoError(t, os.Symlink(filepath.Join(root, "missing"), link))
		_, err := create(t, app, container.Bag{dir.ConfigFieldPath: conf})
		if !errors.Is(err, dir.ErrRead) || !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Create() = %v, want ErrRead wrapping fs.ErrNotExist", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "dir", "disk": "local", "path": link})
	})
}

func Test_Files_Errors(t *testing.T) {
	t.Run("should fail for a missing directory", func(t *testing.T) {
		app, _, fields := setup(t, nil)
		source, err := create(t, app, withFields(fields, container.Bag{dir.ConfigFieldPath: "/missing"}))
		if source != nil {
			t.Fatal("Create() returned a source with an error")
		}
		testutil.Error(t, err, dir.ErrRead,
			"unable to create dir config source: unable to read config directory: open /missing: file does not exist",
			map[string]any{"id": "dir", "disk": "memory", "path": "/missing"})
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("the error does not wrap the disk error")
		}
	})

	t.Run("should fail for a path that is a file", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		_, err := create(t, app, withFields(fields, container.Bag{dir.ConfigFieldPath: "/conf/a.yaml"}))
		testutil.Error(t, err, dir.ErrRead,
			"unable to create dir config source: unable to read config directory: not a directory",
			map[string]any{"id": "dir", "disk": "memory", "path": "/conf/a.yaml"})
		if !errors.Is(err, dir.ErrNotDirectory) {
			t.Fatal("the error does not wrap ErrNotDirectory")
		}
	})

	t.Run("should fail for a directory that cannot be listed", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		broken := &brokenDisk{Fs: afero.NewMemMapFs()}
		testutil.NoError(t, broken.MkdirAll("/conf", 0o700))
		testutil.NoError(t, diskFactory(t, app).Store("broken", broken))
		_, err := create(t, app, container.Bag{dir.ConfigFieldDisk: "broken", dir.ConfigFieldPath: "/conf"})
		testutil.Error(t, err, dir.ErrRead,
			"unable to create dir config source: unable to read config directory: readdir failed",
			map[string]any{"id": "dir", "disk": "broken", "path": "/conf"})
	})

	t.Run("should fail for a sub directory that cannot be opened", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads any directory")
		}
		root := t.TempDir()
		locked := filepath.Join(root, "locked")
		testutil.NoError(t, os.Mkdir(locked, 0o700))
		testutil.NoError(t, os.Chmod(locked, 0))
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
		app := newApp(t, nil, allBundles()...)

		_, err := create(t, app, container.Bag{dir.ConfigFieldPath: root, dir.ConfigFieldRecursive: true})
		if !errors.Is(err, dir.ErrRead) || !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("Create() = %v, want ErrRead wrapping fs.ErrPermission", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "dir", "disk": "local", "path": locked})

		s := mustCreate(t, app, container.Bag{dir.ConfigFieldPath: root}) // not read unless recursive
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should fail for a file that cannot be opened", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads any file")
		}
		root := t.TempDir()
		file := filepath.Join(root, "a.yaml")
		write(t, afero.NewOsFs(), map[string]string{file: "a: 1"})
		testutil.NoError(t, os.Chmod(file, 0))
		_, err := create(t, newApp(t, nil, allBundles()...), container.Bag{dir.ConfigFieldPath: root})
		if !errors.Is(err, dir.ErrRead) || !errors.Is(err, fs.ErrPermission) {
			t.Fatalf("Create() = %v, want ErrRead wrapping fs.ErrPermission", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "dir", "disk": "local", "path": file})
	})

	t.Run("should fail for a file the parser refuses", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1", "/conf/b.json": `[1]`})
		_, err := create(t, app, fields)
		testutil.Error(t, err, dir.ErrParse,
			"unable to create dir config source: unable to parse config file: "+
				"unable to parse json config: json config document is not an object",
			map[string]any{"id": "dir", "path": "/conf/b.json", "parser": "json"})
		if !errors.Is(err, json.ErrNotObject) {
			t.Fatal("the error does not wrap the parser error")
		}
	})

	t.Run("should fail for values that cannot be merged", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.json": `{"a..b": 1}`})
		_, err := create(t, app, fields)
		if !errors.Is(err, dir.ErrMerge) || !errors.Is(err, container.ErrInvalidPath) {
			t.Fatalf("Create() = %v, want ErrMerge wrapping container.ErrInvalidPath", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "dir", "path": "/conf/a.json"})
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

func Test_Source(t *testing.T) {
	t.Run("should close nothing, more than once", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		s := mustCreate(t, app, fields)
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		app, _, fields := setup(t, map[string]string{"/conf/a.yaml": "a:\n  b: 1"})
		s := mustCreate(t, app, fields)
		testutil.NoError(t, s.Bag().Set("a.b", 2))
		testutil.Equal(t, "a.b", s.Bag().Int("a.b"), 1)
	})
}

func Test_Source_Reload(t *testing.T) {
	t.Run("should report no change when the files did not change", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/a.yaml": "a:   1 # same", "/conf/README": "x"})
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should read the directory again", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1", "/conf/b.yaml": "b: 1"})
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/a.yaml": "a: 2", "/conf/c.yaml": "c: 1"})
		testutil.NoError(t, d.Remove("/conf/b.yaml"))
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 2, "c": 1}))
	})

	t.Run("should keep its values when the directory is gone", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		s := mustCreate(t, app, fields)
		testutil.NoError(t, d.RemoveAll("/conf"))
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, dir.ErrRead,
			"unable to reload dir config source: unable to read config directory: open /conf: file does not exist",
			map[string]any{"id": "dir", "disk": "memory", "path": "/conf"})
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should keep its values when a file does not parse", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/b.yaml": "b: ["})
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, dir.ErrParse) || !errors.Is(err, yaml.ErrInvalidDocument) {
			t.Fatalf("Reload() = %v, want ErrParse wrapping yaml.ErrInvalidDocument", err)
		}
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should fail, not skip the files, when the parser factory is closed", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "a: 1"})
		s := mustCreate(t, app, fields)
		testutil.NoError(t, app.Invoke(func(f flam.IConfigParserFactory) { testutil.NoError(t, f.Close()) }))
		write(t, d, map[string]string{"/conf/b.yaml": "b: 1"}) // a change, to read the files
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, dir.ErrParser) || !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Reload() = %v, want ErrParser wrapping factory.ErrClosed", err)
		}
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "dir", "parser": "yaml"})
		testutil.Equal(t, "a", s.Bag().Int("a"), 1)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		app, d, fields := setup(t, map[string]string{"/conf/a.yaml": "port: 8080"})
		s := mustCreate(t, app, fields)
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				// replaced atomically, as a reloaded file must be
				tmp := fmt.Sprintf("/tmp-%d.yaml", i)
				write(t, d, map[string]string{tmp: fmt.Sprintf("port: %d", []int{8080, 9090}[i%2])})
				if err := d.Rename(tmp, "/conf/a.yaml"); err != nil {
					t.Errorf("Rename() = %v", err)
				}
			})
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
			wg.Go(func() {
				if port := s.Bag().Int("port"); port != 8080 && port != 9090 {
					t.Errorf("port = %d, want 8080 or 9090", port)
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

func (p *countingParser) Id() string   { return "cjson" }
func (p *countingParser) Close() error { return nil }

func (p *countingParser) Parse(r io.Reader) (flam.IBag, error) {
	p.calls.Add(1)
	var values map[string]any
	if err := encjson.NewDecoder(r).Decode(&values); err != nil {
		return nil, err
	}
	return container.Bag(values), nil
}

// countingCreator creates the countingParser of the "cjson" driver.
type countingCreator struct {
	calls *atomic.Int32
}

func (c *countingCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "cjson"
}

func (c *countingCreator) Create(flam.IBag) (flam.IConfigParser, error) {
	return &countingParser{calls: c.calls}, nil
}

// setupCounting returns an app whose memory disk holds the files, with a
// counting parser of id "cjson", named by the .cjson extension, the fields
// of a source reading /conf recursively, and the counter.
func setupCounting(t *testing.T, files map[string]string) (*testutil.App, flam.IDisk, container.Bag, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	app := newApp(t, container.Bag{
		config.ParserConfigPath + ".cjson": container.Bag{factory.ConfigFieldDriver: "cjson"},
	}, allBundles()...)
	testutil.NoError(t, app.Provide(func() flam.IConfigParserCreator { return &countingCreator{calls: calls} },
		dig.Group(config.ParserCreatorGroup)))
	d := memDisk(t, app)
	testutil.NoError(t, d.MkdirAll("/conf", 0o700))
	write(t, d, files)
	return app, d, container.Bag{
		dir.ConfigFieldDisk: memory.Id, dir.ConfigFieldPath: "/conf", dir.ConfigFieldRecursive: true,
	}, calls
}

// modTime returns the modification time of the file, read at once: the
// file information of the memory disk follows the later changes.
func modTime(t *testing.T, d flam.IDisk, path string) time.Time {
	t.Helper()
	info, err := d.Stat(path)
	testutil.NoError(t, err)
	return info.ModTime()
}

// touch moves the modification time of the file one second later.
func touch(t *testing.T, d flam.IDisk, path string) {
	t.Helper()
	later := modTime(t, d, path).Add(time.Second)
	testutil.NoError(t, d.Chtimes(path, later, later))
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

// statFailFile is a file whose Stat fails, but for the directories.
type statFailFile struct {
	afero.File
}

func (f *statFailFile) Stat() (os.FileInfo, error) { return nil, errors.New("stat failed") }

func Test_Source_Reload_Stamps(t *testing.T) {
	twoFiles := map[string]string{"/conf/a.cjson": `{"a": 1}`, "/conf/sub/b.cjson": `{"b": 1}`}

	reload := func(t *testing.T, s flam.IConfigReloadableSource, wantChanged bool) {
		t.Helper()
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, wantChanged)
	}

	t.Run("should not parse an unchanged directory again", func(t *testing.T) {
		app, _, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		testutil.Equal(t, "calls after Create", calls.Load(), int32(2))
		for range 5 {
			reload(t, s, false)
		}
		testutil.Equal(t, "calls after the reloads", calls.Load(), int32(2))
	})

	t.Run("should not parse an unchanged empty directory again", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, nil)
		s := mustCreate(t, app, fields)
		reload(t, s, false)
		write(t, d, map[string]string{"/conf/a.cjson": `{"a": 1}`})
		reload(t, s, true)
		testutil.Equal(t, "calls", calls.Load(), int32(1))
	})

	t.Run("should parse every file again when one has a new modification time", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		touch(t, d, "/conf/sub/b.cjson")
		reload(t, s, false)
		testutil.Equal(t, "calls", calls.Load(), int32(4))
		reload(t, s, false) // the new stamps are kept
		testutil.Equal(t, "calls", calls.Load(), int32(4))
	})

	t.Run("should parse a file with a new size, even with the same modification time", func(t *testing.T) {
		app, d, fields, _ := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		before := modTime(t, d, "/conf/a.cjson")
		write(t, d, map[string]string{"/conf/a.cjson": `{"a": 22}`})
		testutil.NoError(t, d.Chtimes("/conf/a.cjson", before, before))
		reload(t, s, true)
		testutil.Equal(t, "a", s.Bag().Get("a"), 22.0)
	})

	t.Run("should miss a change keeping the modification time and size", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		before := modTime(t, d, "/conf/a.cjson")
		write(t, d, map[string]string{"/conf/a.cjson": `{"a": 2}`})
		testutil.NoError(t, d.Chtimes("/conf/a.cjson", before, before))
		reload(t, s, false)
		testutil.Equal(t, "a", s.Bag().Get("a"), 1.0)
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should parse again when a file is added, removed or renamed", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)

		write(t, d, map[string]string{"/conf/c.cjson": `{"c": 1}`})
		reload(t, s, true)
		testutil.Equal(t, "calls after an addition", calls.Load(), int32(5))

		testutil.NoError(t, d.Remove("/conf/c.cjson"))
		reload(t, s, true)
		testutil.Equal(t, "calls after a removal", calls.Load(), int32(7))

		testutil.NoError(t, d.Rename("/conf/a.cjson", "/conf/z.cjson"))
		reload(t, s, false) // the same values, in another order
		testutil.Equal(t, "calls after a rename", calls.Load(), int32(9))
	})

	t.Run("should ignore the changes of the files with no extension", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/README": "notes", "/conf/.hidden.cjson": "{}"})
		reload(t, s, false)
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should parse again for a change of a file with an unknown extension", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/notes.txt": "notes"})
		reload(t, s, false)
		testutil.Equal(t, "calls", calls.Load(), int32(4))
	})

	t.Run("should watch every file with a configured parser", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, map[string]string{"/conf/settings": `{"a": 1}`})
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldParser: "cjson"}))
		write(t, d, map[string]string{"/conf/other": `{"b": 1}`})
		reload(t, s, true)
		testutil.Equal(t, "calls", calls.Load(), int32(3))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1.0, "b": 1.0}))
	})

	t.Run("should not watch the sub directories unless recursive", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, withFields(fields, container.Bag{dir.ConfigFieldRecursive: false}))
		touch(t, d, "/conf/sub/b.cjson")
		reload(t, s, false)
		testutil.Equal(t, "calls", calls.Load(), int32(1))
	})

	t.Run("should not record the snapshot of files it failed to parse", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		write(t, d, map[string]string{"/conf/a.cjson": `{`})
		for i := range 2 {
			if _, err := s.Reload(); !errors.Is(err, dir.ErrParse) {
				t.Fatalf("Reload() %d = %v, want ErrParse", i, err)
			}
		}
		testutil.Equal(t, "calls", calls.Load(), int32(4))
		testutil.Equal(t, "a", s.Bag().Get("a"), 1.0)
	})

	t.Run("should fail when an opened file cannot be stat", func(t *testing.T) {
		app := newApp(t, nil, allBundles()...)
		broken := &statFailDisk{Fs: afero.NewMemMapFs()}
		write(t, broken, map[string]string{"/conf/a.json": `{}`})
		testutil.NoError(t, diskFactory(t, app).Store("statfail", broken))
		_, err := create(t, app, container.Bag{dir.ConfigFieldDisk: "statfail", dir.ConfigFieldPath: "/conf"})
		testutil.Error(t, err, dir.ErrRead,
			"unable to create dir config source: unable to read config directory: stat failed",
			map[string]any{"id": "dir", "disk": "statfail", "path": "/conf/a.json"})
	})

	t.Run("should serialize the reloads", func(t *testing.T) {
		app, d, fields, calls := setupCounting(t, twoFiles)
		s := mustCreate(t, app, fields)
		touch(t, d, "/conf/a.cjson")
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
		}
		wg.Wait()
		testutil.Equal(t, "calls", calls.Load(), int32(4))
	})
}

// dirSource creates a source of the /conf directory holding a.yaml, of
// priority 4, with the extra fields, in an app with the reload default when
// not nil.
func dirSource(t *testing.T, reloadDefault any, extra container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	values := container.Bag{}
	if reloadDefault != nil {
		values[dir.DefaultReloadConfigPath] = reloadDefault
	}
	app := newApp(t, values, allBundles()...)
	write(t, memDisk(t, app), map[string]string{"/conf/a.yaml": "a: 1"})
	fields := container.Bag{dir.ConfigFieldDisk: memory.Id, dir.ConfigFieldPath: "/conf", dir.ConfigFieldPriority: 4}
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
		s, err := dirSource(t, nil, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should be reloadable when activated", func(t *testing.T) {
		s, err := dirSource(t, nil, container.Bag{dir.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should not be reloadable when deactivated, still being a source", func(t *testing.T) {
		s, err := dirSource(t, nil, container.Bag{dir.ConfigFieldReload: false})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)
		testutil.Equal(t, "Id()", s.Id(), "dir")
		testutil.Equal(t, "Priority()", s.Priority(), 4)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.NoError(t, s.Bag().Set("x", 1)) // a copy
		testutil.Equal(t, "Bag() after a change of a copy", s.Bag(), flam.IBag(container.Bag{"a": 1}))
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should take the default stored in the configuration", func(t *testing.T) {
		s, err := dirSource(t, false, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)

		s, err = dirSource(t, false, container.Bag{dir.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable with the field", isReloadable(s), true)
	})

	for _, value := range []any{"false", 0, nil} {
		t.Run(fmt.Sprintf("should refuse the reload %#v", value), func(t *testing.T) {
			s, err := dirSource(t, nil, container.Bag{dir.ConfigFieldReload: value})
			if s != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, dir.ErrInvalidField,
				"unable to create dir config source: invalid dir config source field",
				map[string]any{"id": "dir", "field": "reload"})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		_, err := dirSource(t, "no", nil)
		testutil.Error(t, err, dir.ErrInvalidField,
			"unable to create dir config source: invalid dir config source field",
			map[string]any{"id": "dir", "field": "reload"})
	})
}
