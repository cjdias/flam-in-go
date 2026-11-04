package env

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/env"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// fakeConfig is a flam.IConfig reading Has and Get from a bag; its other
// methods are not implemented.
type fakeConfig struct {
	flam.IConfig
	bag container.Bag
}

func (c fakeConfig) Has(path string) bool { return c.bag.Has(path) }

func (c fakeConfig) Get(path string, def ...any) any { return c.bag.Get(path, def...) }

// defaults returns the bag configured by the bundle.
func defaults(t *testing.T) container.Bag {
	t.Helper()
	bag := container.Bag{}
	testutil.NoError(t, env.NewBundle().(flam.IConfigurableBundle).Config(bag))
	return bag
}

// creators returns the source creators the bundle provides in the source
// creator group, reading the defaults from a configuration of the bag.
func creators(t *testing.T, bag container.Bag) []flam.IConfigSourceCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, env.NewBundle().Register(app))

	var result []flam.IConfigSourceCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.IConfigSourceCreator `group:"flam.config.sources.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// create creates a source of id "env" from the fields, with the bundle
// defaults.
func create(t *testing.T, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	return createWith(t, defaults(t), fields)
}

// createWith creates a source of id "env" from the fields, with the
// configuration of the bag.
func createWith(t *testing.T, bag container.Bag, fields container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "env", factory.ConfigFieldDriver: env.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creators(t, bag)[0].Create(cfg)
}

// mustCreate creates a source mapping the variables to the paths.
func mustCreate(t *testing.T, fields container.Bag) flam.IConfigReloadableSource {
	t.Helper()
	s, err := create(t, fields)
	testutil.NoError(t, err)
	return s.(flam.IConfigReloadableSource)
}

// writeFile writes a dotenv file in a temporary directory and returns its
// path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	testutil.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

// unset unsets the variable for the test, restoring it afterwards.
func unset(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	testutil.NoError(t, os.Unsetenv(key))
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{env.Driver, "flam.config.sources.driver.env"},
			{env.ConfigFieldPriority, "priority"},
			{env.ConfigFieldReload, "reload"},
			{env.DefaultReloadConfigPath, "flam.config.defaults.sources.env.reload"},
			{env.ConfigFieldFiles, "files"},
			{env.ConfigFieldMappings, "mappings"},
			{env.DefaultPriorityConfigPath, "flam.config.defaults.sources.env.priority"},
			{env.DefaultFilesConfigPath, "flam.config.defaults.sources.env.files"},
			{env.DefaultMappingsConfigPath, "flam.config.defaults.sources.env.mappings"},
			{env.ErrorFieldID, "id"},
			{env.ErrorFieldField, "field"},
			{env.ErrorFieldKey, "key"},
			{env.ErrorFieldPath, "path"},
			{env.ErrorFieldOtherKey, "other_key"},
			{env.ErrorFieldOtherPath, "other_path"},
			{env.ErrorFieldFiles, "files"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			env.ErrInvalidField:    "invalid env config source field",
			env.ErrInvalidMapping:  "invalid env config source mapping",
			env.ErrMappingConflict: "conflicting env config source mappings",
			env.ErrRead:            "unable to read env config files",
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
		b := env.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config.sources.env")
		if b == env.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the defaults and no source", func(t *testing.T) {
		bag := container.Bag{"flam": container.Bag{"config": container.Bag{"other": 1}}}
		testutil.NoError(t, env.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "config", bag.Get("flam.config"), container.Bag{
			"other": 1,
			"defaults": container.Bag{"sources": container.Bag{"env": container.Bag{
				"priority": 0, "reload": true, "files": []string{}, "mappings": container.Bag{},
			}}},
		})
		testutil.Equal(t, "sources", bag.Has(config.SourceConfigPath), false)
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		bag := container.Bag{"flam": "scalar"}
		err := env.NewBundle().(flam.IConfigurableBundle).Config(bag)
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, env.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, defaults(t))), 1)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := env.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator_Accept(t *testing.T) {
	c := creators(t, defaults(t))[0]
	for driver, want := range map[string]bool{
		env.Driver:                        true,
		"":                                false,
		"env":                             false,
		"flam.config.sources.driver.file": false,
	} {
		testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
	}
	testutil.Equal(t, "no driver", c.Accept(container.Bag{}), false)
}

func Test_Creator_Fields(t *testing.T) {
	t.Run("should create a source with the configured id and priority", func(t *testing.T) {
		s, err := create(t, container.Bag{env.ConfigFieldPriority: 7})
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", s.Id(), "env")
		testutil.Equal(t, "Priority()", s.Priority(), 7)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("FLAM_ENV_DEFAULT", "x")
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(env.DefaultPriorityConfigPath, 3))
		testutil.NoError(t, bag.Set(env.DefaultFilesConfigPath, []any{writeFile(t, dir, ".env", "FLAM_ENV_FILE=y")}))
		testutil.NoError(t, bag.Set(env.DefaultMappingsConfigPath, map[string]any{
			"FLAM_ENV_DEFAULT": "a", "FLAM_ENV_FILE": "b",
		}))
		s, err := createWith(t, bag, container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "x", "b": "y"}))
	})

	t.Run("should prefer the fields to the defaults", func(t *testing.T) {
		t.Setenv("FLAM_ENV_FIELD", "x")
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(env.DefaultPriorityConfigPath, 3))
		testutil.NoError(t, bag.Set(env.DefaultFilesConfigPath, []string{"missing.env"}))
		testutil.NoError(t, bag.Set(env.DefaultMappingsConfigPath, container.Bag{"FLAM_ENV_FIELD": "default"}))
		s, err := createWith(t, bag, container.Bag{
			env.ConfigFieldPriority: -1,
			env.ConfigFieldFiles:    []string{},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_FIELD": "field"},
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), -1)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"field": "x"}))
	})

	t.Run("should use the built-in defaults when the configuration has none", func(t *testing.T) {
		s, err := createWith(t, container.Bag{}, container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), 0)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should take nil files and mappings as none", func(t *testing.T) {
		s, err := create(t, container.Bag{env.ConfigFieldFiles: nil, env.ConfigFieldMappings: nil})
		testutil.NoError(t, err)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should create a new source per call", func(t *testing.T) {
		c := creators(t, defaults(t))[0]
		cfg := container.Bag{factory.ConfigFieldId: "env", factory.ConfigFieldDriver: env.Driver}
		first, err := c.Create(cfg)
		testutil.NoError(t, err)
		second, err := c.Create(cfg)
		testutil.NoError(t, err)
		if first == second {
			t.Fatal("Create() returned the same source twice")
		}
	})

	t.Run("should be created through a factory", func(t *testing.T) {
		t.Setenv("FLAM_ENV_FACTORY", "8080")
		bag := defaults(t)
		testutil.NoError(t, bag.Set(config.SourceConfigPath+".env", container.Bag{
			factory.ConfigFieldDriver: env.Driver,
			env.ConfigFieldMappings:   container.Bag{"FLAM_ENV_FACTORY": "server.port"},
		}))
		f, err := factory.New(bag.Bag(config.SourceConfigPath), creators(t, bag)...)
		testutil.NoError(t, err)
		s, err := f.Get("env")
		testutil.NoError(t, err)
		testutil.Equal(t, "server.port", s.Bag().Int("server.port"), 8080)
		testutil.NoError(t, f.Close())
	})
}

func Test_Creator_InvalidFields(t *testing.T) {
	scenarios := []struct {
		field string
		value any
	}{
		{env.ConfigFieldPriority, "5"},
		{env.ConfigFieldPriority, 5.0},
		{env.ConfigFieldPriority, int64(5)},
		{env.ConfigFieldPriority, nil},
		{env.ConfigFieldFiles, ".env"},
		{env.ConfigFieldFiles, []any{".env", 1}},
		{env.ConfigFieldFiles, []int{1}},
		{env.ConfigFieldMappings, "A=a"},
		{env.ConfigFieldMappings, []any{"A"}},
		{env.ConfigFieldMappings, map[string]string{"A": "a"}},
	}
	for _, s := range scenarios {
		t.Run(fmt.Sprintf("should refuse %s %#v", s.field, s.value), func(t *testing.T) {
			source, err := create(t, container.Bag{s.field: s.value})
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, env.ErrInvalidField,
				"unable to create env config source: invalid env config source field",
				map[string]any{"id": "env", "field": s.field})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		bag := defaults(t)
		testutil.NoError(t, bag.Set(env.DefaultPriorityConfigPath, "high"))
		_, err := createWith(t, bag, container.Bag{})
		testutil.Error(t, err, env.ErrInvalidField,
			"unable to create env config source: invalid env config source field",
			map[string]any{"id": "env", "field": "priority"})
	})
}

func Test_Creator_Mappings(t *testing.T) {
	invalid := []struct {
		name string
		key  string
		path any
	}{
		{"an empty variable name", "", "a"},
		{"a path that is not a string", "A", 1},
		{"a nil path", "A", nil},
		{"a bag as path", "A", container.Bag{"b": "c"}},
		{"an empty path", "A", ""},
		{"a path with an empty segment", "A", "a..b"},
		{"a path starting with a separator", "A", ".a"},
		{"a path ending with a separator", "A", "a."},
	}
	for _, s := range invalid {
		t.Run("should refuse "+s.name, func(t *testing.T) {
			source, err := create(t, container.Bag{env.ConfigFieldMappings: container.Bag{s.key: s.path}})
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, env.ErrInvalidMapping,
				"unable to create env config source: invalid env config source mapping",
				map[string]any{"id": "env", "key": s.key, "path": s.path})
		})
	}

	t.Run("should report the first invalid mapping in key order", func(t *testing.T) {
		_, err := create(t, container.Bag{env.ConfigFieldMappings: container.Bag{"B": "", "A": "a", "C": ""}})
		testutil.Context(t, err.(*flam.Error), map[string]any{"id": "env", "key": "B", "path": ""})
	})

	conflicts := []struct {
		name     string
		mappings container.Bag
		want     map[string]any
	}{
		{"the same path", container.Bag{"A": "a.b", "B": "a.b"},
			map[string]any{"key": "A", "path": "a.b", "other_key": "B", "other_path": "a.b"}},
		{"a path inside the other", container.Bag{"A": "a.b.c", "B": "a.b"},
			map[string]any{"key": "A", "path": "a.b.c", "other_key": "B", "other_path": "a.b"}},
		{"a path containing the other", container.Bag{"A": "a", "B": "a.b"},
			map[string]any{"key": "A", "path": "a", "other_key": "B", "other_path": "a.b"}},
	}
	for _, s := range conflicts {
		t.Run("should refuse two mappings to "+s.name+", set or not", func(t *testing.T) {
			source, err := create(t, container.Bag{env.ConfigFieldMappings: s.mappings})
			if source != nil {
				t.Fatal("Create() returned a source with an error")
			}
			s.want["id"] = "env"
			testutil.Error(t, err, env.ErrMappingConflict,
				"unable to create env config source: conflicting env config source mappings", s.want)
		})
	}

	t.Run("should accept paths sharing a prefix that are not nested", func(t *testing.T) {
		t.Setenv("FLAM_ENV_AB", "1")
		t.Setenv("FLAM_ENV_ABC", "2")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{
			"FLAM_ENV_AB": "a.b", "FLAM_ENV_ABC": "a.bc",
		}})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": container.Bag{"b": 1, "bc": 2}}))
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

func Test_Source_Values(t *testing.T) {
	scenarios := []struct {
		text string
		want any
	}{
		{"true", true},
		{"TRUE", true},
		{"False", false},
		{"0", 0},
		{"8080", 8080},
		{"-1", -1},
		{"0.5", 0.5},
		{"-2.25", -2.25},
		{"9223372036854775808", "9223372036854775808"},
		{"0755", "0755"},
		{"+1", "+1"},
		{"1.20", "1.20"},
		{"1e3", "1e3"},
		{"NaN", "NaN"},
		{"nan", "nan"},
		{"Inf", "Inf"},
		{"+Inf", "+Inf"},
		{"-Inf", "-Inf"},
		{"infinity", "infinity"},
		{" 1", " 1"},
		{"yes", "yes"},
		{"localhost", "localhost"},
		{"a=b", "a=b"},
	}
	for _, s := range scenarios {
		t.Run(fmt.Sprintf("should read %q as %#v", s.text, s.want), func(t *testing.T) {
			t.Setenv("FLAM_ENV_VALUE", s.text)
			got := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_VALUE": "v"}}).
				Bag().Get("v")
			testutil.Equal(t, "value", got, s.want)
			if fmt.Sprintf("%T", got) != fmt.Sprintf("%T", s.want) {
				t.Fatalf("type = %T, want %T", got, s.want)
			}
		})
	}

	t.Run("should leave out the variables not set or empty", func(t *testing.T) {
		unset(t, "FLAM_ENV_UNSET")
		t.Setenv("FLAM_ENV_EMPTY", "")
		t.Setenv("FLAM_ENV_SET", "x")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{
			"FLAM_ENV_UNSET": "a.unset", "FLAM_ENV_EMPTY": "a.empty", "FLAM_ENV_SET": "a.set",
		}})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": container.Bag{"set": "x"}}))
	})

	t.Run("should nest the values along their paths", func(t *testing.T) {
		t.Setenv("FLAM_ENV_HOST", "db")
		t.Setenv("FLAM_ENV_PORT", "5432")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: map[string]any{
			"FLAM_ENV_HOST": "database.primary.host", "FLAM_ENV_PORT": "database.primary.port",
		}})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"database": container.Bag{
			"primary": container.Bag{"host": "db", "port": 5432},
		}}))
		testutil.Equal(t, "Int()", s.Bag().Int("database.primary.port"), 5432)
	})
}

func Test_Source_Files(t *testing.T) {
	t.Run("should read the files, a later file overriding an earlier one", func(t *testing.T) {
		dir := t.TempDir()
		unset(t, "FLAM_ENV_A")
		unset(t, "FLAM_ENV_B")
		first := writeFile(t, dir, ".env", "FLAM_ENV_A=first\nFLAM_ENV_B=first\n")
		second := writeFile(t, dir, ".env.local", "# comment\nexport FLAM_ENV_B=\"second\"\n")
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    []any{first, second},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a", "FLAM_ENV_B": "b"},
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "first", "b": "second"}))
	})

	t.Run("should let the environment override the files", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("FLAM_ENV_A", "env")
		t.Setenv("FLAM_ENV_B", "")
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    []string{writeFile(t, dir, ".env", "FLAM_ENV_A=file\nFLAM_ENV_B=file")},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a", "FLAM_ENV_B": "b"},
		})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "env"}))
	})

	t.Run("should not change the process environment", func(t *testing.T) {
		dir := t.TempDir()
		unset(t, "FLAM_ENV_A")
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    []string{writeFile(t, dir, ".env", "FLAM_ENV_A=file\nFLAM_ENV_OTHER=x")},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"},
		})
		testutil.Equal(t, "a", s.Bag().String("a"), "file")
		for _, key := range []string{"FLAM_ENV_A", "FLAM_ENV_OTHER"} {
			if _, ok := os.LookupEnv(key); ok {
				t.Fatalf("%s was set in the process environment", key)
			}
		}
	})

	t.Run("should not read a .env file when no file is configured", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, ".env", "FLAM_ENV_A=file")
		t.Chdir(dir)
		unset(t, "FLAM_ENV_A")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"}})
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
	})

	t.Run("should return the error of a file, wrapped", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.env")
		files := []string{missing}
		source, err := create(t, container.Bag{env.ConfigFieldFiles: files})
		if source != nil {
			t.Fatal("Create() returned a source with an error")
		}
		testutil.Error(t, err, env.ErrRead,
			"unable to create env config source: unable to read env config files: open "+missing+
				": no such file or directory",
			map[string]any{"id": "env", "files": files})
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("the error does not wrap the file error")
		}
	})

	t.Run("should keep its own copy of the files", func(t *testing.T) {
		dir := t.TempDir()
		unset(t, "FLAM_ENV_A")
		files := []string{writeFile(t, dir, ".env", "FLAM_ENV_A=file")}
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    files,
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"},
		})
		files[0] = filepath.Join(dir, "missing.env")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})
}

func Test_Source_Contract(t *testing.T) {
	t.Run("should close without error, more than once", func(t *testing.T) {
		s := mustCreate(t, container.Bag{})
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should return a copy of its values", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "x")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a.b"}})
		testutil.NoError(t, s.Bag().Set("a.b", "changed"))
		testutil.Equal(t, "a.b", s.Bag().String("a.b"), "x")
	})

	t.Run("should not change with the configuration it was created from", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "x")
		mappings := container.Bag{"FLAM_ENV_A": "a"}
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: mappings})
		mappings["FLAM_ENV_A"] = "b"
		_, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "x"}))
	})
}

func Test_Source_Reload(t *testing.T) {
	t.Run("should report no change when nothing changed", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "x")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"}})
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
	})

	t.Run("should report no change for a NaN text", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "NaN")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"}})
		for range 3 {
			changed, err := s.Reload()
			testutil.NoError(t, err)
			testutil.Equal(t, "changed", changed, false)
		}
	})

	t.Run("should read the environment again", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "x")
		t.Setenv("FLAM_ENV_B", "y")
		unset(t, "FLAM_ENV_C")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{
			"FLAM_ENV_A": "a", "FLAM_ENV_B": "b", "FLAM_ENV_C": "c",
		}})
		t.Setenv("FLAM_ENV_A", "1")
		testutil.NoError(t, os.Unsetenv("FLAM_ENV_B"))
		t.Setenv("FLAM_ENV_C", "z")

		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": 1, "c": "z"}))
	})

	t.Run("should read the files again", func(t *testing.T) {
		dir := t.TempDir()
		unset(t, "FLAM_ENV_A")
		file := writeFile(t, dir, ".env", "FLAM_ENV_A=old")
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    []string{file},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"},
		})
		writeFile(t, dir, ".env", "FLAM_ENV_A=new")
		changed, err := s.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "a", s.Bag().String("a"), "new")
	})

	t.Run("should keep its values when a file fails", func(t *testing.T) {
		dir := t.TempDir()
		unset(t, "FLAM_ENV_A")
		file := writeFile(t, dir, ".env", "FLAM_ENV_A=old")
		s := mustCreate(t, container.Bag{
			env.ConfigFieldFiles:    []string{file},
			env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"},
		})
		testutil.NoError(t, os.Remove(file))
		changed, err := s.Reload()
		testutil.Equal(t, "changed", changed, false)
		testutil.Error(t, err, env.ErrRead,
			"unable to reload env config source: unable to read env config files: open "+file+
				": no such file or directory",
			map[string]any{"id": "env", "files": []string{file}})
		testutil.Equal(t, "a", s.Bag().String("a"), "old")
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		t.Setenv("FLAM_ENV_A", "x")
		s := mustCreate(t, container.Bag{env.ConfigFieldMappings: container.Bag{"FLAM_ENV_A": "a"}})
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				if _, err := s.Reload(); err != nil {
					t.Errorf("Reload() = %v", err)
				}
			})
			wg.Go(func() {
				if got := s.Bag().String("a"); got != "x" {
					t.Errorf("a = %q, want x", got)
				}
			})
		}
		wg.Wait()
	})
}

// envSource creates a source mapping FLAM_ENV_RELOAD to "a", of priority 4,
// with the extra fields, with the reload default when not nil.
func envSource(t *testing.T, reloadDefault any, extra container.Bag) (flam.IConfigSource, error) {
	t.Helper()
	t.Setenv("FLAM_ENV_RELOAD", "x")
	bag := defaults(t)
	if reloadDefault != nil {
		testutil.NoError(t, bag.Set(env.DefaultReloadConfigPath, reloadDefault))
	}
	fields := container.Bag{
		env.ConfigFieldPriority: 4, env.ConfigFieldMappings: container.Bag{"FLAM_ENV_RELOAD": "a"},
	}
	for key, value := range extra.All() {
		fields[key] = value
	}
	return createWith(t, bag, fields)
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
		s, err := envSource(t, nil, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should be reloadable when activated", func(t *testing.T) {
		s, err := envSource(t, nil, container.Bag{env.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), true)
	})

	t.Run("should not be reloadable when deactivated, still being a source", func(t *testing.T) {
		s, err := envSource(t, nil, container.Bag{env.ConfigFieldReload: false})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)
		testutil.Equal(t, "Id()", s.Id(), "env")
		testutil.Equal(t, "Priority()", s.Priority(), 4)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "x"}))
		testutil.NoError(t, s.Bag().Set("x", 1)) // a copy
		testutil.Equal(t, "Bag() after a change of a copy", s.Bag(), flam.IBag(container.Bag{"a": "x"}))
		testutil.NoError(t, s.Close())
		testutil.NoError(t, s.Close())
	})

	t.Run("should take the default stored in the configuration", func(t *testing.T) {
		s, err := envSource(t, false, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable", isReloadable(s), false)

		s, err = envSource(t, false, container.Bag{env.ConfigFieldReload: true})
		testutil.NoError(t, err)
		testutil.Equal(t, "reloadable with the field", isReloadable(s), true)
	})

	for _, value := range []any{"false", 0, nil} {
		t.Run(fmt.Sprintf("should refuse the reload %#v", value), func(t *testing.T) {
			s, err := envSource(t, nil, container.Bag{env.ConfigFieldReload: value})
			if s != nil {
				t.Fatal("Create() returned a source with an error")
			}
			testutil.Error(t, err, env.ErrInvalidField,
				"unable to create env config source: invalid env config source field",
				map[string]any{"id": "env", "field": "reload"})
		})
	}

	t.Run("should refuse an invalid default", func(t *testing.T) {
		_, err := envSource(t, "no", nil)
		testutil.Error(t, err, env.ErrInvalidField,
			"unable to create env config source: invalid env config source field",
			map[string]any{"id": "env", "field": "reload"})
	})
}
