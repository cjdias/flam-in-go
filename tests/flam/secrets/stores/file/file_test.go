package file

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/file"
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

// creators returns the store creators the bundle provides in the store
// creator group, reading the defaults from a configuration of the bag.
func creators(t *testing.T, bag container.Bag) []flam.ISecretStoreCreator {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, file.NewBundle().Register(app))

	var result []flam.ISecretStoreCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ISecretStoreCreator `group:"flam.secrets.stores.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// create creates a store of id "file" from the fields, with the
// configuration of the bag.
func create(t *testing.T, bag container.Bag, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "file", factory.ConfigFieldDriver: file.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creators(t, bag)[0].Create(cfg)
}

// mustCreate creates a trimming store of the directory.
func mustCreate(t *testing.T, dir string) flam.ISecretStore {
	t.Helper()
	s, err := create(t, container.Bag{}, container.Bag{file.ConfigFieldDir: dir})
	testutil.NoError(t, err)
	return s
}

// write writes the file of the slash separated name in the directory,
// creating its parent directories.
func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	testutil.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	testutil.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// secretsDir returns a directory of secrets as Docker and Kubernetes mount
// them, next to a file out of it.
func secretsDir(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	write(t, parent, "outside", "outside")
	dir := filepath.Join(parent, "secrets")
	write(t, dir, "api_token", "tok\n")
	write(t, dir, "raw", "raw\n\n")
	write(t, dir, "db/password", "pw\r\n")
	write(t, dir, "db/user", "admin")
	write(t, dir, "db/.hidden", "hidden")
	write(t, dir, "db/sub/inner", "inner")
	testutil.NoError(t, os.Symlink("password", filepath.Join(dir, "db", "link")))
	testutil.NoError(t, os.Symlink("missing", filepath.Join(dir, "db", "dangling")))
	testutil.NoError(t, os.Symlink(filepath.Join(parent, "outside"), filepath.Join(dir, "escape")))
	return dir
}

// fileError checks the error of a store of id "file" for the key.
func fileError(t *testing.T, err error, sentinel error, key string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T, want a *flam.Error", err)
	}
	testutil.Context(t, e, map[string]any{file.ErrorFieldID: "file", file.ErrorFieldKey: key})
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{file.Id, "file"},
			{file.Driver, "flam.secrets.stores.driver.file"},
			{file.ConfigFieldDir, "dir"},
			{file.ConfigFieldTrim, "trim"},
			{file.DefaultDirConfigPath, "flam.secrets.defaults.stores.file.dir"},
			{file.DefaultTrimConfigPath, "flam.secrets.defaults.stores.file.trim"},
			{file.ErrorFieldID, "id"},
			{file.ErrorFieldField, "field"},
			{file.ErrorFieldKey, "key"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			file.ErrInvalidField: "invalid file secret store field",
			file.ErrRead:         "unable to read secret file",
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
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores.file")
		if b == file.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the default store and the defaults", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, file.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "secrets", bag.Get("flam.secrets"), container.Bag{
			"stores": container.Bag{"file": container.Bag{factory.ConfigFieldDriver: file.Driver}},
			"defaults": container.Bag{"stores": container.Bag{"file": container.Bag{
				"dir": "/run/secrets", "trim": true,
			}}},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := file.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the store creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, file.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, container.Bag{})), 1)
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

func Test_Creator(t *testing.T) {
	t.Run("should accept its driver only", func(t *testing.T) {
		c := creators(t, container.Bag{})[0]
		for driver, want := range map[string]bool{
			file.Driver:                      true,
			"":                               false,
			"file":                           false,
			"flam.secrets.stores.driver.env": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should take the fields, or their defaults", func(t *testing.T) {
		dir := secretsDir(t)
		defaults := container.Bag{}
		testutil.NoError(t, defaults.Set(file.DefaultDirConfigPath, dir))
		testutil.NoError(t, defaults.Set(file.DefaultTrimConfigPath, false))

		for _, c := range []struct {
			name   string
			bag    container.Bag
			fields container.Bag
			want   string
		}{
			{"fields", defaults, container.Bag{file.ConfigFieldDir: dir, file.ConfigFieldTrim: true}, "tok"},
			{"defaults", defaults, container.Bag{}, "tok\n"},
		} {
			s, err := create(t, c.bag, c.fields)
			testutil.NoError(t, err)
			testutil.Equal(t, c.name+" Id()", s.Id(), "file")
			got, err := s.Get(context.Background(), "api_token")
			testutil.NoError(t, err)
			testutil.Equal(t, c.name, got, c.want)
		}
	})

	t.Run("should use the built-in defaults", func(t *testing.T) {
		s, err := create(t, container.Bag{}, container.Bag{})
		testutil.NoError(t, err)
		// the built-in directory is /run/secrets, missing from the test hosts
		// but where Docker mounts its secrets
		if _, statErr := os.Stat("/run/secrets"); statErr == nil {
			t.Skip("the host has a /run/secrets directory")
		}
		_, err = s.Get(context.Background(), "x")
		fileError(t, err, secrets.ErrNotFound, "x")
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, values := range map[string][]any{
			file.ConfigFieldDir:  {1, ""},
			file.ConfigFieldTrim: {"yes"},
		} {
			for _, value := range values {
				_, err := create(t, container.Bag{}, container.Bag{field: value})
				testutil.Error(t, err, file.ErrInvalidField,
					"unable to create file secret store: invalid file secret store field",
					map[string]any{file.ErrorFieldID: "file", file.ErrorFieldField: field})
			}
		}
	})
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

func Test_Store_Get(t *testing.T) {
	dir := secretsDir(t)
	s := mustCreate(t, dir)

	t.Run("should read the file of the key, without its trailing newline", func(t *testing.T) {
		for key, want := range map[string]string{
			"api_token":   "tok",
			"raw":         "raw\n",
			"db#password": "pw",
			"db/password": "pw",
			"db#user":     "admin",
			"db#link":     "pw",
			"db#.hidden":  "hidden",
		} {
			got, err := s.Get(context.Background(), key)
			testutil.NoError(t, err)
			testutil.Equal(t, key, got, want)
		}
	})

	t.Run("should keep the content when not trimming", func(t *testing.T) {
		raw, err := create(t, container.Bag{}, container.Bag{file.ConfigFieldDir: dir, file.ConfigFieldTrim: false})
		testutil.NoError(t, err)
		got, err := raw.Get(context.Background(), "db#password")
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, "pw\r\n")
	})

	t.Run("should refuse the keys naming no file of the directory", func(t *testing.T) {
		for _, key := range []string{"", ".", "..", "../outside", "db/../../outside", "/etc/passwd", "db//user", "db#", "#user", "./api_token"} {
			_, err := s.Get(context.Background(), key)
			fileError(t, err, secrets.ErrInvalidKey, key)
			testutil.Equal(t, key+" text", err.Error(), "unable to get file secret: invalid secret key")
		}
	})

	t.Run("should refuse a link leading out of the directory", func(t *testing.T) {
		_, err := s.Get(context.Background(), "escape")
		fileError(t, err, file.ErrRead, "escape")
		if strings.Contains(err.Error(), "outside\n") || errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("should not find a missing file or directory", func(t *testing.T) {
		for _, key := range []string{"missing", "db#missing", "db#dangling"} {
			_, err := s.Get(context.Background(), key)
			fileError(t, err, secrets.ErrNotFound, key)
			testutil.Equal(t, key+" text", err.Error(), "unable to get file secret: secret not found")
		}
		_, err := mustCreate(t, filepath.Join(dir, "missing")).Get(context.Background(), "x")
		fileError(t, err, secrets.ErrNotFound, "x")
	})

	t.Run("should fail to read a directory", func(t *testing.T) {
		_, err := s.Get(context.Background(), "db")
		fileError(t, err, file.ErrRead, "db")
		if !strings.HasPrefix(err.Error(), "unable to get file secret: unable to read secret file: ") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("should fail to open a directory that is a file", func(t *testing.T) {
		_, err := mustCreate(t, filepath.Join(dir, "api_token")).Get(context.Background(), "x")
		fileError(t, err, file.ErrRead, "x")
	})
}

func Test_Store_GetMap(t *testing.T) {
	dir := secretsDir(t)
	s := mustCreate(t, dir)

	t.Run("should read the regular files of the directory of the path", func(t *testing.T) {
		got, err := s.GetMap(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"password": "pw", "user": "admin", "link": "pw"})

		got, err = s.GetMap(context.Background(), "db#sub")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"inner": "inner"})
	})

	t.Run("should read the directory of the store for an empty path", func(t *testing.T) {
		got, err := s.GetMap(context.Background(), "")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"api_token": "tok", "raw": "raw\n"})
	})

	t.Run("should refuse the paths naming no directory of the directory", func(t *testing.T) {
		for _, path := range []string{".", "..", "../", "/tmp", "db/", "a//b"} {
			_, err := s.GetMap(context.Background(), path)
			fileError(t, err, secrets.ErrInvalidKey, path)
			testutil.Equal(t, path+" text", err.Error(), "unable to get file secrets: invalid secret key")
		}
	})

	t.Run("should not find a missing directory", func(t *testing.T) {
		_, err := s.GetMap(context.Background(), "missing")
		fileError(t, err, secrets.ErrNotFound, "missing")
		_, err = mustCreate(t, filepath.Join(dir, "missing")).GetMap(context.Background(), "")
		fileError(t, err, secrets.ErrNotFound, "")
	})

	t.Run("should fail to read a file as a directory", func(t *testing.T) {
		_, err := s.GetMap(context.Background(), "api_token")
		fileError(t, err, file.ErrRead, "api_token")
		_, err = mustCreate(t, filepath.Join(dir, "api_token")).GetMap(context.Background(), "")
		fileError(t, err, file.ErrRead, "")
	})

	t.Run("should fail on a file that cannot be read", func(t *testing.T) {
		locked := t.TempDir()
		write(t, locked, "secret", "x")
		testutil.NoError(t, os.Chmod(filepath.Join(locked, "secret"), 0))
		_, err := mustCreate(t, locked).GetMap(context.Background(), "")
		fileError(t, err, file.ErrRead, "")
	})

	t.Run("should close without error", func(t *testing.T) {
		testutil.NoError(t, s.Close())
	})
}
