package config

import (
	"errors"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers/yaml"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/dir"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/env"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/file"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/rest"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// appConfig returns the configuration provided by the config bundle
// registered in the app, after adding the defaults to it.
func appConfig(t *testing.T, app *testutil.App, defaults flam.IBag) flam.IConfig {
	t.Helper()
	var cfg flam.IConfig
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) { cfg = c }))
	testutil.NoError(t, cfg.AddDefaults(defaults))
	return cfg
}

// newApp returns a testutil.App with the bundles registered, the
// configuration provided by the config bundle, among them, holding the
// defaults of the configurable bundles.
func newApp(t *testing.T, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	app := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	// the drivers are registered by the aggregators: configure them too
	bag := container.Bag{}
	for _, b := range []flam.IBundle{json.NewBundle(), yaml.NewBundle(), env.NewBundle(), file.NewBundle(), dir.NewBundle(), rest.NewBundle()} {
		if app.HasBundle(b.Id()) {
			testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		}
	}
	appConfig(t, app, bag)
	return app
}

// factories returns the parser and source factories of the app container.
func factories(t *testing.T, app *testutil.App) (flam.IConfigParserFactory, flam.IConfigSourceFactory) {
	t.Helper()
	var p flam.IConfigParserFactory
	var s flam.IConfigSourceFactory
	testutil.NoError(t, app.Invoke(func(pf flam.IConfigParserFactory, sf flam.IConfigSourceFactory) {
		p, s = pf, sf
	}))
	return p, s
}

// failingSource is a flam.IConfigSource whose Close fails.
type failingSource struct {
	err error
}

func (s *failingSource) Id() string     { return "failing" }
func (s *failingSource) Close() error   { return s.err }
func (s *failingSource) Priority() int  { return 0 }
func (s *failingSource) Bag() flam.IBag { return container.Bag{} }

// bootCreator is a flam.IConfigSourceCreator of the "boot" driver,
// creating a source holding {"from": id}, or {"x..y": 1}, a key the
// configuration refuses, when the "invalid" field is true, or failing with
// the error fail when the "fail" field is true.
type bootCreator struct {
	fail error
}

func (c bootCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "boot"
}

func (c bootCreator) Create(bag flam.IBag) (flam.IConfigSource, error) {
	id := bag.String(factory.ConfigFieldId)
	switch {
	case bag.Bool("fail"):
		return nil, c.fail
	case bag.Bool("invalid"):
		return &bootSource{id: id, values: container.Bag{"x..y": 1}}, nil
	default:
		return &bootSource{id: id, values: container.Bag{"from": id}}, nil
	}
}

// bootSource is the flam.IConfigSource created by bootCreator.
type bootSource struct {
	id     string
	values container.Bag
}

func (s *bootSource) Id() string     { return s.id }
func (s *bootSource) Close() error   { return nil }
func (s *bootSource) Priority() int  { return 0 }
func (s *bootSource) Bag() flam.IBag { return s.values }

// newBootApp returns a testutil.App with the config bundle registered, a
// real configuration holding the defaults, and the "boot" source driver
// failing with fail.
func newBootApp(t *testing.T, defaults flam.IBag, fail error) (*testutil.App, flam.IConfig) {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(
		func() flam.IConfigSourceCreator { return bootCreator{fail: fail} },
		dig.Group(config.SourceCreatorGroup),
	))
	testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
	return app, appConfig(t, app, defaults)
}

// ---------------------------------------------------------------------------
// config bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should have its id and be bootable and closable", func(t *testing.T) {
		b := config.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.config")
		if b == config.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}
		if _, ok := b.(flam.IClosableBundle); !ok {
			t.Fatal("the bundle does not implement IClosableBundle")
		}
		if _, ok := b.(flam.IBootableBundle); !ok {
			t.Fatal("the bundle does not implement IBootableBundle")
		}
		if _, ok := b.(flam.IConfigurableBundle); ok {
			t.Fatal("the bundle implements IConfigurableBundle")
		}
	})

	t.Run("should provide the configuration and the factories, without drivers", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, config.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0", "provider/0", "provider/0", "provider/0", "provider/0"})

		cfg := appConfig(t, app, container.Bag{})
		if cfg != appConfig(t, app, container.Bag{}) {
			t.Fatal("the container did not provide a single IConfig")
		}
		testutil.Equal(t, "Entries()", len(cfg.Entries()), 0)
		p, s := factories(t, app)
		testutil.Equal(t, "parsers", p.Available(), []string{})
		testutil.Equal(t, "sources", s.Available(), []string{})
	})

	t.Run("should create the parsers and sources of the drivers", func(t *testing.T) {
		app := newApp(t, config.NewBundle(), parsers.NewBundle(), sources.NewBundle())
		p, _ := factories(t, app)
		testutil.Equal(t, "parsers", p.Available(), []string{"json", "yaml"})
		parser, err := p.Get(yaml.Id)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", parser.Id(), "yaml")
	})

	t.Run("should return the error of the application", func(t *testing.T) {
		boom := errors.New("boom")
		for name, b := range map[string]flam.IBundle{
			"config": config.NewBundle(), "parsers": parsers.NewBundle(), "sources": sources.NewBundle(),
		} {
			app := testutil.NewApp()
			app.Fail = func(string) error { return boom }
			if err := b.Register(app); !errors.Is(err, boom) {
				t.Errorf("%s Register() = %v, want boom", name, err)
			}
		}
	})
}

