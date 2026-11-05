package funit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/pkg/funit"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// fakeT is a testing.TB recording the failures and the logs of a runner,
// so that the failures a runner reports do not fail the test running it.
type fakeT struct {
	testing.TB

	mu     sync.Mutex
	errors []string
	logs   []string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func (f *fakeT) Logf(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}

// failures returns the failures reported.
func (f *fakeT) failures() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.errors...)
}

// newApp returns an application of the values, with the bundles, closed
// once the test ends.
func newApp(t *testing.T, values container.Bag, bundles ...flam.IBundle) flam.IApp {
	t.Helper()
	a := app.New(values)
	for _, b := range bundles {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

// newRunner returns the runner of the application reporting to a fake
// test.
func newRunner(t *testing.T, a flam.IApp) (*funit.Runner, *fakeT) {
	t.Helper()
	ft := &fakeT{}
	r, err := funit.NewRunner(a, ft)
	testutil.NoError(t, err)
	return r, ft
}

// runError fails the test unless err wraps the sentinel, and the fake test
// was failed with it.
func runError(t *testing.T, err error, ft *fakeT, sentinel error) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("Run() = %v, want %v", err, sentinel)
	}
	if failures := ft.failures(); len(failures) != 1 || failures[0] != "funit: "+err.Error() {
		t.Errorf("failures = %q, want the error of the run", failures)
	}
}

// configValue returns the value of the configuration of the application at
// the path.
func configValue(t *testing.T, a flam.IApp, path string) any {
	t.Helper()
	var v any
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) { v = c.Get(path) }))
	return v
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "DefaultStartTimeout", funit.DefaultStartTimeout, 10*time.Second)
	testutil.Equal(t, "DefaultStopTimeout", funit.DefaultStopTimeout, 10*time.Second)
	testutil.Equal(t, "DefaultPublishedTimeout", funit.DefaultPublishedTimeout, time.Second)
	testutil.Equal(t, "DefaultConfigParser", funit.DefaultConfigParser, "yaml")
	for name, tc := range map[string][2]string{
		"ErrorFieldStage":   {funit.ErrorFieldStage, "stage"},
		"ErrorFieldFile":    {funit.ErrorFieldFile, "file"},
		"ErrorFieldParser":  {funit.ErrorFieldParser, "parser"},
		"ErrorFieldProcess": {funit.ErrorFieldProcess, "process"},
		"ErrorFieldTimeout": {funit.ErrorFieldTimeout, "timeout"},
		"StageConfig":       {funit.StageConfig, "config"},
		"StageBoot":         {funit.StageBoot, "boot"},
		"StageArrange":      {funit.StageArrange, "arrange"},
		"StageRun":          {funit.StageRun, "run"},
		"StageAct":          {funit.StageAct, "act"},
		"StageAssert":       {funit.StageAssert, "assert"},
		"StageStop":         {funit.StageStop, "stop"},
		"StageTeardown":     {funit.StageTeardown, "teardown"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrNilApp": funit.ErrNilApp, "ErrNilTest": funit.ErrNilTest, "ErrInvalidStage": funit.ErrInvalidStage,
		"ErrRan": funit.ErrRan, "ErrConfig": funit.ErrConfig, "ErrProcess": funit.ErrProcess,
		"ErrBoot": funit.ErrBoot, "ErrStage": funit.ErrStage, "ErrRun": funit.ErrRun,
		"ErrStartTimeout": funit.ErrStartTimeout, "ErrStopTimeout": funit.ErrStopTimeout,
		"ErrNotRunning": funit.ErrNotRunning, "ErrRollback": funit.ErrRollback,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_NewRunner(t *testing.T) {
	a := newApp(t, nil)
	var nilT *testing.T
	for name, tc := range map[string]struct {
		app      flam.IApp
		t        testing.TB
		sentinel error
	}{
		"nil app":  {nil, t, funit.ErrNilApp},
		"nil test": {a, nil, funit.ErrNilTest},
		"nil *T":   {a, nilT, funit.ErrNilTest},
	} {
		r, err := funit.NewRunner(tc.app, tc.t)
		testutil.Equal(t, name+" runner", r, (*funit.Runner)(nil))
		if !errors.Is(err, tc.sentinel) {
			t.Errorf("%s: NewRunner() = %v, want %v", name, err, tc.sentinel)
		}
	}

	r, _ := newRunner(t, a)
	testutil.Equal(t, "PubSub()", r.PubSub(), nil)
	testutil.Equal(t, "Tx()", r.Tx() == nil, true)
	testutil.Equal(t, "Connection()", r.Connection(), nil)
	testutil.Equal(t, "Redis()", r.Redis(), nil)
	testutil.Equal(t, "Published()", r.Published(), []funit.Published{})
}

func Test_Runner_Builders(t *testing.T) {
	var nilFunc func()
	for name, build := range map[string]func(r *funit.Runner){
		"empty arrange id":   func(r *funit.Runner) { r.WithArrange("", func() {}) },
		"arrange non func":   func(r *funit.Runner) { r.WithArrange("a", 1) },
		"nil act":            func(r *funit.Runner) { r.WithAct(nil) },
		"nil func assert":    func(r *funit.Runner) { r.WithAssert("a", nilFunc) },
		"empty teardown id":  func(r *funit.Runner) { r.WithTeardown("", func() {}) },
		"first error stands": func(r *funit.Runner) { r.WithAct(1).WithAct(2) },
	} {
		t.Run("should refuse a "+name, func(t *testing.T) {
			r, ft := newRunner(t, newApp(t, nil))
			build(r)
			err := r.Run()
			runError(t, err, ft, funit.ErrInvalidStage)
		})
	}

	t.Run("should ignore the timeouts of 0 or less and a nil logger", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		r.WithStartTimeout(0).WithStopTimeout(-1).WithPublishedTimeout(0).WithLogger(nil)
		testutil.NoError(t, r.Run())
		testutil.Equal(t, "failures", len(ft.failures()), 0)
	})

	t.Run("should refuse to run twice", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		testutil.NoError(t, r.Run())
		runError(t, r.Run(), ft, funit.ErrRan)
	})
}

