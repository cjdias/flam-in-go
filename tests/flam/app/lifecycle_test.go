package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/clock"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/config/parsers"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/dir"
	"github.com/cjdias/flam-in-go/pkg/flam/config/sources/file"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams"
	logfile "github.com/cjdias/flam-in-go/pkg/flam/log/streams/file"
	"github.com/cjdias/flam-in-go/pkg/flam/pubsub"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// plainBundle is a flam.IBundle implementing no optional interface, whose
// Register calls register when set.
type plainBundle struct {
	id       string
	register func(a flam.IApp) error
}

func (b *plainBundle) Id() string { return b.id }

func (b *plainBundle) Register(a flam.IApp) error {
	if b.register != nil {
		return b.register(a)
	}
	return nil
}

// fullBundle is a bundle implementing every optional interface, logging
// each call as "id.method", then calling the function of the method when
// set.
type fullBundle struct {
	id  string
	log *[]string

	config   func(bag flam.IBag) error
	register func(a flam.IApp) error
	boot     func(a flam.IApp) error
	run      func(a flam.IApp) error
	close    func(a flam.IApp) error
}

var (
	_ flam.IConfigurableBundle = (*fullBundle)(nil)
	_ flam.IBootableBundle     = (*fullBundle)(nil)
	_ flam.IRunnableBundle     = (*fullBundle)(nil)
	_ flam.IClosableBundle     = (*fullBundle)(nil)
)

func (b *fullBundle) Id() string { return b.id }

func (b *fullBundle) Config(bag flam.IBag) error {
	*b.log = append(*b.log, b.id+".config")
	if b.config != nil {
		return b.config(bag)
	}
	return nil
}

func (b *fullBundle) Register(a flam.IApp) error { return b.call("register", b.register, a) }
func (b *fullBundle) Boot(a flam.IApp) error     { return b.call("boot", b.boot, a) }
func (b *fullBundle) Run(a flam.IApp) error      { return b.call("run", b.run, a) }
func (b *fullBundle) Close(a flam.IApp) error    { return b.call("close", b.close, a) }

func (b *fullBundle) call(method string, fn func(flam.IApp) error, a flam.IApp) error {
	*b.log = append(*b.log, b.id+"."+method)
	if fn != nil {
		return fn(a)
	}
	return nil
}

// asFlamError returns err as a *flam.Error, failing the test otherwise.
func asFlamError(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %T %v, want a *flam.Error", err, err)
	}
	return e
}

// coreBundles returns new bundles of the ids New registers.
func coreBundles() []flam.IBundle {
	return []flam.IBundle{
		clock.NewBundle(), pubsub.NewBundle(), trigger.NewBundle(), disk.NewBundle(), disks.NewBundle(),
		config.NewBundle(), parsers.NewBundle(), sources.NewBundle(),
		log.NewBundle(), serializers.NewBundle(), streams.NewBundle(),
	}
}

// configOf returns the configuration provided by the config bundle.
func configOf(t *testing.T, a flam.IApp) flam.IConfig {
	t.Helper()
	var cfg flam.IConfig
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) { cfg = c }))
	return cfg
}

