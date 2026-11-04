package disk

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/disk"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// config is a fake flam.IConfig reading its bags from a container.Bag; its
// other methods are not implemented.
type config struct {
	flam.IConfig

	bag container.Bag
}

func (c *config) Bag(path string, def ...flam.IBag) flam.IBag {
	return c.bag.Bag(path, def...)
}

// newApp returns a testutil.App providing a fake flam.IConfig holding the
// defaults of the configurable bundles merged with values, and with the
// bundles registered.
func newApp(t *testing.T, values container.Bag, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IConfig { return &config{bag: bag} }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	return app
}

// diskFactory returns the flam.IDiskFactory of the app container.
func diskFactory(t *testing.T, app *testutil.App) flam.IDiskFactory {
	t.Helper()
	var f flam.IDiskFactory
	testutil.NoError(t, app.Invoke(func(provided flam.IDiskFactory) { f = provided }))
	return f
}

// getDisk returns the disk of the id from the factory of the app container.
func getDisk(t *testing.T, app *testutil.App, id string) flam.IDisk {
	t.Helper()
	d, err := diskFactory(t, app).Get(id)
	testutil.NoError(t, err)
	return d
}

// ---------------------------------------------------------------------------
// declarations
// ---------------------------------------------------------------------------

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "DiskConfigPath", disk.DiskConfigPath, "flam.disk.disks")
	testutil.Equal(t, "DiskCreatorGroup", disk.DiskCreatorGroup, "flam.disk.disks.creator")
	testutil.Equal(t, "memory.Id", memory.Id, "memory")
	testutil.Equal(t, "memory.Driver", memory.Driver, "flam.disk.disks.driver.memory")
	testutil.Equal(t, "local.Id", local.Id, "local")
	testutil.Equal(t, "local.Driver", local.Driver, "flam.disk.disks.driver.local")
	testutil.Equal(t, "local.ConfigFieldRoot", local.ConfigFieldRoot, "root")
}

// ---------------------------------------------------------------------------
// disk bundle
// ---------------------------------------------------------------------------

func Test_DiskBundle(t *testing.T) {
	t.Run("should provide a single IDiskFactory", func(t *testing.T) {
		b := disk.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.disk")

		app := newApp(t, nil, b, memory.NewBundle())
		testutil.Equal(t, "calls", app.Calls(), []string{
			"provider/0", "bundle:flam.disk", "provider/0",
			"bundle:flam.disk.disks.memory", "provider/1",
		})

		first, second := diskFactory(t, app), diskFactory(t, app)
		if first == nil || first != second {
			t.Fatal("the container did not provide a single IDiskFactory")
		}
		testutil.Equal(t, "Available()", first.Available(), []string{"memory"})
	})

	t.Run("should be the IFactory of disks", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle())
		var f flam.IFactory[flam.IDisk]
		testutil.NoError(t, app.Invoke(func(provided flam.IFactory[flam.IDisk]) { f = provided }))
		testutil.Equal(t, "Available()", f.Available(), []string{})
	})

	t.Run("should fail without IConfig", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(disk.NewBundle()))
		if err := app.Invoke(func(flam.IDiskFactory) {}); err == nil {
			t.Fatal("Invoke() did not fail without IConfig")
		}
	})

	t.Run("should close the provided IDiskFactory", func(t *testing.T) {
		b := disk.NewBundle()
		closable, ok := b.(flam.IClosableBundle)
		if !ok {
			t.Fatal("the bundle does not implement IClosableBundle")
		}

		app := newApp(t, nil, b, memory.NewBundle())
		f := diskFactory(t, app)
		testutil.NoError(t, closable.Close(app))
		if _, err := f.Get(memory.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Get() after Close = %v, want factory.ErrClosed", err)
		}
		testutil.NoError(t, closable.Close(app))
	})

	t.Run("should create and close the IDiskFactory when never used", func(t *testing.T) {
		b := disk.NewBundle()
		app := newApp(t, nil, b, memory.NewBundle())
		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		if _, err := diskFactory(t, app).Get(memory.Id); !errors.Is(err, factory.ErrClosed) {
			t.Fatalf("Get() after Close = %v, want factory.ErrClosed", err)
		}
	})
}

// ---------------------------------------------------------------------------
// disks bundle
// ---------------------------------------------------------------------------