func Test_Runner_Stages(t *testing.T) {
	t.Run("should run the stages in order, ending the run", func(t *testing.T) {
		a := newApp(t, nil)
		r, ft := newRunner(t, a)
		var calls []string
		call := func(name string) func(flam.IConfig) { return func(flam.IConfig) { calls = append(calls, name) } }
		err := r.
			WithLogger(funit.NewLogger(ft)).
			WithArrange("a1", call("a1")).
			WithArrange("a2", call("a2")).
			WithAct(call("act")).
			WithAssert("s1", call("s1")).
			WithTeardown("t1", func() error { calls = append(calls, "t1"); return errBoom }).
			WithTeardown("t2", call("t2")).
			Run()
		if !errors.Is(err, funit.ErrStage) || !errors.Is(err, errBoom) {
			t.Fatalf("Run() = %v, want the error of the teardown", err)
		}
		testutil.Equal(t, "calls", calls, []string{"a1", "a2", "act", "s1", "t1", "t2"})
		failures := ft.failures()
		if len(failures) != 1 || !strings.Contains(failures[0], "boom") {
			t.Errorf("failures = %q, want the teardown error", failures)
		}
		for _, want := range []string{"funit: arrange: a1", "funit: teardown: t1 failed: "} {
			found := false
			for _, l := range ft.logs {
				found = found || strings.HasPrefix(l, want)
			}
			if !found {
				t.Errorf("logs = %q, want %q", ft.logs, want)
			}
		}
	})

	for name, tc := range map[string]struct {
		build func(r *funit.Runner, calls *[]string)
		want  []string
	}{
		"arrange": {func(r *funit.Runner, calls *[]string) {
			r.WithArrange("a", func() error { return errBoom }).
				WithAct(func() { *calls = append(*calls, "act") })
		}, []string{"teardown"}},
		"act": {func(r *funit.Runner, calls *[]string) {
			r.WithAct(func() error { return errBoom }).
				WithAssert("s", func() { *calls = append(*calls, "assert") })
		}, []string{"teardown"}},
		"assert": {func(r *funit.Runner, calls *[]string) {
			r.WithAssert("s1", func() error { return errBoom }).
				WithAssert("s2", func() { *calls = append(*calls, "s2") })
		}, []string{"teardown"}},
	} {
		t.Run("should stop at a failing "+name+", tearing down", func(t *testing.T) {
			r, ft := newRunner(t, newApp(t, nil))
			var calls []string
			tc.build(r, &calls)
			r.WithLogger(funit.NewLogger(ft)).WithTeardown("t", func() { calls = append(calls, "teardown") })
			err := r.Run()
			runError(t, err, ft, funit.ErrStage)
			if !errors.Is(err, errBoom) {
				t.Errorf("Run() = %v, want errBoom", err)
			}
			testutil.Equal(t, "calls", calls, tc.want)
		})
	}

	t.Run("should tear down a panicking act", func(t *testing.T) {
		r, _ := newRunner(t, newApp(t, nil))
		tornDown := false
		r.WithAct(func() { panic("act") }).WithTeardown("t", func() { tornDown = true })
		testutil.Panics(t, "Run()", func() { _ = r.Run() })
		testutil.Equal(t, "torn down", tornDown, true)
	})

	t.Run("should fail with a boot failure", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil, failingBundle{boot: errBoom}))
		runError(t, r.WithLogger(funit.NewLogger(ft)).Run(), ft, funit.ErrBoot)
	})

	t.Run("should fail with a run failure without kennel", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil, failingBundle{run: errBoom}))
		runError(t, r.Run(), ft, funit.ErrRun)
	})
}

