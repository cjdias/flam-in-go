package env

import (
	"context"
	"errors"
	"os"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/stores/env"
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
	testutil.NoError(t, env.NewBundle().Register(app))

	var result []flam.ISecretStoreCreator
	testutil.NoError(t, app.Invoke(func(args struct {
		dig.In
		Creators []flam.ISecretStoreCreator `group:"flam.secrets.stores.creator"`
	}) {
		result = args.Creators
	}))
	return result
}

// create creates a store of id "env" from the fields, with the
// configuration of the bag.
func create(t *testing.T, bag container.Bag, fields container.Bag) (flam.ISecretStore, error) {
	t.Helper()
	cfg := container.Bag{factory.ConfigFieldId: "env", factory.ConfigFieldDriver: env.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return creators(t, bag)[0].Create(cfg)
}

// mustCreate creates a store of the prefix.
func mustCreate(t *testing.T, prefix string) flam.ISecretStore {
	t.Helper()
	s, err := create(t, container.Bag{}, container.Bag{env.ConfigFieldPrefix: prefix})
	testutil.NoError(t, err)
	return s
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{env.Id, "env"},
			{env.Driver, "flam.secrets.stores.driver.env"},
			{env.ConfigFieldPrefix, "prefix"},
			{env.DefaultPrefixConfigPath, "flam.secrets.defaults.stores.env.prefix"},
			{env.ErrorFieldID, "id"},
			{env.ErrorFieldField, "field"},
			{env.ErrorFieldKey, "key"},
			{env.ErrorFieldVariable, "variable"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		testutil.Equal(t, "message", env.ErrInvalidField.Error(), "invalid env secret store field")
		testutil.Equal(t, "unwrap", errors.Unwrap(env.ErrInvalidField), nil)
	})
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and return a new bundle per call", func(t *testing.T) {
		b := env.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.stores.env")
		if b == env.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the default store and the defaults", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, env.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "secrets", bag.Get("flam.secrets"), container.Bag{
			"stores": container.Bag{"env": container.Bag{factory.ConfigFieldDriver: env.Driver}},
			"defaults": container.Bag{"stores": container.Bag{"env": container.Bag{
				"prefix": "",
			}}},
		})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := env.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the store creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, env.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		testutil.Equal(t, "creators", len(creators(t, container.Bag{})), 1)
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