// set sets a value in a new bag.
func set(t *testing.T, path string, v any) container.Bag {
	t.Helper()
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(path, v))
	return bag
}

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func Test_New(t *testing.T) {
	t.Run("should provide the application", func(t *testing.T) {
		a := app.New(nil)
		var provided flam.IApp
		testutil.NoError(t, a.Invoke(func(p flam.IApp) { provided = p }))
		if provided != a {
			t.Fatal("the container did not provide the application")
		}
		if first, second := a.Container(), a.Container(); first == nil || first != second {
			t.Fatal("Container() did not return a single container")
		}
	})

	t.Run("should register the core bundles", func(t *testing.T) {
		a := app.New(nil)
		for _, b := range coreBundles() {
			testutil.Equal(t, "HasBundle("+b.Id()+")", a.HasBundle(b.Id()), true)
			testutil.Error(t, a.RegisterBundle(b), app.ErrDuplicateBundle,
				"unable to register application bundle: application bundle id already registered",
				map[string]any{app.ErrorFieldBundle: b.Id()})
		}
		testutil.NoError(t, a.Invoke(func(
			flam.IClock, flam.IPubSub[string, string], flam.ITriggerFactory, flam.IDiskFactory,
			flam.IConfig, flam.IConfigParserFactory, flam.IConfigSourceFactory,
			flam.ILogger, flam.ILogSerializerFactory, flam.ILogStreamFactory,
		) {
		}))
		testutil.NoError(t, a.Invoke(func(f flam.ILogSerializerFactory) {
			testutil.Equal(t, "serializers", f.Available(), []string{"json", "stringify"})
		}))
		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Close())
	})

	t.Run("should refuse to provide again a core value, but decorate it", func(t *testing.T) {
		a := app.New(nil)
		clk := testutil.NewClock(time.Time{})
		if err := a.Provide(func() flam.IClock { return clk }); err == nil {
			t.Fatal("Provide() of an IClock = nil, want an error")
		}
		testutil.NoError(t, a.Decorate(func(flam.IClock) flam.IClock { return clk }))
		testutil.NoError(t, a.Invoke(func(c flam.IClock) {
			if c != clk {
				t.Error("the decorated IClock is not the fake one")
			}
		}))
	})

	t.Run("should give the values to the configuration of the config bundle", func(t *testing.T) {
		a := app.New(set(t, "a.b", 1))
		if configOf(t, a) != configOf(t, a) {
			t.Fatal("the container did not provide a single IConfig")
		}
		testutil.Equal(t, "a.b", configOf(t, a).Int("a.b"), 1)
	})

	t.Run("should start the configuration with a copy of the values", func(t *testing.T) {
		values := container.Bag{"a": container.Bag{"b": 1}}
		a := app.New(values)
		testutil.NoError(t, values.Set("a.b", 2))
		testutil.Equal(t, "a.b", configOf(t, a).Int("a.b"), 1)
	})

	t.Run("should take a nil bag as no values", func(t *testing.T) {
		a := app.New(container.Bag(nil))
		testutil.Equal(t, "Entries()", configOf(t, a).Entries(), []string{"flam"})
		testutil.NoError(t, a.Boot())
	})

	t.Run("should fail the boot and run when the values are refused", func(t *testing.T) {
		a := app.New(container.Bag{"x..y": 1})
		b := &fullBundle{id: "b", log: &[]string{}}
		testutil.NoError(t, a.RegisterBundle(b))

		err := a.Boot()
		if !errors.Is(err, container.ErrInvalidPath) {
			t.Fatalf("Boot() = %v, want container.ErrInvalidPath", err)
		}
		if !strings.HasPrefix(err.Error(), "unable to boot application: ") {
			t.Errorf("Boot() = %q, want the boot message", err)
		}
		testutil.Context(t, asFlamError(t, err), map[string]any{})
		if runErr := a.Run(); runErr != err {
			t.Fatalf("Run() = %v, want the boot error", runErr)
		}
		testutil.Equal(t, "log", *b.log, []string{"b.config", "b.register"})
	})
}

// ---------------------------------------------------------------------------
// RegisterBundle
// ---------------------------------------------------------------------------