// failingBundle is a bundle failing its boot or its run.
type failingBundle struct{ boot, run error }

func (failingBundle) Id() string               { return "failing" }
func (failingBundle) Register(flam.IApp) error { return nil }
func (b failingBundle) Boot(flam.IApp) error   { return b.boot }
func (b failingBundle) Run(flam.IApp) error    { return b.run }

func Test_Runner_Config(t *testing.T) {
	dir := t.TempDir()
	yamlFile := filepath.Join(dir, "config.yaml")
	jsonFile := filepath.Join(dir, "config.json")
	testutil.NoError(t, os.WriteFile(yamlFile, []byte("app:\n  name: yaml\n  yaml: true\n"), 0o600))
	testutil.NoError(t, os.WriteFile(jsonFile, []byte(`{"app": {"name": "json", "json": true}}`), 0o600))
	badFile := filepath.Join(dir, "bad.yaml")
	testutil.NoError(t, os.WriteFile(badFile, []byte("app: [\n"), 0o600))

	t.Run("should add the configuration for the time of the run", func(t *testing.T) {
		a := newApp(t, nil)
		r, _ := newRunner(t, a)
		var seen map[string]any
		testutil.NoError(t, r.
			WithConfig(yamlFile, 10).
			WithConfig(jsonFile, 20, "json").
			WithConfigValues(container.Bag{"app": container.Bag{"name": "values"}}, 30).
			WithAct(func(c flam.IConfig) {
				seen = map[string]any{
					"name": c.Get("app.name"), "yaml": c.Get("app.yaml"), "json": c.Get("app.json"),
				}
			}).
			Run())
		testutil.Equal(t, "seen", seen, map[string]any{"name": "values", "yaml": true, "json": true})
		testutil.Equal(t, "removed", configValue(t, a, "app.name"), nil)
	})

	for name, tc := range map[string]struct {
		file, parser string
	}{
		"missing file":   {filepath.Join(dir, "none.yaml"), "yaml"},
		"unknown parser": {yamlFile, "toml"},
		"invalid file":   {badFile, "yaml"},
		"unparsable":     {yamlFile, "json"},
	} {
		t.Run("should fail with a "+name, func(t *testing.T) {
			r, ft := newRunner(t, newApp(t, nil))
			err := r.WithLogger(funit.NewLogger(ft)).WithConfig(tc.file, 0, tc.parser).Run()
			runError(t, err, ft, funit.ErrConfig)
			var e *flam.Error
			errors.As(err, &e)
			testutil.Equal(t, "file", e.Context()[funit.ErrorFieldFile], tc.file)
		})
	}

	t.Run("should read a file with the default parser for an empty id", func(t *testing.T) {
		a := newApp(t, nil)
		r, _ := newRunner(t, a)
		parsers := make([]string, 1, 2)
		var name any
		testutil.NoError(t, r.
			WithConfig(yamlFile, 10, parsers...).
			WithAct(func(c flam.IConfig) { name = c.Get("app.name") }).
			Run())
		testutil.Equal(t, "name", name, "yaml")
		testutil.Equal(t, "parsers", parsers[:cap(parsers)], []string{"", ""})
	})

	t.Run("should fail without configuration", func(t *testing.T) {
		r, ft := newRunner(t, testutil.NewApp())
		runError(t, r.WithConfigValues(container.Bag{"a": 1}, 0).Run(), ft, funit.ErrConfig)
	})
}