func Test_DisksBundle(t *testing.T) {
	t.Run("should register every driver, to use with the disk bundle", func(t *testing.T) {
		b := disks.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.disk.disks")

		bag := container.Bag{}
		testutil.NoError(t, memory.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.NoError(t, local.NewBundle().(flam.IConfigurableBundle).Config(bag))

		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IConfig { return &config{bag: bag} }))
		testutil.NoError(t, app.RegisterBundle(b))
		for _, id := range []string{"flam.disk.disks.memory", "flam.disk.disks.local", "flam.disk.disks.s3", "flam.disk.disks.gcs"} {
			if !app.HasBundle(id) {
				t.Errorf("bundle %q not registered", id)
			}
		}
		if app.HasBundle("flam.disk") {
			t.Fatal("the disks bundle registered the disk bundle")
		}
		if err := app.Invoke(func(flam.IDiskFactory) {}); err == nil {
			t.Fatal("the disks bundle provided an IDiskFactory")
		}

		testutil.NoError(t, app.RegisterBundle(disk.NewBundle()))
		f := diskFactory(t, app)
		testutil.Equal(t, "Available()", f.Available(), []string{"local", "memory"})
		testutil.NoError(t, f.GenerateAll())
	})
}

// ---------------------------------------------------------------------------
// drivers
// ---------------------------------------------------------------------------

func Test_DriverBundles(t *testing.T) {
	scenarios := []struct {
		name   string
		bundle func() flam.IBundle
		id     string
		diskID string
		driver string
	}{
		{"memory", memory.NewBundle, "flam.disk.disks.memory", memory.Id, memory.Driver},
		{"local", local.NewBundle, "flam.disk.disks.local", local.Id, local.Driver},
	}

	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			t.Run("should have its id", func(t *testing.T) {
				testutil.Equal(t, "Id()", s.bundle().Id(), s.id)
			})

			t.Run("should configure its default disk", func(t *testing.T) {
				bag := container.Bag{}
				testutil.NoError(t, s.bundle().(flam.IConfigurableBundle).Config(bag))
				testutil.Equal(t, "config", bag.Get(""), container.Bag{"flam": container.Bag{
					"disk": container.Bag{"disks": container.Bag{
						s.diskID: container.Bag{factory.ConfigFieldDriver: s.driver},
					}},
				}})
			})

			t.Run("should return the error of the bag", func(t *testing.T) {
				bag := container.Bag{"flam": "scalar"}
				if err := s.bundle().(flam.IConfigurableBundle).Config(bag); err == nil {
					t.Fatal("Config() did not fail on a conflicting bag")
				}
			})

			t.Run("should provide its creator in the creator group", func(t *testing.T) {
				app := testutil.NewApp()
				testutil.NoError(t, s.bundle().Register(app))
				testutil.Equal(t, "calls", app.Calls(), []string{"provider/1"})
			})

			t.Run("should create its disks only", func(t *testing.T) {
				app := newApp(t, container.Bag{
					disk.DiskConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: "other"},
				}, disk.NewBundle(), s.bundle())

				d := getDisk(t, app, s.diskID)
				testutil.Equal(t, "Id()", d.Id(), s.diskID)
				if _, err := diskFactory(t, app).Get("other"); !errors.Is(err, factory.ErrNoCreator) {
					t.Fatalf("Get(other) = %v, want factory.ErrNoCreator", err)
				}
			})

			t.Run("should create a disk of any id", func(t *testing.T) {
				app := newApp(t, container.Bag{
					disk.DiskConfigPath + ".custom": container.Bag{factory.ConfigFieldDriver: s.driver},
				}, disk.NewBundle(), s.bundle())

				testutil.Equal(t, "Id()", getDisk(t, app, "custom").Id(), "custom")
			})
		})
	}
}

func Test_MemoryDisk(t *testing.T) {
	app := newApp(t, nil, disk.NewBundle(), memory.NewBundle())
	f := diskFactory(t, app)

	t.Run("should read what was written", func(t *testing.T) {
		d := getDisk(t, app, memory.Id)
		testutil.Equal(t, "Name()", d.Name(), "MemMapFS")
		testutil.NoError(t, afero.WriteFile(d, "/dir/file.txt", []byte("hello"), 0o644))

		again := getDisk(t, app, memory.Id)
		content, err := afero.ReadFile(again, "/dir/file.txt")
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
	})

	t.Run("should generate new empty disks", func(t *testing.T) {
		testutil.NoError(t, afero.WriteFile(getDisk(t, app, memory.Id), "/file.txt", nil, 0o644))

		generated, err := f.Generate(memory.Id)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", generated.Id(), memory.Id)
		if _, err := generated.Stat("/file.txt"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat() on a generated disk = %v, want fs.ErrNotExist", err)
		}
	})
}

