package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/source"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/env"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/file"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

var errBoom = errors.New("boom")

// failing is a store failing to close.
type failing struct{ flam.ISecretStore }

func (failing) Id() string   { return "bad" }
func (failing) Close() error { return errBoom }

// newApp returns a testutil.App with the config and secrets bundles.
func newApp(t *testing.T) *testutil.App {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(secrets.NewBundle()))
	return a
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the paths, groups and fields", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{secrets.StoreConfigPath, "flam.secrets.stores"},
			{secrets.StoreCreatorGroup, "flam.secrets.stores.creator"},
			{secrets.ErrorFieldID, "id"},
			{secrets.ErrorFieldKey, "key"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			secrets.ErrNotFound:   "secret not found",
			secrets.ErrInvalidKey: "invalid secret key",
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
		b := secrets.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets")
		if b == secrets.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IConfigurableBundle); ok {
			t.Fatal("the bundle is configurable, but has no default")
		}
	})

	t.Run("should provide the store factory", func(t *testing.T) {
		a := newApp(t)
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-2:], []string{"bundle:flam.secrets", "provider/0"})
		testutil.NoError(t, a.Invoke(func(f flam.ISecretStoreFactory) {
			testutil.Equal(t, "stores", f.Available(), []string{})
		}))
	})

	t.Run("should read the stores from the configuration", func(t *testing.T) {
		a := newApp(t)
		testutil.NoError(t, a.RegisterBundle(env.NewBundle()))
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) error {
			return c.Set(secrets.StoreConfigPath+".main", container.Bag{factory.ConfigFieldDriver: env.Driver})
		}))
		testutil.NoError(t, a.Invoke(func(f flam.ISecretStoreFactory) {
			testutil.Equal(t, "stores", f.Available(), []string{"main"})
			s, err := f.Get("main")
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", s.Id(), "main")
		}))
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		a := testutil.NewApp()
		a.Fail = func(string) error { return errBoom }
		if err := secrets.NewBundle().Register(a); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})

	t.Run("should close the factory and its stores", func(t *testing.T) {
		a := newApp(t)
		var f flam.ISecretStoreFactory
		testutil.NoError(t, a.Invoke(func(sf flam.ISecretStoreFactory) { f = sf }))
		testutil.NoError(t, f.Store("bad", failing{}))

		err := secrets.NewBundle().(flam.IClosableBundle).Close(a)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want boom", err)
		}
		testutil.Equal(t, "closed", f.Has("bad"), false)
		testutil.NoError(t, secrets.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to close without the factory dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(secrets.NewBundle()))
		if err := secrets.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

// ---------------------------------------------------------------------------
// stores bundle
// ---------------------------------------------------------------------------

func Test_StoresBundle(t *testing.T) {
	t.Run("should have its id and register every driver", func(t *testing.T) {
		b := stores.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores")
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		for _, id := range []string{
			"flam.secrets.stores.env", "flam.secrets.stores.file", "flam.secrets.stores.vault",
		} {
			testutil.Equal(t, id, a.HasBundle(id), true)
		}
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		a := testutil.NewApp()
		a.Fail = func(string) error { return errBoom }
		if err := stores.NewBundle().Register(a); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// application
// ---------------------------------------------------------------------------

// newApplication returns an application of the values with the secrets
// bundles, booted.
func newApplication(t *testing.T, values container.Bag) flam.IApp {
	t.Helper()
	a := app.New(values)
	t.Cleanup(func() { _ = a.Close() })
	for _, b := range []flam.IBundle{secrets.NewBundle(), stores.NewBundle(), source.NewBundle()} {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Boot())
	return a
}

func Test_Application(t *testing.T) {
	t.Run("should load the secrets into the configuration", func(t *testing.T) {
		t.Setenv("FLAM_SECRETS_DB_PASSWORD", "s3cret")
		values := container.Bag{}
		testutil.NoError(t, values.Set(env.DefaultPrefixConfigPath, "FLAM_SECRETS_"))
		testutil.NoError(t, values.Set(source.DefaultStoreConfigPath, env.Id))
		testutil.NoError(t, values.Set(source.DefaultValuesConfigPath, container.Bag{
			"flam.database.connections.main.password": "db#password",
		}))
		a := newApplication(t, values)

		testutil.NoError(t, a.Invoke(func(c flam.IConfig, f flam.ISecretStoreFactory) error {
			testutil.Equal(t, "password", c.String("flam.database.connections.main.password"), "s3cret")
			testutil.Equal(t, "source", c.HasSource(source.Id), true)
			s, err := f.Get(env.Id)
			if err != nil {
				return err
			}
			got, err := s.Get(context.Background(), "db#password")
			testutil.Equal(t, "Get()", got, "s3cret")
			return err
		}))
	})

	t.Run("should take the stores and values of the sources added before", func(t *testing.T) {
		dir := t.TempDir()
		testutil.NoError(t, os.MkdirAll(filepath.Join(dir, "db"), 0o700))
		testutil.NoError(t, os.WriteFile(filepath.Join(dir, "db", "password"), []byte("pw\n"), 0o600))
		t.Setenv("FLAM_SECRETS_STORE", "files")
		t.Setenv("FLAM_SECRETS_DIR", dir)
		t.Setenv("FLAM_SECRETS_KEY", "db#password")

		values := container.Bag{}
		testutil.NoError(t, values.Set(config.SourceConfigPath+".app", container.Bag{
			factory.ConfigFieldDriver: "flam.config.sources.driver.env",
			"mappings": container.Bag{
				"FLAM_SECRETS_STORE": source.DefaultStoreConfigPath,
				"FLAM_SECRETS_DIR":   secrets.StoreConfigPath + ".files.dir",
				"FLAM_SECRETS_KEY":   source.DefaultValuesConfigPath + ".app.password",
			},
		}))
		testutil.NoError(t, values.Set(secrets.StoreConfigPath+".files.driver", file.Driver))
		a := newApplication(t, values)

		testutil.NoError(t, a.Invoke(func(c flam.IConfig) {
			testutil.Equal(t, "password", c.String("app.password"), "pw")
		}))
	})

	t.Run("should boot without values nor store", func(t *testing.T) {
		a := newApplication(t, nil)
		testutil.NoError(t, a.Invoke(func(c flam.IConfig) {
			testutil.Equal(t, "source", c.HasSource(source.Id), true)
		}))
	})
}