// process is a watchdog process running until closed, telling it runs
// once started unless silent, ignoring its close when stubborn until
// released, or failing with err.
type process struct {
	id       string
	silent   bool
	stubborn bool
	err      error

	mu      sync.Mutex
	running bool
	closed  chan struct{}
	release chan struct{}
	once    sync.Once
}

// newProcess returns the process of the id.
func newProcess(id string) *process {
	return &process{id: id, closed: make(chan struct{}), release: make(chan struct{})}
}

func (p *process) Id() string { return p.id }

func (p *process) Run(context.Context) error {
	if p.err != nil {
		return p.err
	}
	p.mu.Lock()
	p.running = !p.silent
	p.mu.Unlock()
	<-p.closed
	if p.stubborn {
		<-p.release
	}
	return nil
}

func (p *process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *process) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

// processApp returns an application with the watchdog bundle and the
// processes, inactive unless active, run by a fake test.
func processApp(t *testing.T, active bool, processes ...flam.IWatchdogProcess) flam.IApp {
	t.Helper()
	values := container.Bag{}
	for _, p := range processes {
		testutil.NoError(t, values.Set(watchdog.ProcessConfigPath+"."+p.Id()+"."+watchdog.ConfigFieldActive, active))
	}
	testutil.NoError(t, values.Set(watchdog.SignalsConfigPath, false))
	a := newApp(t, values, watchdog.NewBundle())
	for _, p := range processes {
		testutil.NoError(t, a.Provide(func() flam.IWatchdogProcess { return p }, dig.Group(watchdog.ProcessGroup)))
	}
	return a
}

