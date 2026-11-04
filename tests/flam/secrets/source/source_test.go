package source

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets"
	"github.com/cjdias/flam-in-go/pkg/flam/secrets/source"
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

// fakeStore is a store of id "fake" reading its secrets from a map, or
// failing with err.
type fakeStore struct {
	mu      sync.Mutex
	secrets map[string]string
	err     error
	reads   int
}

func (s *fakeStore) Id() string   { return "fake" }
func (s *fakeStore) Close() error { return nil }

func (s *fakeStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if s.err != nil {
		return "", s.err
	}
	value, ok := s.secrets[key]
	if !ok {
		return "", flam.NewWrappedError(secrets.ErrNotFound, "unable to get fake secret")
	}
	return value, nil
}

func (s *fakeStore) GetMap(context.Context, string) (map[string]string, error) { return nil, nil }

// set sets the secret of the key, or the error of the store.
func (s *fakeStore) set(key, value string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.secrets[key] = value
	s.err = err
}

// fixture is an application providing itself, a configuration of a bag
// and, unless without, a store factory holding a fake store, with the bundle
// registered.
type fixture struct {
	app   *testutil.App
	store *fakeStore
}

// newFixture returns a fixture of the configuration bag.
func newFixture(t *testing.T, bag container.Bag, withFactory bool) *fixture {
	t.Helper()
	f := &fixture{
		app:   testutil.NewApp(),
		store: &fakeStore{secrets: map[string]string{"db#password": "s3cret", "db#user": "admin"}},
	}
	testutil.NoError(t, f.app.Provide(func() flam.IConfig { return fakeConfig{bag: bag} }))
	testutil.NoError(t, f.app.Provide(func() flam.IApp { return f.app }))
	if withFactory {
		testutil.NoError(t, f.app.Provide(func() (flam.ISecretStoreFactory, error) {
			stores, err := factory.New[flam.ISecretStore](nil)
			if err != nil {
				return nil, err
			}
			return stores, stores.Store("fake", f.store)
		}))
	}
	testutil.NoError(t, f.app.RegisterBundle(source.NewBundle()))
	return f
}

// creator returns the single creator of the source creator group.
func (f *fixture) creator(t *testing.T) flam.IConfigSourceCreator {
	t.Helper()
	var result []flam.IConfigSourceCreator
	testutil.NoError(t, f.app.Invoke(func(args struct {
		dig.In
		Creators []flam.IConfigSourceCreator `group:"flam.config.sources.creator"`
	}) {
		result = args.Creators
	}))
	testutil.Equal(t, "creators", len(result), 1)
	return result[0]
}

// create creates a source of id "secrets" of the fields with the creator.
func create(c flam.IConfigSourceCreator, fields container.Bag) (flam.IConfigSource, error) {
	cfg := container.Bag{factory.ConfigFieldId: "secrets", factory.ConfigFieldDriver: source.Driver}
	for key, value := range fields.All() {
		cfg[key] = value
	}
	return c.Create(cfg)
}

// mustCreate creates a source of the fields, reading the fake store, with
// a new fixture.
func mustCreate(t *testing.T, fields container.Bag) (flam.IConfigSource, *fixture) {
	t.Helper()
	f := newFixture(t, container.Bag{}, true)
	all := container.Bag{source.ConfigFieldStore: "fake"}
	for key, value := range fields.All() {
		all[key] = value
	}
	s, err := create(f.creator(t), all)
	testutil.NoError(t, err)
	return s, f
}

// idField returns the context of an error of the field.
func idField(field string) map[string]any {
	return map[string]any{source.ErrorFieldID: "secrets", source.ErrorFieldField: field}
}

// idPath returns the context of an error of the path.
func idPath(path string) map[string]any {
	return map[string]any{source.ErrorFieldID: "secrets", source.ErrorFieldPath: path}
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	t.Run("should declare the driver constants", func(t *testing.T) {
		for _, c := range []struct{ got, want string }{
			{source.Id, "secrets"},
			{source.Driver, "flam.config.sources.driver.secrets"},
			{source.ConfigFieldPriority, "priority"},
			{source.ConfigFieldReload, "reload"},
			{source.ConfigFieldStore, "store"},
			{source.ConfigFieldValues, "values"},
			{source.DefaultPriorityConfigPath, "flam.secrets.source.priority"},
			{source.DefaultReloadConfigPath, "flam.secrets.source.reload"},
			{source.DefaultStoreConfigPath, "flam.secrets.source.store"},
			{source.DefaultValuesConfigPath, "flam.secrets.source.values"},
			{source.ErrorFieldID, "id"},
			{source.ErrorFieldField, "field"},
			{source.ErrorFieldPath, "path"},
			{source.ErrorFieldKey, "key"},
			{source.ErrorFieldStore, "store"},
		} {
			testutil.Equal(t, c.want, c.got, c.want)
		}
	})

	t.Run("should declare the sentinels with their messages", func(t *testing.T) {
		for err, text := range map[error]string{
			source.ErrInvalidField:   "invalid secrets config source field",
			source.ErrInvalidValue:   "invalid secrets config source value",
			source.ErrMissingFactory: "secret store factory not provided by the container",
			source.ErrStore:          "unable to get secrets config source store",
			source.ErrSecret:         "unable to read secrets config source secret",
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
		b := source.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.secrets.source")
		if b == source.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Fatal("the bundle is closable, but holds nothing")
		}
	})

	t.Run("should configure the default source and the defaults", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, source.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.Equal(t, "source", bag.Get(config.SourceConfigPath), container.Bag{
			"secrets": container.Bag{factory.ConfigFieldDriver: source.Driver},
		})
		testutil.Equal(t, "defaults", bag.Get("flam.secrets"), container.Bag{"source": container.Bag{
			"priority": 0, "reload": false, "store": "", "values": container.Bag{},
		}})
	})

	t.Run("should return the error of the bag", func(t *testing.T) {
		err := source.NewBundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
		if !errors.Is(err, container.ErrPathConflict) {
			t.Fatalf("Config() = %v, want container.ErrPathConflict", err)
		}
	})

	t.Run("should provide a single creator in the source creator group", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, source.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
		newFixture(t, container.Bag{}, false).creator(t)
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		app := testutil.NewApp()
		boom := errors.New("boom")
		app.Fail = func(string) error { return boom }
		if err := source.NewBundle().Register(app); !errors.Is(err, boom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})
}