func Test_RegisterBundle(t *testing.T) {
	t.Run("should configure, then register the bundle", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		b := &fullBundle{id: "b", log: &log, config: func(bag flam.IBag) error {
			return bag.Set("b.v", 1)
		}, register: func(a flam.IApp) error {
			return a.Provide(func() int { return 42 })
		}}
		testutil.Equal(t, "HasBundle()", a.HasBundle("b"), false)
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "HasBundle()", a.HasBundle("b"), true)
		testutil.Equal(t, "log", log, []string{"b.config", "b.register"})
		testutil.Equal(t, "b.v", configOf(t, a).Int("b.v"), 1)
		testutil.NoError(t, a.Invoke(func(v int) { testutil.Equal(t, "provided", v, 42) }))
	})

	t.Run("should register a bundle implementing no optional interface", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.RegisterBundle(&plainBundle{id: "p"}))
		testutil.Equal(t, "HasBundle()", a.HasBundle("p"), true)
		testutil.NoError(t, a.Boot())
		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Close())
	})

	t.Run("should keep the values of New and the first defaults", func(t *testing.T) {
		first := &fullBundle{id: "first", log: &[]string{}, config: func(bag flam.IBag) error {
			return errors.Join(bag.Set("v.new", "first"), bag.Set("v.first", "first"))
		}}
		second := &fullBundle{id: "second", log: &[]string{}, config: func(bag flam.IBag) error {
			return errors.Join(bag.Set("v.first", "second"), bag.Set("v.second", "second"))
		}}
		third := &fullBundle{id: "third", log: &[]string{}, config: func(bag flam.IBag) error {
			return errors.Join(bag.Set("v.second", "third"), bag.Set("v.third", "third"))
		}}
		want := map[string]any{"new": "new", "first": "first", "second": "second", "third": "third"}

		// the configuration taken before the first, the last, or no bundle
		for _, taken := range []int{0, 2, -1} {
			a := app.New(set(t, "v.new", "new"))
			for i, b := range []flam.IBundle{first, second, third} {
				if i == taken {
					configOf(t, a)
				}
				testutil.NoError(t, a.RegisterBundle(b))
			}
			testutil.Equal(t, "v", configOf(t, a).StringMap("v"), want)
		}
	})

	t.Run("should keep the defaults of the core bundles, but not over the values", func(t *testing.T) {
		a := app.New(set(t, dir.DefaultDiskConfigPath, memory.Id))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &[]string{}, config: func(bag flam.IBag) error {
			return errors.Join(bag.Set(file.DefaultDiskConfigPath, memory.Id), bag.Set(dir.DefaultDiskConfigPath, "other"))
		}}))
		cfg := configOf(t, a)
		testutil.Equal(t, "file disk", cfg.String(file.DefaultDiskConfigPath), local.Id)
		testutil.Equal(t, "dir disk", cfg.String(dir.DefaultDiskConfigPath), memory.Id)
	})

	t.Run("should register the bundles registered by a bundle", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		inner := &fullBundle{id: "inner", log: &log}
		outer := &fullBundle{id: "outer", log: &log, register: func(a flam.IApp) error {
			return a.RegisterBundle(inner)
		}}
		testutil.NoError(t, a.RegisterBundle(outer))
		testutil.Equal(t, "HasBundle(inner)", a.HasBundle("inner"), true)
		testutil.NoError(t, a.Boot())
		testutil.Equal(t, "log", log, []string{
			"outer.config", "outer.register", "inner.config", "inner.register", "outer.boot", "inner.boot",
		})
	})

	for name, b := range map[string]flam.IBundle{"nil": nil, "typed nil": (*plainBundle)(nil)} {
		t.Run("should refuse a "+name+" bundle", func(t *testing.T) {
			testutil.Error(t, app.New(nil).RegisterBundle(b), app.ErrNilBundle,
				"unable to register application bundle: nil application bundle", map[string]any{})
		})
	}

	t.Run("should refuse a duplicate id", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&plainBundle{id: "b"}))
		testutil.Error(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}), app.ErrDuplicateBundle,
			"unable to register application bundle: application bundle id already registered",
			map[string]any{app.ErrorFieldBundle: "b"})
		testutil.Equal(t, "log", log, []string{})
	})

	t.Run("should keep a bundle whose Config fails, without registering it", func(t *testing.T) {
		boom := errors.New("boom")
		a := app.New(nil)
		log := []string{}
		err := a.RegisterBundle(&fullBundle{id: "b", log: &log, config: func(flam.IBag) error { return boom }})
		testutil.Error(t, err, boom, "unable to register application bundle: boom",
			map[string]any{app.ErrorFieldBundle: "b"})
		testutil.Equal(t, "HasBundle()", a.HasBundle("b"), true)
		testutil.Equal(t, "log", log, []string{"b.config"})

		// registered in part, it fails the boot, booting no bundle
		for _, err := range []error{a.Boot(), a.Run()} {
			testutil.Error(t, err, boom, "unable to boot application: unable to register application bundle: boom",
				map[string]any{})
		}
		testutil.Equal(t, "log", log, []string{"b.config"})
		testutil.NoError(t, a.Close())
	})

	t.Run("should keep a copy of the defaults of a bundle", func(t *testing.T) {
		a := app.New(nil)
		var kept flam.IBag
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &[]string{}, config: func(bag flam.IBag) error {
			kept = bag
			return bag.Set("v", 1)
		}}))
		testutil.NoError(t, kept.Set("v", 2))
		testutil.Equal(t, "v", configOf(t, a).Int("v"), 1)
	})

	for name, taken := range map[string]bool{"before": false, "after": true} {
		t.Run("should fail when the configuration refuses the defaults, "+name+" it is taken", func(t *testing.T) {
			a := app.New(nil)
			if taken {
				configOf(t, a)
			}
			log := []string{}
			err := a.RegisterBundle(&fullBundle{id: "b", log: &log, config: func(bag flam.IBag) error {
				bag.(container.Bag)["x..y"] = 1
				return bag.Set("ok", true)
			}})
			if !errors.Is(err, container.ErrInvalidPath) {
				t.Fatalf("RegisterBundle() = %v, want container.ErrInvalidPath", err)
			}
			testutil.Context(t, asFlamError(t, err), map[string]any{app.ErrorFieldBundle: "b"})
			testutil.Equal(t, "log", log, []string{"b.config"})
			testutil.Equal(t, "HasBundle()", a.HasBundle("b"), true)
			testutil.Equal(t, "Has(ok)", configOf(t, a).Has("ok"), false)
		})
	}

	t.Run("should keep a bundle whose Register fails", func(t *testing.T) {
		boom := errors.New("boom")
		a := app.New(nil)
		err := a.RegisterBundle(&plainBundle{id: "b", register: func(flam.IApp) error { return boom }})
		testutil.Error(t, err, boom, "unable to register application bundle: boom",
			map[string]any{app.ErrorFieldBundle: "b"})
		testutil.Equal(t, "HasBundle()", a.HasBundle("b"), true)
		testutil.Error(t, a.Boot(), boom, "unable to boot application: unable to register application bundle: boom",
			map[string]any{})
	})

	t.Run("should refuse a bundle once booting", func(t *testing.T) {
		a := app.New(nil)
		var during error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &[]string{}, boot: func(a flam.IApp) error {
			during = a.RegisterBundle(&plainBundle{id: "during"})
			return nil
		}}))
		testutil.NoError(t, a.Boot())
		testutil.Error(t, during, app.ErrBooted,
			"unable to register application bundle: application already booted", map[string]any{})
		testutil.Error(t, a.RegisterBundle(&plainBundle{id: "after"}), app.ErrBooted,
			"unable to register application bundle: application already booted", map[string]any{})
		testutil.Equal(t, "HasBundle()", a.HasBundle("during") || a.HasBundle("after"), false)
	})

	t.Run("should refuse a bundle once closing", func(t *testing.T) {
		a := app.New(nil)
		var during error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &[]string{}, close: func(a flam.IApp) error {
			during = a.RegisterBundle(&plainBundle{id: "during"})
			return nil
		}}))
		testutil.NoError(t, a.Close())
		testutil.Error(t, during, app.ErrClosed,
			"unable to register application bundle: application closed", map[string]any{})
		testutil.Error(t, a.RegisterBundle(&plainBundle{id: "after"}), app.ErrClosed,
			"unable to register application bundle: application closed", map[string]any{})
	})
}