func Test_Creator(t *testing.T) {
	t.Run("should accept its driver only", func(t *testing.T) {
		c := creators(t, container.Bag{})[0]
		for driver, want := range map[string]bool{
			env.Driver:                        true,
			"":                                false,
			"env":                             false,
			"flam.secrets.stores.driver.file": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should take the prefix of the field, the default or none", func(t *testing.T) {
		t.Setenv("FIELD_K", "field")
		t.Setenv("DEFAULT_K", "default")
		t.Setenv("K", "none")
		defaults := container.Bag{}
		testutil.NoError(t, defaults.Set(env.DefaultPrefixConfigPath, "DEFAULT_"))

		for _, c := range []struct {
			name   string
			bag    container.Bag
			fields container.Bag
			want   string
		}{
			{"field", defaults, container.Bag{env.ConfigFieldPrefix: "FIELD_"}, "field"},
			{"default", defaults, container.Bag{}, "default"},
			{"none", container.Bag{}, container.Bag{}, "none"},
		} {
			s, err := create(t, c.bag, c.fields)
			testutil.NoError(t, err)
			testutil.Equal(t, c.name+" Id()", s.Id(), "env")
			got, err := s.Get(context.Background(), "k")
			testutil.NoError(t, err)
			testutil.Equal(t, c.name, got, c.want)
		}
	})

	t.Run("should refuse a prefix that is not a string", func(t *testing.T) {
		defaults := container.Bag{}
		testutil.NoError(t, defaults.Set(env.DefaultPrefixConfigPath, 1))
		for _, c := range []struct{ bag, fields container.Bag }{
			{container.Bag{}, container.Bag{env.ConfigFieldPrefix: 1}},
			{defaults, container.Bag{}},
		} {
			_, err := create(t, c.bag, c.fields)
			testutil.Error(t, err, env.ErrInvalidField,
				"unable to create env secret store: invalid env secret store field",
				map[string]any{env.ErrorFieldID: "env", env.ErrorFieldField: env.ConfigFieldPrefix})
		}
	})
}

// ---------------------------------------------------------------------------
// store
// ---------------------------------------------------------------------------

func Test_Store_Get(t *testing.T) {
	t.Run("should read the variable named after the key", func(t *testing.T) {
		t.Setenv("APP_DB_PASSWORD", "s3cret")
		t.Setenv("APP_API_KEY", "key")
		t.Setenv("APP_A1_B", "a1b")
		t.Setenv("APP_X_Y_Z", "xyz")
		t.Setenv("APP_EMPTY", "")
		s := mustCreate(t, "APP_")

		for key, want := range map[string]string{
			"db#password": "s3cret",
			"db.password": "s3cret",
			"dbPassword":  "s3cret",
			"DB_PASSWORD": "s3cret",
			"api_key":     "key",
			"API_KEY":     "key",
			"a1B":         "a1b",
			"x-y/z":       "xyz",
			"x-é":         "", // not set: APP_X__
			"empty":       "",
		} {
			got, err := s.Get(context.Background(), key)
			if key == "x-é" {
				testutil.Error(t, err, secrets.ErrNotFound, "unable to get env secret: secret not found",
					map[string]any{env.ErrorFieldID: "env", env.ErrorFieldKey: key, env.ErrorFieldVariable: "APP_X__"})
				continue
			}
			testutil.NoError(t, err)
			testutil.Equal(t, key, got, want)
		}
	})

	t.Run("should refuse an empty key", func(t *testing.T) {
		_, err := mustCreate(t, "").Get(context.Background(), "")
		testutil.Error(t, err, secrets.ErrInvalidKey, "unable to get env secret: invalid secret key",
			map[string]any{env.ErrorFieldID: "env", env.ErrorFieldKey: ""})
	})

	t.Run("should not find a variable that is not set", func(t *testing.T) {
		t.Setenv("APP_MISSING", "")
		testutil.NoError(t, os.Unsetenv("APP_MISSING"))
		_, err := mustCreate(t, "APP_").Get(context.Background(), "missing")
		testutil.Error(t, err, secrets.ErrNotFound, "unable to get env secret: secret not found",
			map[string]any{env.ErrorFieldID: "env", env.ErrorFieldKey: "missing", env.ErrorFieldVariable: "APP_MISSING"})
	})
}

func Test_Store_GetMap(t *testing.T) {
	t.Setenv("FLAMSECRETS_DB_USER", "admin")
	t.Setenv("FLAMSECRETS_DB_PASSWORD", "s3cret")
	t.Setenv("FLAMSECRETS_DB_", "no name")
	t.Setenv("FLAMSECRETS_TOKEN", "tok")

	t.Run("should read the variables of the path", func(t *testing.T) {
		got, err := mustCreate(t, "FLAMSECRETS_").GetMap(context.Background(), "db")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"user": "admin", "password": "s3cret"})
	})

	t.Run("should read the variables of the prefix for an empty path", func(t *testing.T) {
		got, err := mustCreate(t, "FLAMSECRETS_").GetMap(context.Background(), "")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{
			"db_user": "admin", "db_password": "s3cret", "db_": "no name", "token": "tok",
		})
	})

	t.Run("should read the variables of the path without prefix", func(t *testing.T) {
		got, err := mustCreate(t, "").GetMap(context.Background(), "flamsecrets#db")
		testutil.NoError(t, err)
		testutil.Equal(t, "GetMap()", got, map[string]string{"user": "admin", "password": "s3cret"})
	})

	t.Run("should refuse to read the whole environment", func(t *testing.T) {
		_, err := mustCreate(t, "").GetMap(context.Background(), "")
		testutil.Error(t, err, secrets.ErrInvalidKey, "unable to get env secrets: invalid secret key",
			map[string]any{env.ErrorFieldID: "env", env.ErrorFieldKey: ""})
	})

	t.Run("should not find a path without variables", func(t *testing.T) {
		_, err := mustCreate(t, "FLAMSECRETS_").GetMap(context.Background(), "missing")
		testutil.Error(t, err, secrets.ErrNotFound, "unable to get env secrets: secret not found",
			map[string]any{env.ErrorFieldID: "env", env.ErrorFieldKey: "missing", env.ErrorFieldVariable: "FLAMSECRETS_MISSING_"})
	})

	t.Run("should close without error", func(t *testing.T) {
		testutil.NoError(t, mustCreate(t, "").Close())
	})
}