func Test_LocalDisk(t *testing.T) {
	t.Run("should reach the whole file system without root", func(t *testing.T) {
		dir := t.TempDir()
		app := newApp(t, nil, disk.NewBundle(), local.NewBundle())
		d := getDisk(t, app, local.Id)

		path := filepath.Join(dir, "file.txt")
		testutil.NoError(t, afero.WriteFile(d, path, []byte("hello"), 0o644))
		content, err := os.ReadFile(path)
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")
	})

	t.Run("should be restricted to its root", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "root")
		testutil.NoError(t, os.Mkdir(root, 0o755))
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".rooted": container.Bag{
				factory.ConfigFieldDriver: local.Driver,
				local.ConfigFieldRoot:     root,
			},
		}, disk.NewBundle(), local.NewBundle())
		d := getDisk(t, app, "rooted")

		testutil.NoError(t, afero.WriteFile(d, "/file.txt", []byte("hello"), 0o644))
		content, err := os.ReadFile(filepath.Join(root, "file.txt"))
		testutil.NoError(t, err)
		testutil.Equal(t, "content", string(content), "hello")

		testutil.NoError(t, os.WriteFile(filepath.Join(dir, "outside.txt"), nil, 0o644))
		if _, err := d.Stat("../outside.txt"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat() outside the root = %v, want os.ErrNotExist", err)
		}
	})
}

// ---------------------------------------------------------------------------
// disk bundle: provider and Close
// ---------------------------------------------------------------------------

// closerDisk is an in-memory flam.IDisk implementing io.Closer, failing its
// Close with closeErr.
type closerDisk struct {
	afero.Fs

	id       string
	closed   int
	closeErr error
}

func (d *closerDisk) Id() string { return d.id }

func (d *closerDisk) Close() error {
	d.closed++
	return d.closeErr
}