// ---------------------------------------------------------------------------
// container
// ---------------------------------------------------------------------------

func Test_Container(t *testing.T) {
	type dep struct{ v int }

	t.Run("should provide, decorate and invoke", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.Provide(func() *dep { return &dep{v: 1} }))
		testutil.NoError(t, a.Decorate(func(d *dep) *dep { return &dep{v: d.v + 1} }))
		testutil.NoError(t, a.Invoke(func(d *dep) { testutil.Equal(t, "v", d.v, 2) }))
		testutil.NoError(t, a.Invoke(func(d *dep) { testutil.Equal(t, "v", d.v, 2) }, dig.FillInvokeInfo(&dig.InvokeInfo{})))
	})

	methods := []struct {
		name    string
		message string
		call    func(a flam.IApp, fn any) error
	}{
		{"Invoke", "unable to invoke application function", func(a flam.IApp, fn any) error { return a.Invoke(fn) }},
		{"Provide", "unable to provide application function", func(a flam.IApp, fn any) error { return a.Provide(fn) }},
		{"Decorate", "unable to decorate application function", func(a flam.IApp, fn any) error { return a.Decorate(fn) }},
	}
	for _, m := range methods {
		for name, fn := range map[string]any{"nil": nil, "nil function": (func())(nil), "non function": 42} {
			t.Run(m.name+" should refuse a "+name, func(t *testing.T) {
				testutil.Error(t, m.call(app.New(nil), fn), app.ErrInvalidFunction,
					m.message+": invalid application function", map[string]any{})
			})
		}

		t.Run(m.name+" should fail once closed", func(t *testing.T) {
			a := app.New(nil)
			testutil.NoError(t, a.Close())
			testutil.Error(t, m.call(a, func() *dep { return nil }), app.ErrClosed,
				m.message+": application closed", map[string]any{})
		})
	}

	t.Run("should wrap the errors of the container", func(t *testing.T) {
		a := app.New(nil)

		err := a.Invoke(func(*dep) {})
		if err == nil || !strings.HasPrefix(err.Error(), "unable to invoke application function: ") {
			t.Fatalf("Invoke() = %v, want a wrapped missing dependency", err)
		}
		testutil.Context(t, asFlamError(t, err), map[string]any{})

		testutil.NoError(t, a.Provide(func() *dep { return &dep{} }))
		err = a.Provide(func() *dep { return &dep{} })
		if err == nil || !strings.HasPrefix(err.Error(), "unable to provide application function: ") {
			t.Fatalf("Provide() = %v, want a wrapped duplicate provider", err)
		}

		testutil.NoError(t, a.Decorate(func(d *dep) *dep { return d }))
		err = a.Decorate(func(d *dep) *dep { return d })
		if err == nil || !strings.HasPrefix(err.Error(), "unable to decorate application function: ") {
			t.Fatalf("Decorate() = %v, want a wrapped duplicate decorator", err)
		}
	})

	t.Run("should return the error of the invoked function", func(t *testing.T) {
		boom := errors.New("boom")
		err := app.New(nil).Invoke(func() error { return boom })
		testutil.Error(t, err, boom, "unable to invoke application function: boom", map[string]any{})
	})
}