// ---------------------------------------------------------------------------
// creator
// ---------------------------------------------------------------------------

func Test_Creator(t *testing.T) {
	t.Run("should accept its driver only", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, false).creator(t)
		for driver, want := range map[string]bool{
			source.Driver:                    true,
			"":                               false,
			"secrets":                        false,
			"flam.config.sources.driver.env": false,
		} {
			testutil.Equal(t, driver, c.Accept(container.Bag{factory.ConfigFieldDriver: driver}), want)
		}
	})

	t.Run("should create a source of the secrets of the values", func(t *testing.T) {
		s, f := mustCreate(t, container.Bag{
			source.ConfigFieldPriority: 5,
			source.ConfigFieldValues: container.Bag{
				"flam.database.connections.main.password": "db#password",
				"app": map[string]any{
					"db": container.Bag{"user": "db#user"},
				},
				"none": container.Bag(nil),
			},
		})
		testutil.Equal(t, "Id()", s.Id(), "secrets")
		testutil.Equal(t, "Priority()", s.Priority(), 5)
		testutil.NoError(t, s.Close())
		want := container.Bag{}
		testutil.NoError(t, want.Set("flam.database.connections.main.password", "s3cret"))
		testutil.NoError(t, want.Set("app.db.user", "admin"))
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(want))

		bag := s.Bag()
		testutil.NoError(t, bag.Set("app.db.user", "changed"))
		testutil.Equal(t, "copy", s.Bag().Get("app.db.user"), "admin")

		if _, ok := s.(flam.IConfigReloadableSource); ok {
			t.Fatal("the source is reloadable, but its reload is deactivated")
		}
		calls := f.app.Calls()
		testutil.Equal(t, "invoked", calls[len(calls)-1], "invoker/0")
	})

	t.Run("should take the defaults stored in the configuration", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(source.DefaultPriorityConfigPath, 3))
		testutil.NoError(t, bag.Set(source.DefaultReloadConfigPath, true))
		testutil.NoError(t, bag.Set(source.DefaultStoreConfigPath, "fake"))
		testutil.NoError(t, bag.Set(source.DefaultValuesConfigPath, container.Bag{"a": "db#user"}))
		s, err := create(newFixture(t, bag, true).creator(t), container.Bag{})
		testutil.NoError(t, err)
		testutil.Equal(t, "Priority()", s.Priority(), 3)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"a": "admin"}))
		if _, ok := s.(flam.IConfigReloadableSource); !ok {
			t.Fatal("the source is not reloadable")
		}
	})

	t.Run("should create an empty source without store", func(t *testing.T) {
		f := newFixture(t, container.Bag{}, false)
		for _, fields := range []container.Bag{{}, {source.ConfigFieldValues: map[string]any{}}} {
			s, err := create(f.creator(t), fields)
			testutil.NoError(t, err)
			testutil.Equal(t, "Priority()", s.Priority(), 0)
			testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{}))
		}
	})

	t.Run("should take the store factory from the container once", func(t *testing.T) {
		f := newFixture(t, container.Bag{}, true)
		c := f.creator(t)
		before := len(f.app.Calls())
		for range 3 {
			_, err := create(c, container.Bag{
				source.ConfigFieldStore:  "fake",
				source.ConfigFieldValues: container.Bag{"a": "db#user"},
			})
			testutil.NoError(t, err)
		}
		testutil.Equal(t, "invokes", len(f.app.Calls())-before, 1)
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, true).creator(t)
		for field, values := range map[string][]any{
			source.ConfigFieldPriority: {"high"},
			source.ConfigFieldReload:   {"yes"},
			source.ConfigFieldStore:    {1, ""},
			source.ConfigFieldValues:   {"a", []any{"x"}},
		} {
			for _, value := range values {
				_, err := create(c, container.Bag{
					source.ConfigFieldStore:  "fake",
					source.ConfigFieldValues: container.Bag{"a": "db#user"},
					field:                    value,
				})
				testutil.Error(t, err, source.ErrInvalidField,
					"unable to create secrets config source: invalid secrets config source field", idField(field))
			}
		}
	})

	t.Run("should refuse the invalid values", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, true).creator(t)
		for path, values := range map[string]container.Bag{
			"a":     {"a": 1},
			"b":     {"b": ""},
			"c":     {"c": nil},
			"":      {"": "db#user"},
			"d.":    {"d": container.Bag{"": "db#user"}},
			"e..f":  {"e..f": "db#user"},
			"g.h":   {"g": "db#user", "g.h": "db#user"},
			"i.j.k": {"i": container.Bag{"j": container.Bag{"k": []string{"x"}}}},
		} {
			_, err := create(c, container.Bag{source.ConfigFieldStore: "fake", source.ConfigFieldValues: values})
			testutil.Error(t, err, source.ErrInvalidValue,
				"unable to create secrets config source: invalid secrets config source value", idPath(path))
		}
	})

	t.Run("should fail without store factory", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, false).creator(t)
		_, err := create(c, container.Bag{
			source.ConfigFieldStore:  "fake",
			source.ConfigFieldValues: container.Bag{"a": "db#user"},
		})
		if !errors.Is(err, source.ErrMissingFactory) {
			t.Fatalf("error = %v, want ErrMissingFactory", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("error = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{source.ErrorFieldID: "secrets"})
	})

	t.Run("should fail on an unknown store", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, true).creator(t)
		_, err := create(c, container.Bag{
			source.ConfigFieldStore:  "unknown",
			source.ConfigFieldValues: container.Bag{"a": "db#user"},
		})
		if !errors.Is(err, source.ErrStore) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("error = %v, want ErrStore and factory.ErrUnknownID", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		testutil.Context(t, e, map[string]any{source.ErrorFieldID: "secrets", source.ErrorFieldStore: "unknown"})
	})

	t.Run("should fail on a secret that cannot be read", func(t *testing.T) {
		c := newFixture(t, container.Bag{}, true).creator(t)
		_, err := create(c, container.Bag{
			source.ConfigFieldStore:  "fake",
			source.ConfigFieldValues: container.Bag{"a": "db#user", "b": "db#missing"},
		})
		if !errors.Is(err, source.ErrSecret) || !errors.Is(err, secrets.ErrNotFound) {
			t.Fatalf("error = %v, want ErrSecret and secrets.ErrNotFound", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		testutil.Context(t, e, map[string]any{
			source.ErrorFieldID: "secrets", source.ErrorFieldPath: "b", source.ErrorFieldKey: "db#missing",
		})
		if !strings.HasPrefix(err.Error(), "unable to create secrets config source: unable to read secrets config source secret: ") {
			t.Fatalf("error = %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// source
// ---------------------------------------------------------------------------

func Test_Source_Reload(t *testing.T) {
	t.Run("should read the secrets again, reporting a change", func(t *testing.T) {
		s, f := mustCreate(t, container.Bag{
			source.ConfigFieldReload: true,
			source.ConfigFieldValues: container.Bag{"db.password": "db#password"},
		})
		r := s.(flam.IConfigReloadableSource)

		changed, err := r.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "unchanged", changed, false)

		f.store.set("db#password", "rotated", nil)
		changed, err = r.Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, true)
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"db": container.Bag{"password": "rotated"}}))
	})

	t.Run("should keep the values when a secret fails", func(t *testing.T) {
		s, f := mustCreate(t, container.Bag{
			source.ConfigFieldReload: true,
			source.ConfigFieldValues: container.Bag{"db.password": "db#password"},
		})
		boom := errors.New("boom")
		f.store.set("db#password", "rotated", boom)
		changed, err := s.(flam.IConfigReloadableSource).Reload()
		testutil.Equal(t, "changed", changed, false)
		if !errors.Is(err, source.ErrSecret) || !errors.Is(err, boom) {
			t.Fatalf("Reload() = %v, want ErrSecret and boom", err)
		}
		if !strings.HasPrefix(err.Error(), "unable to reload secrets config source: ") {
			t.Fatalf("error = %v", err)
		}
		testutil.Equal(t, "Bag()", s.Bag(), flam.IBag(container.Bag{"db": container.Bag{"password": "s3cret"}}))
	})

	t.Run("should reload nothing without values", func(t *testing.T) {
		s, f := mustCreate(t, container.Bag{source.ConfigFieldReload: true})
		changed, err := s.(flam.IConfigReloadableSource).Reload()
		testutil.NoError(t, err)
		testutil.Equal(t, "changed", changed, false)
		testutil.Equal(t, "reads", f.store.reads, 0)
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		s, _ := mustCreate(t, container.Bag{
			source.ConfigFieldReload: true,
			source.ConfigFieldValues: container.Bag{"db.password": "db#password"},
		})
		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				_, _ = s.(flam.IConfigReloadableSource).Reload()
				_ = s.Bag()
			})
		}
		wg.Wait()
	})
}