func Test_DiskBundle_Provider(t *testing.T) {
	t.Run("should fail the provider when a creator of the group is nil", func(t *testing.T) {
		b := disk.NewBundle()
		app := newApp(t, nil, b)
		testutil.NoError(t, app.Provide(func() flam.IDiskCreator { return nil }, dig.Group(disk.DiskCreatorGroup)))

		err := app.Invoke(func(flam.IDiskFactory) {})
		if !errors.Is(err, factory.ErrNilCreator) {
			t.Fatalf("Invoke() = %v, want factory.ErrNilCreator", err)
		}
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("Invoke() = %T, want a wrapped *flam.Error", err)
		}
		testutil.Equal(t, "Error()", e.Error(), "unable to create factory: nil factory creator")
		testutil.Context(t, e, map[string]any{})

		err = b.(flam.IClosableBundle).Close(app) // creates the factory to close it
		if !errors.Is(err, factory.ErrNilCreator) {
			t.Fatalf("Close() = %v, want factory.ErrNilCreator", err)
		}
	})

	t.Run("should fail without IConfig", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(disk.NewBundle()))
		err := app.Invoke(func(flam.IDiskFactory) {})
		if err == nil || !strings.Contains(err.Error(), "flam.IConfig") {
			t.Fatalf("Invoke() = %v, want a missing flam.IConfig error", err)
		}
	})

	t.Run("should return the error of the application on Register", func(t *testing.T) {
		cause := errors.New("boom")
		for name, b := range map[string]flam.IBundle{
			"disk": disk.NewBundle(), "disks": disks.NewBundle(), "memory": memory.NewBundle(), "local": local.NewBundle(),
		} {
			app := testutil.NewApp()
			app.Fail = func(string) error { return cause }
			if err := b.Register(app); !errors.Is(err, cause) {
				t.Errorf("%s Register() = %v, want it to wrap the application error", name, err)
			}
		}
	})

	t.Run("should collect the creators of the group, from every driver", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle(), memory.NewBundle(), local.NewBundle())
		var creators []flam.IDiskCreator
		testutil.NoError(t, app.Invoke(func(args struct {
			dig.In
			Creators []flam.IDiskCreator `group:"flam.disk.disks.creator"`
		}) {
			creators = args.Creators
		}))
		testutil.Equal(t, "creators", len(creators), 2)

		f := diskFactory(t, app)
		testutil.Equal(t, "Available()", f.Available(), []string{"local", "memory"})
		testutil.NoError(t, f.GenerateAll())
		testutil.Equal(t, "Stored()", f.Stored(), []string{"local", "memory"})
	})

	t.Run("should use a custom creator provided in the group", func(t *testing.T) {
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".custom": container.Bag{factory.ConfigFieldDriver: "custom"},
		}, disk.NewBundle())
		testutil.NoError(t, app.Provide(func() flam.IDiskCreator { return &customCreator{} }, dig.Group(disk.DiskCreatorGroup)))

		d := getDisk(t, app, "custom")
		if _, ok := d.(*closerDisk); !ok {
			t.Fatalf("Get() = %T, want the custom disk", d)
		}
	})

	t.Run("should read the configuration on first use only", func(t *testing.T) {
		bag := container.Bag{}
		testutil.NoError(t, memory.NewBundle().(flam.IConfigurableBundle).Config(bag))
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IConfig { return &config{bag: bag} }))
		testutil.NoError(t, app.RegisterBundle(disk.NewBundle()))
		testutil.NoError(t, app.RegisterBundle(memory.NewBundle()))

		testutil.NoError(t, bag.Set(disk.DiskConfigPath+".before", container.Bag{factory.ConfigFieldDriver: memory.Driver}))
		f := diskFactory(t, app)
		testutil.NoError(t, bag.Set(disk.DiskConfigPath+".after", container.Bag{factory.ConfigFieldDriver: memory.Driver}))
		testutil.NoError(t, bag.Set(disk.DiskConfigPath+".before."+factory.ConfigFieldDriver, "other"))

		testutil.Equal(t, "Available()", f.Available(), []string{"before", "memory"})
		testutil.Equal(t, "Available() again", diskFactory(t, app).Available(), []string{"before", "memory"})
		_, err := f.Get("before")
		testutil.NoError(t, err)
	})

	t.Run("should provide a factory without disks when unconfigured", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle())
		f := diskFactory(t, app)
		testutil.Equal(t, "Available()", f.Available(), []string{})
		_, err := f.Get(memory.Id)
		testutil.Error(t, err, factory.ErrUnknownID,
			"unable to generate resource: unknown factory resource id", map[string]any{"id": memory.Id})
	})
}

// customCreator creates a closerDisk for the "custom" driver.
type customCreator struct{}

func (c *customCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "custom"
}

func (c *customCreator) Create(bag flam.IBag) (flam.IDisk, error) {
	return &closerDisk{Fs: afero.NewMemMapFs(), id: bag.String(factory.ConfigFieldId)}, nil
}

func Test_DiskBundle_Close(t *testing.T) {
	t.Run("should close the disks of the factory", func(t *testing.T) {
		b := disk.NewBundle()
		app := newApp(t, nil, b)
		d := &closerDisk{Fs: afero.NewMemMapFs(), id: "a"}
		testutil.NoError(t, diskFactory(t, app).Store("a", d))

		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		testutil.Equal(t, "closed", d.closed, 1)
		testutil.Equal(t, "Stored()", diskFactory(t, app).Stored(), []string(nil))
	})

	t.Run("should return the joined Close errors of the disks", func(t *testing.T) {
		cause := errors.New("boom")
		b := disk.NewBundle()
		app := newApp(t, nil, b)
		f := diskFactory(t, app)
		testutil.NoError(t, f.Store("a", &closerDisk{Fs: afero.NewMemMapFs(), id: "a", closeErr: cause}))
		testutil.NoError(t, f.Store("b", &closerDisk{Fs: afero.NewMemMapFs(), id: "b"}))

		err := b.(flam.IClosableBundle).Close(app)
		joined, ok := err.(interface{ Unwrap() []error })
		if !ok {
			t.Fatalf("Close() = %T %v, want an errors.Join result", err, err)
		}
		errs := joined.Unwrap()
		testutil.Equal(t, "errors", len(errs), 1)
		testutil.Error(t, errs[0], cause, "unable to remove resource: boom", map[string]any{"id": "a"})

		testutil.NoError(t, b.(flam.IClosableBundle).Close(app)) // closed once
	})

	t.Run("should close the factory of the given application only", func(t *testing.T) {
		b := disk.NewBundle()
		app1 := newApp(t, nil, b, memory.NewBundle())
		app2 := newApp(t, nil, disk.NewBundle(), memory.NewBundle())
		f1, f2 := diskFactory(t, app1), diskFactory(t, app2)

		testutil.NoError(t, b.(flam.IClosableBundle).Close(app1))
		testutil.Equal(t, "Available() of the closed one", f1.Available(), []string(nil))
		testutil.Equal(t, "Available() of the other one", f2.Available(), []string{"memory"})
	})
}