func Test_Runner_Processes(t *testing.T) {
	t.Run("should run the processes of the test during the act", func(t *testing.T) {
		api, jobs := newProcess("api"), newProcess("jobs")
		a := processApp(t, false, api, jobs)
		r, ft := newRunner(t, a)
		var running [2]bool
		testutil.NoError(t, r.WithLogger(funit.NewLogger(ft)).WithProcess("api").WithAct(func() {
			running = [2]bool{api.IsRunning(), jobs.IsRunning()}
		}).Run())
		testutil.Equal(t, "running", running, [2]bool{true, false})
		testutil.Equal(t, "failures", len(ft.failures()), 0)
	})

	t.Run("should wait for the active processes of the application", func(t *testing.T) {
		api := newProcess("api")
		r, _ := newRunner(t, processApp(t, true, api))
		running := false
		testutil.NoError(t, r.WithAct(func() { running = api.IsRunning() }).Run())
		testutil.Equal(t, "running", running, true)
	})

	t.Run("should run without active process", func(t *testing.T) {
		r, _ := newRunner(t, processApp(t, false, newProcess("api")))
		testutil.NoError(t, r.Run())
	})

	t.Run("should fail with an unknown process", func(t *testing.T) {
		r, ft := newRunner(t, processApp(t, false, newProcess("api")))
		runError(t, r.WithProcess("none").Run(), ft, funit.ErrProcess)
	})

	t.Run("should fail with a process without kennel", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		runError(t, r.WithProcess("api").Run(), ft, funit.ErrProcess)
	})

	t.Run("should fail with a failing process", func(t *testing.T) {
		api := newProcess("api")
		api.err = errBoom
		r, ft := newRunner(t, processApp(t, false, api))
		err := r.WithLogger(funit.NewLogger(ft)).WithProcess("api").Run()
		runError(t, err, ft, funit.ErrRun)
	})

	t.Run("should fail when the processes do not start in time", func(t *testing.T) {
		api := newProcess("api")
		api.silent = true
		r, ft := newRunner(t, processApp(t, false, api))
		err := r.WithProcess("api").WithStartTimeout(20 * time.Millisecond).Run()
		runError(t, err, ft, funit.ErrStartTimeout)
	})

	t.Run("should fail when the application does not stop in time", func(t *testing.T) {
		api := newProcess("api")
		api.stubborn = true
		defer close(api.release)
		r, ft := newRunner(t, processApp(t, false, api))
		tornDown := false
		err := r.
			WithProcess("api").
			WithStopTimeout(20*time.Millisecond).
			WithTeardown("t", func() { tornDown = true }).
			Run()
		runError(t, err, ft, funit.ErrStopTimeout)
		testutil.Equal(t, "torn down while running", tornDown, false)
	})

	t.Run("should fail when the processes end before running", func(t *testing.T) {
		api := newProcess("api")
		api.silent = true
		_ = api.Close() // its run ends at once
		r, ft := newRunner(t, processApp(t, false, api))
		runError(t, r.WithProcess("api").Run(), ft, funit.ErrNotRunning)
	})

	t.Run("should close the processes once, through the kennel", func(t *testing.T) {
		api := &closeCounting{process: newProcess("api")}
		a := processApp(t, false, api)
		r, _ := newRunner(t, a)
		testutil.NoError(t, r.WithProcess("api").Run())
		testutil.NoError(t, a.Close())
		testutil.Equal(t, "closes", api.closes.Load(), int32(1))
	})

	for name, active := range map[string]bool{"inactive": false, "active": true} {
		t.Run("should leave an "+name+" process "+name, func(t *testing.T) {
			a := processApp(t, active, newProcess("api"))
			r, _ := newRunner(t, a)
			testutil.NoError(t, r.WithProcess("api").Run())
			var got bool
			testutil.NoError(t, a.Invoke(func(k flam.IKennel) { got = k.IsActive("api") }))
			testutil.Equal(t, "active", got, active)
		})
	}

	t.Run("should deactivate the processes when the arrange fails", func(t *testing.T) {
		a := processApp(t, false, newProcess("api"))
		r, ft := newRunner(t, a)
		err := r.WithProcess("api").WithArrange("a", func() error { return errBoom }).Run()
		runError(t, err, ft, funit.ErrStage)
		var got bool
		testutil.NoError(t, a.Invoke(func(k flam.IKennel) { got = k.IsActive("api") }))
		testutil.Equal(t, "active", got, false)
	})

	t.Run("should fail when the run of the application fails once stopped", func(t *testing.T) {
		api, failing := newProcess("api"), &closeFailing{process: newProcess("worker")}
		r, ft := newRunner(t, processApp(t, false, api, failing))
		err := r.WithProcess("api").WithProcess("worker").Run()
		if !errors.Is(err, errBoom) {
			t.Fatalf("Run() = %v, want errBoom", err)
		}
		testutil.Equal(t, "failures", len(ft.failures()), 1)
	})
}

// closeCounting is a process counting its closes.
type closeCounting struct {
	*process
	closes atomic.Int32
}

func (p *closeCounting) Close() error {
	p.closes.Add(1)
	return p.process.Close()
}

// closeFailing is a process failing to close, and returning an error once
// closed.
type closeFailing struct{ *process }

func (p *closeFailing) Run(ctx context.Context) error {
	_ = p.process.Run(ctx)
	return errBoom
}

func (p *closeFailing) Close() error {
	_ = p.process.Close()
	return errBoom
}