func Test_Bundle_Boot(t *testing.T) {
	boot := func(t *testing.T, app *testutil.App) error {
		t.Helper()
		b := config.NewBundle()
		return b.(flam.IBootableBundle).Boot(app)
	}
	sources := container.Bag{
		"flam": container.Bag{"config": container.Bag{"sources": container.Bag{
			"a": container.Bag{"driver": "boot"},
			"b": container.Bag{"driver": "boot"},
		}}},
	}

	t.Run("should add every configured source, in id order", func(t *testing.T) {
		app, cfg := newBootApp(t, sources, nil)
		var seen []any
		testutil.NoError(t, cfg.AddObserver("o", "from", func(_, newValue any) {
			seen = append(seen, newValue)
		}))
		testutil.NoError(t, boot(t, app))
		testutil.Equal(t, "HasSource(a)", cfg.HasSource("a"), true)
		testutil.Equal(t, "HasSource(b)", cfg.HasSource("b"), true)
		testutil.Equal(t, "from", cfg.String("from"), "b")
		testutil.Equal(t, "observed", seen, []any{"a", "b"})
	})

	t.Run("should add nothing when there is no configured source", func(t *testing.T) {
		app, cfg := newBootApp(t, container.Bag{}, nil)
		testutil.NoError(t, boot(t, app))
		testutil.Equal(t, "Entries()", len(cfg.Entries()), 0)
	})

	t.Run("should add nothing when the boot is disabled", func(t *testing.T) {
		defaults := sources.Clone()
		testutil.NoError(t, defaults.Set(config.BootConfigPath, false))
		app, cfg := newBootApp(t, defaults, nil)
		testutil.NoError(t, boot(t, app))
		testutil.Equal(t, "HasSource(a)", cfg.HasSource("a"), false)
		testutil.Equal(t, "HasSource(b)", cfg.HasSource("b"), false)
	})

	t.Run("should boot when the boot flag is not a bool", func(t *testing.T) {
		defaults := sources.Clone()
		testutil.NoError(t, defaults.Set(config.BootConfigPath, "no"))
		app, cfg := newBootApp(t, defaults, nil)
		testutil.NoError(t, boot(t, app))
		testutil.Equal(t, "HasSource(a)", cfg.HasSource("a"), true)
	})

	t.Run("should skip the sources already added", func(t *testing.T) {
		app, cfg := newBootApp(t, sources, nil)
		testutil.NoError(t, cfg.AddSource(&bootSource{id: "a", values: container.Bag{"own": true}}))
		testutil.NoError(t, boot(t, app))
		testutil.Equal(t, "own", cfg.Bool("own"), true)
		testutil.Equal(t, "HasSource(b)", cfg.HasSource("b"), true)
		testutil.NoError(t, app.Invoke(func(f flam.IConfigSourceFactory) {
			testutil.Equal(t, "Stored()", f.Stored(), []string{"b"})
		}))
	})

	t.Run("should stop at a source failing to be created", func(t *testing.T) {
		boom := errors.New("boom")
		defaults := sources.Clone()
		testutil.NoError(t, defaults.Set("flam.config.sources.b.fail", true))
		testutil.NoError(t, defaults.Set("flam.config.sources.c", container.Bag{"driver": "boot"}))
		app, cfg := newBootApp(t, defaults, boom)

		err := boot(t, app)
		if !errors.Is(err, boom) {
			t.Fatalf("Boot() = %v, want boom", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Boot() = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{config.ErrorFieldID: "b"})
		testutil.Equal(t, "HasSource(a)", cfg.HasSource("a"), true)
		testutil.Equal(t, "HasSource(b)", cfg.HasSource("b"), false)
		testutil.Equal(t, "HasSource(c)", cfg.HasSource("c"), false)
	})

	t.Run("should fail when a source cannot be added", func(t *testing.T) {
		defaults := sources.Clone()
		testutil.NoError(t, defaults.Set("flam.config.sources.a.invalid", true))
		app, cfg := newBootApp(t, defaults, nil)

		err := boot(t, app)
		if !errors.Is(err, container.ErrInvalidPath) {
			t.Fatalf("Boot() = %v, want container.ErrInvalidPath", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Boot() = %T, want a *flam.Error", err)
		}
		testutil.Context(t, e, map[string]any{config.ErrorFieldID: "a"})
		testutil.Equal(t, "HasSource(a)", cfg.HasSource("a"), false)
		testutil.Equal(t, "HasSource(b)", cfg.HasSource("b"), false)
	})

	t.Run("should fail when the bundle is not registered", func(t *testing.T) {
		if err := boot(t, testutil.NewApp()); err == nil {
			t.Fatal("Boot() without the providers = nil, want an error")
		}
	})
}

func Test_Bundle_Close(t *testing.T) {
	t.Run("should close both factories", func(t *testing.T) {
		b := config.NewBundle()
		app := newApp(t, b, parsers.NewBundle())
		p, s := factories(t, app)
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		if _, err := p.Get(json.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("parser Get() after Close = %v, want factory.ErrClosed", err)
		}
		if err := s.Store("failing", &failingSource{}); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("source Store() after Close = %v, want factory.ErrClosed", err)
		}
	})

	t.Run("should create and close the factories when never used", func(t *testing.T) {
		b := config.NewBundle()
		app := newApp(t, b, parsers.NewBundle())
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		p, _ := factories(t, app)
		if _, err := p.Get(json.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("parser Get() after Close = %v, want factory.ErrClosed", err)
		}
	})

	t.Run("should close the parser factory even when the source factory fails", func(t *testing.T) {
		boom := errors.New("boom")
		b := config.NewBundle()
		app := newApp(t, b, parsers.NewBundle())
		p, s := factories(t, app)
		testutil.NoError(t, s.Store("failing", &failingSource{err: boom}))

		if err := b.(flam.IClosableBundle).Close(app); !errors.Is(err, boom) {
			t.Fatalf("Close() = %v, want boom", err)
		}
		if _, err := p.Get(json.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("parser Get() after Close = %v, want factory.ErrClosed", err)
		}
	})

	t.Run("should fail when the bundle is not registered", func(t *testing.T) {
		if err := config.NewBundle().(flam.IClosableBundle).Close(testutil.NewApp()); err == nil {
			t.Fatal("Close() without the providers = nil, want an error")
		}
	})
}

// ---------------------------------------------------------------------------
// driver aggregators
// ---------------------------------------------------------------------------

func Test_Aggregators(t *testing.T) {
	scenarios := []struct {
		name   string
		bundle func() flam.IBundle
		calls  []string
	}{
		{"parsers", parsers.NewBundle, []string{
			"bundle:flam.config.parsers",
			"bundle:flam.config.parsers.json", "provider/1",
			"bundle:flam.config.parsers.yaml", "provider/1",
		}},
		{"sources", sources.NewBundle, []string{
			"bundle:flam.config.sources",
			"bundle:flam.config.sources.env", "provider/1",
			"bundle:flam.config.sources.file", "provider/1",
			"bundle:flam.config.sources.dir", "provider/1",
			"bundle:flam.config.sources.rest", "provider/0", "provider/1",
		}},
	}
	for _, s := range scenarios {
		t.Run("should register every "+s.name+" driver, and nothing else", func(t *testing.T) {
			b := s.bundle()
			testutil.Equal(t, "Id()", b.Id(), "flam.config."+s.name)
			app := testutil.NewApp()
			testutil.NoError(t, app.RegisterBundle(b))
			testutil.Equal(t, "calls", app.Calls(), s.calls)
			if _, ok := b.(flam.IConfigurableBundle); ok {
				t.Error("the bundle implements IConfigurableBundle")
			}
			if _, ok := b.(flam.IClosableBundle); ok {
				t.Error("the bundle implements IClosableBundle")
			}
		})
	}
}