// ---------------------------------------------------------------------------
// Boot and Run
// ---------------------------------------------------------------------------

func Test_Boot(t *testing.T) {
	t.Run("should boot the bundles once, in registration order", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log}))
		testutil.NoError(t, a.RegisterBundle(&plainBundle{id: "p"}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		log = log[:0]

		testutil.NoError(t, a.Boot())
		testutil.NoError(t, a.Boot())
		testutil.Equal(t, "log", log, []string{"a.boot", "b.boot"})
	})

	t.Run("should stop at the first failure, and keep returning it", func(t *testing.T) {
		boom := errors.New("boom")
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, boot: func(flam.IApp) error { return boom }}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		log = log[:0]

		err := a.Boot()
		testutil.Error(t, err, boom, "unable to boot application: boom", map[string]any{app.ErrorFieldBundle: "a"})
		if again := a.Boot(); again != err {
			t.Fatalf("Boot() again = %v, want the same error", again)
		}
		testutil.Equal(t, "log", log, []string{"a.boot"})
	})

	t.Run("should fail when called back while booting, as the boot is not over", func(t *testing.T) {
		a := app.New(nil)
		boom := errors.New("boom")
		var during error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &[]string{}, boot: func(a flam.IApp) error {
			during = a.Boot()
			return nil
		}}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &[]string{}, boot: func(flam.IApp) error {
			return boom
		}}))
		testutil.Error(t, a.Boot(), boom, "unable to boot application: boom", map[string]any{app.ErrorFieldBundle: "b"})
		testutil.Error(t, during, app.ErrBooting, "unable to boot application: application booting", map[string]any{})
	})

	t.Run("should stop when a bundle closes the application", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, boot: func(a flam.IApp) error {
			return a.Close()
		}}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		testutil.Error(t, a.Boot(), app.ErrClosed, "unable to boot application: application closed", map[string]any{})
		testutil.Equal(t, "log", log, []string{"a.config", "a.register", "b.config", "b.register", "a.boot", "b.close", "a.close"})
	})

	t.Run("should refuse to run while booting", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		var runErr error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, boot: func(a flam.IApp) error {
			runErr = a.Run()
			return nil
		}}))
		testutil.NoError(t, a.Boot())
		testutil.Error(t, runErr, app.ErrBooting, "unable to run application: application booting", map[string]any{})
		testutil.Equal(t, "log", log, []string{"a.config", "a.register", "a.boot"})
		testutil.NoError(t, a.Run())
	})

	t.Run("should fail once closed", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.Close())
		testutil.Error(t, a.Boot(), app.ErrClosed, "unable to boot application: application closed", map[string]any{})
	})
}