func Test_Runner_Signals(t *testing.T) {
	t.Run("should record the messages of the topics", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		testutil.NoError(t, r.
			WithArrangeSignals("users.*", "audit").
			WithAct(func(ps flam.IPubSub[string, string]) error {
				testutil.Equal(t, "PubSub()", r.PubSub(), ps)
				for _, topic := range []string{"users.created", "other", "audit"} {
					if err := ps.Publish(context.Background(), topic, topic+"!"); err != nil {
						return err
					}
				}
				return nil
			}).
			WithAssertPublished("audit", "users.created").
			Run())
		testutil.Equal(t, "failures", len(ft.failures()), 0)
		published := r.Published()
		if len(published) != 2 {
			t.Fatalf("Published() = %v, want 2 messages", published)
		}
		messages := map[string]string{published[0].Topic: published[0].Message, published[1].Topic: published[1].Message}
		testutil.Equal(t, "messages", messages, map[string]string{"users.created": "users.created!", "audit": "audit!"})
	})

	t.Run("should report the unexpected messages", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		testutil.NoError(t, r.
			WithPublishedTimeout(20*time.Millisecond).
			WithArrangeSignals("a").
			WithAct(func(ps flam.IPubSub[string, string]) error {
				return ps.Publish(context.Background(), "a", "x")
			}).
			WithAssertPublished("a", "a").
			Run())
		failures := ft.failures()
		if len(failures) != 1 || !strings.Contains(failures[0], `published on ["a"], want ["a" "a"]`) {
			t.Errorf("failures = %q, want the missing message", failures)
		}
	})

	t.Run("should subscribe to a topic once", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		testutil.NoError(t, r.
			WithPublishedTimeout(20*time.Millisecond).
			WithArrangeSignals("a", "a").
			WithArrangeSignals("a").
			WithAct(func(ps flam.IPubSub[string, string]) error {
				return ps.Publish(context.Background(), "a", "x")
			}).
			WithAssertPublished("a").
			Run())
		testutil.Equal(t, "failures", ft.failures(), []string{})
	})

	t.Run("should take the pub/sub", func(t *testing.T) {
		r, _ := newRunner(t, newApp(t, nil))
		testutil.NoError(t, r.WithArrangePubSub().WithAct(func(ps flam.IPubSub[string, string]) {
			testutil.Equal(t, "PubSub()", r.PubSub(), ps)
		}).Run())
	})

	t.Run("should fail with an invalid topic", func(t *testing.T) {
		r, ft := newRunner(t, newApp(t, nil))
		runError(t, r.WithArrangeSignals("a..b").Run(), ft, funit.ErrStage)
	})
}

// dataApp returns an application with a sqlite database "main" and a mini
// redis connection "default".
func dataApp(t *testing.T) flam.IApp {
	t.Helper()
	values := container.Bag{}
	for path, value := range map[string]any{
		database.DialectConfigPath + ".main": container.Bag{
			factory.ConfigFieldDriver: sqlite.Driver,
			sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
		},
		database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
		redis.ConnectionConfigPath + ".default": container.Bag{factory.ConfigFieldDriver: mini.Driver},
	} {
		testutil.NoError(t, values.Set(path, value))
	}
	return newApp(t, values, database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(), redis.NewBundle(), mini.NewBundle())
}

// user is a record of the database.
type user struct {
	Id   uint
	Name string
}

func Test_Runner_Data(t *testing.T) {
	t.Run("should work in a transaction rolled back once the run ends", func(t *testing.T) {
		a := dataApp(t)
		testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error {
			conn, err := f.Get("main")
			if err != nil {
				return err
			}
			return conn.AutoMigrate(&user{})
		}))
		r, _ := newRunner(t, a)
		var total int64
		testutil.NoError(t, r.
			WithArrangeDatabase("main").
			WithArrange("seed", func() error { return r.Tx().Create(&user{Name: "ann"}).Error }).
			WithAct(func() error { return r.Connection().Model(&user{}).Count(&total).Error }).
			WithAssertTotal(1, &total).
			Run())
		testutil.NoError(t, a.Invoke(func(f flam.IDatabaseConnectionFactory) error {
			conn, _ := f.Get("main")
			return conn.Model(&user{}).Count(&total).Error
		}))
		testutil.Equal(t, "total once rolled back", total, int64(0))
	})

	t.Run("should fail when the transaction cannot be rolled back", func(t *testing.T) {
		r, ft := newRunner(t, dataApp(t))
		err := r.
			WithArrangeDatabase("main").
			WithAct(func() error { return r.Tx().Commit().Error }).
			Run()
		runError(t, err, ft, funit.ErrRollback)
	})

	t.Run("should take the redis connection", func(t *testing.T) {
		r, _ := newRunner(t, dataApp(t))
		var got string
		testutil.NoError(t, r.
			WithArrangeRedis("default").
			WithAct(func() error {
				if err := r.Redis().Set(context.Background(), "k", "v", 0).Err(); err != nil {
					return err
				}
				var err error
				got, err = r.Redis().Get(context.Background(), "k").Result()
				return err
			}).
			Run())
		testutil.Equal(t, "got", got, "v")
	})

	for name, build := range map[string]func(r *funit.Runner){
		"unknown database": func(r *funit.Runner) { r.WithArrangeDatabase("none") },
		"unknown redis":    func(r *funit.Runner) { r.WithArrangeRedis("none") },
		"closed database": func(r *funit.Runner) {
			r.WithArrange("close", func(f flam.IDatabaseConnectionFactory) error {
				conn, _ := f.Get("main")
				db, _ := conn.DB()
				return db.Close()
			}).WithArrangeDatabase("main")
		},
	} {
		t.Run("should fail with an "+name, func(t *testing.T) {
			r, ft := newRunner(t, dataApp(t))
			build(r)
			runError(t, r.Run(), ft, funit.ErrStage)
		})
	}
}