// ---------------------------------------------------------------------------
// disks bundle: registration
// ---------------------------------------------------------------------------

func Test_DisksBundle_Registration(t *testing.T) {
	t.Run("should register the memory, local, s3 and gcs drivers", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(disks.NewBundle()))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.disk.disks",
			"bundle:flam.disk.disks.memory", "provider/1",
			"bundle:flam.disk.disks.local", "provider/1",
			"bundle:flam.disk.disks.s3", "provider/1",
			"bundle:flam.disk.disks.gcs", "provider/1",
		})
	})

	t.Run("should neither configure nor close by itself", func(t *testing.T) {
		b := disks.NewBundle()
		if _, ok := b.(flam.IConfigurableBundle); ok {
			t.Error("the disks bundle implements IConfigurableBundle")
		}
		if _, ok := b.(flam.IClosableBundle); ok {
			t.Error("the disks bundle implements IClosableBundle")
		}
	})
}

// ---------------------------------------------------------------------------
// drivers: configuration
// ---------------------------------------------------------------------------

func Test_DriverBundles_Config(t *testing.T) {
	for name, s := range map[string]struct {
		bundle func() flam.IBundle
		diskID string
		driver string
	}{
		"memory": {memory.NewBundle, memory.Id, memory.Driver},
		"local":  {local.NewBundle, local.Id, local.Driver},
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("should keep the other disk configurations", func(t *testing.T) {
				bag := container.Bag{}
				other := container.Bag{factory.ConfigFieldDriver: "other"}
				testutil.NoError(t, bag.Set(disk.DiskConfigPath+".other", other))
				testutil.NoError(t, s.bundle().(flam.IConfigurableBundle).Config(bag))
				testutil.Equal(t, "disks", bag.Bag(disk.DiskConfigPath), flam.IBag(container.Bag{
					"other":  container.Bag{factory.ConfigFieldDriver: "other"},
					s.diskID: container.Bag{factory.ConfigFieldDriver: s.driver},
				}))
			})

			t.Run("should return the very error of the bag", func(t *testing.T) {
				want := container.Bag{"flam": "scalar"}.Set(disk.DiskConfigPath+"."+s.diskID, container.Bag{})
				if want == nil {
					t.Fatal("the conflicting bag accepted the path")
				}
				got := s.bundle().(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"})
				if got == nil || got.Error() != want.Error() {
					t.Fatalf("Config() = %v, want %v", got, want)
				}
			})

			t.Run("should not be closable", func(t *testing.T) {
				if _, ok := s.bundle().(flam.IClosableBundle); ok {
					t.Error("the driver bundle implements IClosableBundle")
				}
			})
		})
	}
}

// ---------------------------------------------------------------------------
// drivers: disks
// ---------------------------------------------------------------------------

func Test_MemoryDisk_Isolation(t *testing.T) {
	t.Run("should generate disks sharing no content with each other", func(t *testing.T) {
		f := diskFactory(t, newApp(t, nil, disk.NewBundle(), memory.NewBundle()))
		first, err := f.Generate(memory.Id)
		testutil.NoError(t, err)
		second, err := f.Generate(memory.Id)
		testutil.NoError(t, err)

		testutil.NoError(t, afero.WriteFile(first, "/file.txt", nil, 0o644))
		if _, err := second.Stat("/file.txt"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat() on another generated disk = %v, want fs.ErrNotExist", err)
		}
		testutil.Equal(t, "Stored()", f.Stored(), []string{})
	})

	t.Run("should create distinct disks for distinct ids", func(t *testing.T) {
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: memory.Driver},
		}, disk.NewBundle(), memory.NewBundle())
		testutil.NoError(t, afero.WriteFile(getDisk(t, app, memory.Id), "/file.txt", nil, 0o644))
		other := getDisk(t, app, "other")
		testutil.Equal(t, "Name()", other.Name(), "MemMapFS")
		if _, err := other.Stat("/file.txt"); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("Stat() on another disk = %v, want fs.ErrNotExist", err)
		}
	})

	t.Run("should be removable without failing, not being an io.Closer", func(t *testing.T) {
		app := newApp(t, nil, disk.NewBundle(), memory.NewBundle())
		getDisk(t, app, memory.Id)
		testutil.NoError(t, diskFactory(t, app).Remove(memory.Id))
	})
}