func Test_Run(t *testing.T) {
	t.Run("should boot, then run the bundles in registration order", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log}))
		testutil.NoError(t, a.RegisterBundle(&plainBundle{id: "p"}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		log = log[:0]

		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Run())
		testutil.Equal(t, "log", log, []string{"a.boot", "b.boot", "a.run", "b.run", "a.run", "b.run"})
	})

	t.Run("should fail when called back while running", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		var during error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, run: func(a flam.IApp) error {
			during = a.Run()
			return nil
		}}))
		testutil.NoError(t, a.Run())
		testutil.Error(t, during, app.ErrRunning, "unable to run application: application running", map[string]any{})
		testutil.NoError(t, a.Run()) // runs again once over
	})

	t.Run("should run nothing when the boot fails", func(t *testing.T) {
		boom := errors.New("boom")
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, boot: func(flam.IApp) error { return boom }}))
		log = log[:0]

		testutil.Error(t, a.Run(), boom, "unable to boot application: boom", map[string]any{app.ErrorFieldBundle: "a"})
		testutil.Error(t, a.Run(), boom, "unable to boot application: boom", map[string]any{app.ErrorFieldBundle: "a"})
		testutil.Equal(t, "log", log, []string{"a.boot"})
	})

	t.Run("should stop at the first failure", func(t *testing.T) {
		boom := errors.New("boom")
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, run: func(flam.IApp) error { return boom }}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		log = log[:0]

		testutil.Error(t, a.Run(), boom, "unable to run application: boom", map[string]any{app.ErrorFieldBundle: "a"})
		testutil.Equal(t, "log", log, []string{"a.boot", "b.boot", "a.run"})
	})

	t.Run("should stop when a bundle closes the application", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, run: func(a flam.IApp) error {
			return a.Close()
		}}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		testutil.Error(t, a.Run(), app.ErrClosed, "unable to run application: application closed", map[string]any{})
		testutil.Equal(t, "log", log, []string{
			"a.config", "a.register", "b.config", "b.register", "a.boot", "b.boot", "a.run", "b.close", "a.close",
		})
	})

	t.Run("should fail once closed", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.Close())
		testutil.Error(t, a.Run(), app.ErrClosed, "unable to run application: application closed", map[string]any{})
	})
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func Test_Close(t *testing.T) {
	t.Run("should close the bundles once, in reverse registration order", func(t *testing.T) {
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log}))
		testutil.NoError(t, a.RegisterBundle(&plainBundle{id: "p"}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log}))
		log = log[:0]

		testutil.NoError(t, a.Close())
		testutil.NoError(t, a.Close())
		testutil.Equal(t, "log", log, []string{"b.close", "a.close"})
		testutil.Equal(t, "HasBundle()", a.HasBundle("a"), true)
		if a.Container() == nil {
			t.Fatal("Container() = nil after Close")
		}
	})

	t.Run("should close every bundle, joining their errors", func(t *testing.T) {
		boomA, boomB := errors.New("boom a"), errors.New("boom b")
		a := app.New(nil)
		log := []string{}
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &log, close: func(flam.IApp) error { return boomA }}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "m", log: &log}))
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "b", log: &log, close: func(flam.IApp) error { return boomB }}))
		log = log[:0]

		err := a.Close()
		testutil.Equal(t, "log", log, []string{"b.close", "m.close", "a.close"})
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			t.Fatalf("Close() = %T, want joined errors", err)
		}
		errs := joined.Unwrap()
		testutil.Equal(t, "errors", len(errs), 2)
		testutil.Error(t, errs[0], boomB, "unable to close application bundle: boom b", map[string]any{app.ErrorFieldBundle: "b"})
		testutil.Error(t, errs[1], boomA, "unable to close application bundle: boom a", map[string]any{app.ErrorFieldBundle: "a"})
	})

	t.Run("should let the bundles use the container while closing", func(t *testing.T) {
		a := app.New(nil)
		var during, again error
		testutil.NoError(t, a.RegisterBundle(&fullBundle{id: "a", log: &[]string{}, close: func(a flam.IApp) error {
			during = a.Invoke(func(flam.IApp) {})
			again = a.Close()
			return nil
		}}))
		testutil.NoError(t, a.Close())
		testutil.NoError(t, during)
		testutil.NoError(t, again)
	})
}