func Test_Runner_Asserts(t *testing.T) {
	var (
		nilErr    error
		boomErr   = fmt.Errorf("wrapped: %w", errBoom)
		closedErr = fmt.Errorf("query: %w", sql.ErrConnDone)
		textErr   = errors.New("sql: database is closed")
		yes, no   = true, false
		two       = int64(2)
	)
	for name, tc := range map[string]struct {
		build func(r *funit.Runner)
		fails bool
	}{
		"no error":                {func(r *funit.Runner) { r.WithAssertNoError(&nilErr) }, false},
		"an error":                {func(r *funit.Runner) { r.WithAssertNoError(&boomErr) }, true},
		"expected error":          {func(r *funit.Runner) { r.WithAssertError("e", &boomErr, errBoom) }, false},
		"expected error and text": {func(r *funit.Runner) { r.WithAssertError("e", &boomErr, errBoom, "wrapped") }, false},
		"missing error":           {func(r *funit.Runner) { r.WithAssertError("e", &nilErr, errBoom) }, true},
		"other error":             {func(r *funit.Runner) { r.WithAssertError("e", &closedErr, errBoom) }, true},
		"other text":              {func(r *funit.Runner) { r.WithAssertError("e", &boomErr, errBoom, "other") }, true},
		"closed database":         {func(r *funit.Runner) { r.WithAssertDatabaseError(&closedErr) }, false},
		"closed database text":    {func(r *funit.Runner) { r.WithAssertDatabaseError(&textErr) }, false},
		"no database error":       {func(r *funit.Runner) { r.WithAssertDatabaseError(&nilErr) }, true},
		"other database error":    {func(r *funit.Runner) { r.WithAssertDatabaseError(&boomErr) }, true},
		"total":                   {func(r *funit.Runner) { r.WithAssertTotal(2, &two) }, false},
		"other total":             {func(r *funit.Runner) { r.WithAssertTotal(3, &two) }, true},
		"bool":                    {func(r *funit.Runner) { r.WithAssertBool(&yes, true) }, false},
		"other bool":              {func(r *funit.Runner) { r.WithAssertBool(&no, true) }, true},
	} {
		t.Run(name, func(t *testing.T) {
			r, ft := newRunner(t, newApp(t, nil))
			tc.build(r)
			testutil.NoError(t, r.Run())
			testutil.Equal(t, "failed", len(ft.failures()) == 1, tc.fails)
		})
	}
}

func Test_Logger_NilTest(t *testing.T) {
	var nilT *testing.T
	l := funit.NewLogger(nilT)
	l.Stage("arrange")
	l.Step("arrange", "seed")
	l.Error("arrange", "seed", errBoom)
}

func Test_Logger(t *testing.T) {
	ft := &fakeT{}
	l := funit.NewLogger(ft)
	l.Stage("arrange")
	l.Step("arrange", "seed")
	l.Error("arrange", "seed", errBoom)
	l.Error("boot", "", errBoom)
	testutil.Equal(t, "logs", ft.logs, []string{
		"funit: arrange", "funit: arrange: seed", "funit: arrange: seed failed: boom", "funit: boot failed: boom",
	})
}