func Test_LocalDisk_Root(t *testing.T) {
	t.Run("should be the whole file system, as an OsFs, without or with an empty root", func(t *testing.T) {
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".empty": container.Bag{factory.ConfigFieldDriver: local.Driver, local.ConfigFieldRoot: ""},
		}, disk.NewBundle(), local.NewBundle())
		dir := t.TempDir()

		for _, id := range []string{local.Id, "empty"} {
			d := getDisk(t, app, id)
			testutil.Equal(t, "Name()", d.Name(), "OsFs")
			path := filepath.Join(dir, id+".txt")
			testutil.NoError(t, afero.WriteFile(d, path, []byte(id), 0o644))
			content, err := os.ReadFile(path)
			testutil.NoError(t, err)
			testutil.Equal(t, "content", string(content), id)
		}
	})

	t.Run("should be a BasePathFs with relative paths under a root", func(t *testing.T) {
		root := t.TempDir()
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".rooted": container.Bag{factory.ConfigFieldDriver: local.Driver, local.ConfigFieldRoot: root},
		}, disk.NewBundle(), local.NewBundle())
		d := getDisk(t, app, "rooted")
		testutil.Equal(t, "Name()", d.Name(), "BasePathFs")

		testutil.NoError(t, os.WriteFile(filepath.Join(root, "file.txt"), []byte("hello"), 0o644))
		for _, path := range []string{"file.txt", "/file.txt", "dir/../file.txt"} {
			content, err := afero.ReadFile(d, path)
			testutil.NoError(t, err)
			testutil.Equal(t, "content of "+path, string(content), "hello")
		}
	})

	t.Run("should fail with os.ErrNotExist on paths escaping the root", func(t *testing.T) {
		dir := t.TempDir()
		root := filepath.Join(dir, "root")
		testutil.NoError(t, os.Mkdir(root, 0o755))
		testutil.NoError(t, os.WriteFile(filepath.Join(dir, "outside.txt"), nil, 0o644))
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".rooted": container.Bag{factory.ConfigFieldDriver: local.Driver, local.ConfigFieldRoot: root},
		}, disk.NewBundle(), local.NewBundle())
		d := getDisk(t, app, "rooted")

		for _, path := range []string{"../outside.txt", "../../outside.txt", filepath.Join(dir, "outside.txt")} {
			if _, err := d.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("Stat(%q) outside the root = %v, want os.ErrNotExist", path, err)
			}
		}
		if err := afero.WriteFile(d, "../escaped.txt", nil, 0o644); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("WriteFile() outside the root = %v, want os.ErrNotExist", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "escaped.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("a file was written outside the root: %v", err)
		}
	})

	t.Run("should give each configured disk its own root", func(t *testing.T) {
		rootA, rootB := t.TempDir(), t.TempDir()
		app := newApp(t, container.Bag{
			disk.DiskConfigPath + ".a": container.Bag{factory.ConfigFieldDriver: local.Driver, local.ConfigFieldRoot: rootA},
			disk.DiskConfigPath + ".b": container.Bag{factory.ConfigFieldDriver: local.Driver, local.ConfigFieldRoot: rootB},
		}, disk.NewBundle(), local.NewBundle())

		testutil.NoError(t, afero.WriteFile(getDisk(t, app, "a"), "/file.txt", []byte("a"), 0o644))
		testutil.NoError(t, afero.WriteFile(getDisk(t, app, "b"), "/file.txt", []byte("b"), 0o644))
		for root, want := range map[string]string{rootA: "a", rootB: "b"} {
			content, err := os.ReadFile(filepath.Join(root, "file.txt"))
			testutil.NoError(t, err)
			testutil.Equal(t, "content", string(content), want)
		}
	})
}