// ---------------------------------------------------------------------------
// with the core bundles
// ---------------------------------------------------------------------------

func Test_CoreBundles(t *testing.T) {
	t.Run("should boot the configuration and log bundles, and close them", func(t *testing.T) {
		a := app.New(nil)
		testutil.NoError(t, a.Run())

		cfg := configOf(t, a)
		testutil.Equal(t, "reloader", cfg.HasObserver(config.ReloaderTriggerID, config.ReloadPeriodConfigPath), true)
		testutil.Equal(t, "flusher", cfg.HasObserver(log.FlusherTriggerID, log.FlusherPeriodConfigPath), true)
		testutil.NoError(t, a.Close())
		testutil.Equal(t, "reloader", cfg.HasObserver(config.ReloaderTriggerID, config.ReloadPeriodConfigPath), false)
		testutil.Equal(t, "flusher", cfg.HasObserver(log.FlusherTriggerID, log.FlusherPeriodConfigPath), false)
	})

	t.Run("should log to the configured streams, flushing on close", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "app.log")
		a := app.New(set(t, log.StreamConfigPath+".file", container.Bag{
			"driver": logfile.Driver, logfile.ConfigFieldPath: path, logfile.ConfigFieldChannels: []any{"*"},
		}))
		clk := testutil.NewClock(time.Date(2026, 11, 2, 9, 43, 0, 0, time.UTC))
		testutil.NoError(t, a.Decorate(func(flam.IClock) flam.IClock { return clk }))
		testutil.NoError(t, a.Run())
		testutil.NoError(t, a.Invoke(func(l flam.ILogger) {
			l.SignalInfo("db", "connected", container.Bag{"host": "x"})
			l.BroadcastDebug("hidden")
		}))
		testutil.NoError(t, a.Close())

		data, err := os.ReadFile(path)
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(data),
			`{"channel":"db","host":"x","level":"INFO","message":"connected","time":"2026-11-02T09:43:00.000+0000"}`+"\n")
	})

	t.Run("should add the configured sources, read from the local disk", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		testutil.NoError(t, os.WriteFile(path, []byte(`{"v": 1}`), 0o600))
		a := app.New(set(t, config.SourceConfigPath+".settings", container.Bag{
			"driver": file.Driver, file.ConfigFieldPath: path,
		}))
		testutil.NoError(t, a.Boot())
		testutil.Equal(t, "v", configOf(t, a).Int("v"), 1)
		testutil.NoError(t, a.Close())
	})

	t.Run("should reload the sources with the triggers of a decorated clock", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.json")
		testutil.NoError(t, os.WriteFile(path, []byte(`{"v": 1}`), 0o600))
		values := set(t, config.SourceConfigPath+".settings", container.Bag{
			"driver": file.Driver, file.ConfigFieldPath: path,
		})
		testutil.NoError(t, values.Set(config.ReloadPeriodConfigPath, "1s"))
		a := app.New(values)
		clk := testutil.NewClock(time.Now())
		testutil.NoError(t, a.Decorate(func(flam.IClock) flam.IClock { return clk }))
		testutil.NoError(t, a.Boot())

		cfg := configOf(t, a)
		changed := make(chan struct{}, 1)
		testutil.NoError(t, cfg.AddObserver("v", "v", func(_, _ any) { changed <- struct{}{} }))
		testutil.NoError(t, os.WriteFile(path, []byte(`{"v": 22}`), 0o600))
		clk.Advance(time.Second)
		select {
		case <-changed:
			testutil.Equal(t, "v", cfg.Int("v"), 22)
		case <-time.After(5 * time.Second):
			t.Fatal("the sources were not reloaded")
		}
		testutil.NoError(t, a.Close())
	})
}
