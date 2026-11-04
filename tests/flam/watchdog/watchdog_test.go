package watchdog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/app"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// process is a watchdog process whose n-th run, from 1, calls fn with the
// context of the run, which may wait for the process to be closed on
// closed.
type process struct {
	id       string
	fn       func(n int, ctx context.Context, closed <-chan struct{}) error
	closeErr error
	closed   chan struct{}

	mu      sync.Mutex
	runs    int
	running bool
	closes  int
}

// newProcess returns the process of the id running fn.
func newProcess(id string, fn func(n int, ctx context.Context, closed <-chan struct{}) error) *process {
	return &process{id: id, fn: fn, closed: make(chan struct{})}
}

// ending returns a process of the id whose runs return err.
func ending(id string, err error) *process {
	return newProcess(id, func(int, context.Context, <-chan struct{}) error { return err })
}

func (p *process) Id() string { return p.id }

func (p *process) Run(ctx context.Context) error {
	p.mu.Lock()
	p.runs++
	n := p.runs
	p.running = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.running = false
	}()
	return p.fn(n, ctx, p.closed)
}

func (p *process) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *process) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closes++
	if p.closes == 1 {
		close(p.closed)
	}
	return p.closeErr
}

// counts returns the number of runs and closes of the process.
func (p *process) counts() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.runs, p.closes
}

// recorder is a logger adaptor recording its reports, and panicking
// after each when panics.
type recorder struct {
	mu      sync.Mutex
	reports []string
	errs    []error
	panics  bool
}

func (r *recorder) Id() string { return "rec" }

func (r *recorder) Start(id string) { r.record("start:"+id, nil) }

func (r *recorder) Error(id string, e error) { r.record("error:"+id, e) }

func (r *recorder) Done(id string) { r.record("done:"+id, nil) }

func (r *recorder) record(report string, e error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, report)
	if e != nil {
		r.errs = append(r.errs, e)
	}
	if r.panics {
		panic("logger")
	}
}

// sorted returns the reports, sorted, as the processes run concurrently.
func (r *recorder) sorted() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Sorted(slices.Values(r.reports))
}

// recorderCreator creates the recorder for the "rec" driver.
type recorderCreator struct{ r *recorder }

func (c recorderCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "rec"
}

func (c recorderCreator) Create(flam.IBag) (flam.IWatchdogLoggerAdaptor, error) {
	return c.r, nil
}

// newApp returns a testutil.App with the config and watchdog bundles, a
// fake clock, a logger adaptor "rec" of the returned recorder, the
// processes in the process group, and the configuration holding the
// values, the restart delay being 0 unless set.
func newApp(t *testing.T, values map[string]any, processes ...flam.IWatchdogProcess) (*testutil.App, *recorder) {
	t.Helper()
	a := testutil.NewApp()
	testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
	r := &recorder{}
	testutil.NoError(t, a.Provide(func() flam.IWatchdogLoggerAdaptorCreator { return recorderCreator{r} },
		dig.Group(watchdog.LoggerCreatorGroup)))
	for _, p := range processes {
		testutil.NoError(t, a.Provide(func() flam.IWatchdogProcess { return p }, dig.Group(watchdog.ProcessGroup)))
	}

	bag := container.Bag{}
	testutil.NoError(t, bag.Set(watchdog.LoggerConfigPath+".rec", container.Bag{factory.ConfigFieldDriver: "rec"}))
	testutil.NoError(t, bag.Set(watchdog.RestartDelayConfigPath, "0s"))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a, r
}

// clock returns the fake clock of the app.
func clock(t *testing.T, a flam.IApp) *testutil.Clock {
	t.Helper()
	var c flam.IClock
	testutil.NoError(t, a.Invoke(func(clock flam.IClock) { c = clock }))
	return c.(*testutil.Clock)
}

// advance advances the clock by the delay of the timer a watchdog waits
// on, checking the timer does not fire earlier.
func advance(t *testing.T, c *testutil.Clock, delay time.Duration) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); c.Timers() != 1; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no timer of %v to advance", delay)
		}
	}
	c.Advance(delay - time.Nanosecond)
	testutil.Equal(t, fmt.Sprintf("timer of %v pending", delay), c.Timers(), 1)
	c.Advance(time.Nanosecond)
}

// panicError fails the test, and stops it, unless err is a *flam.Error
// wrapping ErrPanic, with the text and the context of the process and the
// value, along with the stack of the panic.
func panicError(t *testing.T, err error, text string, value any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, watchdog.ErrPanic) || !errors.As(err, &e) {
		t.Fatalf("error = %v, want it to wrap ErrPanic", err)
	}
	testutil.Equal(t, "Error()", e.Error(), text)
	ctx := e.Context()
	if stack, _ := ctx[watchdog.ErrorFieldStack].(string); !strings.Contains(stack, "watchdog_test.go") {
		t.Errorf("stack = %q, want the stack of the panic", stack)
	}
	delete(ctx, watchdog.ErrorFieldStack)
	testutil.Equal(t, "Context()", ctx, map[string]any{watchdog.ErrorFieldID: "api", watchdog.ErrorFieldPanic: value})
}

// kennel returns the kennel of the app.
func kennel(t *testing.T, a flam.IApp) flam.IKennel {
	t.Helper()
	var k flam.IKennel
	testutil.NoError(t, a.Invoke(func(kennel flam.IKennel) { k = kennel }))
	return k
}

// run runs the watchdog bundle of the app.
func run(a flam.IApp) error {
	return watchdog.NewBundle().(flam.IRunnableBundle).Run(a)
}

// closeBundle closes the watchdog bundle of the app.
func closeBundle(a flam.IApp) error {
	return watchdog.NewBundle().(flam.IClosableBundle).Close(a)
}

// kennelError fails the test, and stops it, unless err, returned from the
// container, wraps a *flam.Error wrapping the sentinel, with the text and
// exactly the context.
func kennelError(t *testing.T, err, sentinel error, text string, ctx map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	testutil.Equal(t, "Error()", e.Error(), text)
	testutil.Context(t, e, ctx)
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "KennelId", watchdog.KennelId, "flam.watchdog.kennel")
	testutil.Equal(t, "LoggerConfigPath", watchdog.LoggerConfigPath, "flam.watchdog.loggers")
	testutil.Equal(t, "LoggerCreatorGroup", watchdog.LoggerCreatorGroup, "flam.watchdog.loggers.creator")
	testutil.Equal(t, "ProcessGroup", watchdog.ProcessGroup, "flam.watchdog.processes")
	testutil.Equal(t, "ProcessConfigPath", watchdog.ProcessConfigPath, "flam.watchdog.processes")
	testutil.Equal(t, "RunConfigPath", watchdog.RunConfigPath, "flam.watchdog.run")
	testutil.Equal(t, "SignalsConfigPath", watchdog.SignalsConfigPath, "flam.watchdog.signals")
	testutil.Equal(t, "RestartDelayConfigPath", watchdog.RestartDelayConfigPath, "flam.watchdog.restart_delay")
	testutil.Equal(t, "RestartMaxDelayConfigPath", watchdog.RestartMaxDelayConfigPath, "flam.watchdog.restart_max_delay")
	testutil.Equal(t, "DrainDelayConfigPath", watchdog.DrainDelayConfigPath, "flam.watchdog.drain_delay")
	testutil.Equal(t, "ConfigFieldActive", watchdog.ConfigFieldActive, "active")
	testutil.Equal(t, "ConfigFieldLogger", watchdog.ConfigFieldLogger, "logger")
	testutil.Equal(t, "DefaultLoggerConfigPath", watchdog.DefaultLoggerConfigPath, "flam.watchdog.defaults.processes.logger")
	testutil.Equal(t, "ErrorFieldID", watchdog.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", watchdog.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldLogger", watchdog.ErrorFieldLogger, "logger")
	testutil.Equal(t, "ErrorFieldProcess", watchdog.ErrorFieldProcess, "process")
	testutil.Equal(t, "ErrorFieldPanic", watchdog.ErrorFieldPanic, "panic")
	testutil.Equal(t, "ErrorFieldStack", watchdog.ErrorFieldStack, "stack")
	for name, err := range map[string]error{
		"ErrNilProcess":       watchdog.ErrNilProcess,
		"ErrInvalidID":        watchdog.ErrInvalidID,
		"ErrIDMismatch":       watchdog.ErrIDMismatch,
		"ErrDuplicateProcess": watchdog.ErrDuplicateProcess,
		"ErrUnknownProcess":   watchdog.ErrUnknownProcess,
		"ErrRunning":          watchdog.ErrRunning,
		"ErrClosed":           watchdog.ErrClosed,
		"ErrLogger":           watchdog.ErrLogger,
		"ErrInvalidField":     watchdog.ErrInvalidField,
		"ErrPanic":            watchdog.ErrPanic,
		"ErrClose":            watchdog.ErrClose,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the kennel and the logger factory", func(t *testing.T) {
		b := watchdog.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.watchdog")

		a := testutil.NewApp()
		testutil.NoError(t, a.Provide(func() flam.IClock { return testutil.NewClock(time.Unix(0, 0)) }))
		testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
		testutil.NoError(t, a.RegisterBundle(b))
		calls := a.Calls()
		testutil.Equal(t, "calls", calls[len(calls)-4:], []string{"bundle:flam.watchdog", "provider/0", "provider/0", "provider/0"})
		testutil.NoError(t, a.Invoke(func(k flam.IKennel, f flam.IWatchdogLoggerAdaptorFactory) {
			testutil.Equal(t, "Id()", k.Id(), watchdog.KennelId)
			testutil.Equal(t, "Available()", k.Available(), []string{})
			testutil.Equal(t, "loggers", f.Available(), []string{})
		}))
	})

	t.Run("should register every logger driver", func(t *testing.T) {
		b := loggers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.watchdog.loggers")
	})

	t.Run("should close the kennel and the logger factory", func(t *testing.T) {
		api, jobs := ending("api", nil), ending("jobs", nil)
		api.closeErr, jobs.closeErr = errBoom, errBoom
		a, _ := newApp(t, nil, jobs, api)
		var f flam.IWatchdogLoggerAdaptorFactory
		testutil.NoError(t, a.Invoke(func(loggers flam.IWatchdogLoggerAdaptorFactory) { f = loggers }))
		testutil.NoError(t, f.Store("bad", failingLogger{}))

		err := closeBundle(a)
		if !errors.Is(err, errBoom) || !errors.Is(err, watchdog.ErrClose) {
			t.Fatalf("Close() = %v, want ErrClose and errBoom", err)
		}
		var e *flam.Error
		testutil.Equal(t, "as", errors.As(err, &e), true)
		testutil.Equal(t, "Error()", e.Error(), "unable to close watchdog kennel: unable to close watchdog process: boom")
		testutil.Context(t, e, map[string]any{watchdog.ErrorFieldID: "api"})
		testutil.Equal(t, "loggers closed", f.Has("bad"), false)
		for _, p := range []*process{api, jobs} {
			_, closes := p.counts()
			testutil.Equal(t, p.id+" closes", closes, 1)
		}

		testutil.NoError(t, closeBundle(a))
		_, closes := api.counts()
		testutil.Equal(t, "closed once", closes, 1)
		kennelError(t, kennel(t, a).Add("other", true, "", ending("other", nil)), watchdog.ErrClosed,
			"unable to add watchdog process: watchdog kennel closed", map[string]any{watchdog.ErrorFieldID: "other"})
		kennelError(t, run(a), watchdog.ErrClosed,
			"unable to run watchdog kennel: watchdog kennel closed", map[string]any{watchdog.ErrorFieldID: watchdog.KennelId})
	})

	t.Run("should fail to close without the dependencies", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
		if err := closeBundle(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})
}

// failingLogger is a logger adaptor failing to close.
type failingLogger struct{ flam.IWatchdogLoggerAdaptor }

func (failingLogger) Id() string   { return "bad" }
func (failingLogger) Close() error { return errBoom }

func Test_Kennel_Processes(t *testing.T) {
	t.Run("should add the processes of the group with their configuration", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{
			watchdog.DefaultLoggerConfigPath:                                   "rec",
			watchdog.ProcessConfigPath + ".jobs." + watchdog.ConfigFieldActive: false,
			watchdog.ProcessConfigPath + ".bare." + watchdog.ConfigFieldLogger: "",
		}, ending("jobs", nil), ending("api", nil), ending("bare", nil))
		k := kennel(t, a)
		testutil.Equal(t, "Available()", k.Available(), []string{"api", "bare", "jobs"})
		testutil.Equal(t, "Has(api)", k.Has("api"), true)
		testutil.Equal(t, "Has(none)", k.Has("none"), false)
		testutil.Equal(t, "IsActive(api)", k.IsActive("api"), true)
		testutil.Equal(t, "IsActive(jobs)", k.IsActive("jobs"), false)
		testutil.Equal(t, "IsActive(none)", k.IsActive("none"), false)
	})

	for name, tc := range map[string]struct {
		values map[string]any
		field  string
		id     string
	}{
		"an active field that is not a bool": {
			map[string]any{watchdog.ProcessConfigPath + ".api." + watchdog.ConfigFieldActive: "yes"}, watchdog.ConfigFieldActive, "api",
		},
		"a logger field that is not a string": {
			map[string]any{watchdog.ProcessConfigPath + ".api." + watchdog.ConfigFieldLogger: 7}, watchdog.ConfigFieldLogger, "api",
		},
		"a default logger that is not a string": {
			map[string]any{watchdog.DefaultLoggerConfigPath: 7}, watchdog.ConfigFieldLogger, "a",
		},
	} {
		t.Run("should fail on "+name+", closing the processes", func(t *testing.T) {
			first, api, last := ending("a", nil), ending("api", nil), ending("z", nil)
			a, _ := newApp(t, tc.values, first, api, last)
			kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrInvalidField,
				"unable to create watchdog kennel: invalid watchdog process field",
				map[string]any{watchdog.ErrorFieldID: tc.id, watchdog.ErrorFieldField: tc.field})
			for _, p := range []*process{first, api, last} {
				_, closes := p.counts()
				testutil.Equal(t, p.id+" closes", closes, 1)
			}
		})
	}

	t.Run("should report the failure once, closing the processes once", func(t *testing.T) {
		values := map[string]any{watchdog.ProcessConfigPath + ".api." + watchdog.ConfigFieldActive: "yes"}
		api := ending("api", nil)
		a, _ := newApp(t, values, api)
		if err := run(a); !errors.Is(err, watchdog.ErrInvalidField) {
			t.Fatalf("Run() = %v, want ErrInvalidField", err)
		}
		testutil.NoError(t, closeBundle(a))
		if err := a.Invoke(func(flam.IKennel) {}); !errors.Is(err, watchdog.ErrInvalidField) {
			t.Fatalf("Invoke() = %v, want ErrInvalidField", err)
		}
		runs, closes := api.counts()
		testutil.Equal(t, "counts", [2]int{runs, closes}, [2]int{0, 1})

		a, _ = newApp(t, values, ending("api", nil))
		if err := closeBundle(a); !errors.Is(err, watchdog.ErrInvalidField) {
			t.Fatalf("Close() = %v, want ErrInvalidField", err)
		}
		testutil.NoError(t, closeBundle(a))
	})

	for name, values := range map[string]map[string]any{
		"an integer delay":        {watchdog.RestartDelayConfigPath: 5},
		"an unparsable delay":     {watchdog.RestartDelayConfigPath: "1 minute"},
		"a negative delay":        {watchdog.RestartDelayConfigPath: "-1s"},
		"an invalid maximum":      {watchdog.RestartMaxDelayConfigPath: true},
		"a maximum shorter delay": {watchdog.RestartDelayConfigPath: "2s", watchdog.RestartMaxDelayConfigPath: "1s"},
		"an invalid drain delay":  {watchdog.DrainDelayConfigPath: "soon"},
	} {
		t.Run("should fail on "+name+", closing the processes", func(t *testing.T) {
			path := watchdog.RestartDelayConfigPath
			for _, other := range []string{watchdog.RestartMaxDelayConfigPath, watchdog.DrainDelayConfigPath} {
				if _, ok := values[other]; ok {
					path = other
				}
			}
			api := ending("api", nil)
			a, _ := newApp(t, values, api)
			kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrInvalidField,
				"unable to create watchdog kennel: invalid watchdog process field",
				map[string]any{watchdog.ErrorFieldID: watchdog.KennelId, watchdog.ErrorFieldField: path})
			_, closes := api.counts()
			testutil.Equal(t, "closes", closes, 1)
		})
	}

	t.Run("should fail on a process configuration that is not a bag", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{watchdog.ProcessConfigPath + ".api": false}, ending("api", nil))
		kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrInvalidField,
			"unable to create watchdog kennel: invalid watchdog process field",
			map[string]any{watchdog.ErrorFieldID: "api", watchdog.ErrorFieldField: watchdog.ProcessConfigPath + ".api"})
	})

	t.Run("should fail on a process of the group of an invalid id", func(t *testing.T) {
		for _, id := range []string{"", "api.v2"} {
			a, _ := newApp(t, nil, ending(id, nil))
			kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrInvalidID,
				"unable to create watchdog kennel: invalid watchdog process id", map[string]any{watchdog.ErrorFieldID: id})
		}
	})

	t.Run("should fail on a process of the group that cannot be added", func(t *testing.T) {
		a, _ := newApp(t, nil, ending("api", nil), ending("api", nil))
		kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrDuplicateProcess,
			"unable to add watchdog process: watchdog process id already in use", map[string]any{watchdog.ErrorFieldID: "api"})

		api := ending("api", nil)
		a, _ = newApp(t, nil, api, (*process)(nil))
		kennelError(t, a.Invoke(func(flam.IKennel) {}), watchdog.ErrNilProcess,
			"unable to add watchdog process: nil watchdog process", map[string]any{watchdog.ErrorFieldID: ""})
		_, closes := api.counts()
		testutil.Equal(t, "api closes", closes, 1)
	})
}

func Test_Kennel_Add(t *testing.T) {
	t.Run("should add a process", func(t *testing.T) {
		a, _ := newApp(t, nil)
		k := kennel(t, a)
		testutil.NoError(t, k.Add("api", false, "rec", ending("api", nil)))
		testutil.NoError(t, k.Add("jobs", true, "", ending("jobs", nil)))
		testutil.Equal(t, "Available()", k.Available(), []string{"api", "jobs"})
		testutil.Equal(t, "IsActive()", []bool{k.IsActive("api"), k.IsActive("jobs")}, []bool{false, true})
	})

	t.Run("should fail on an invalid process", func(t *testing.T) {
		a, _ := newApp(t, nil, ending("api", nil))
		k := kennel(t, a)
		kennelError(t, k.Add("x", true, "", nil), watchdog.ErrNilProcess,
			"unable to add watchdog process: nil watchdog process", map[string]any{watchdog.ErrorFieldID: "x"})
		kennelError(t, k.Add("x", true, "", (*process)(nil)), watchdog.ErrNilProcess,
			"unable to add watchdog process: nil watchdog process", map[string]any{watchdog.ErrorFieldID: "x"})
		kennelError(t, k.Add("", true, "", ending("", nil)), watchdog.ErrInvalidID,
			"unable to add watchdog process: invalid watchdog process id", map[string]any{watchdog.ErrorFieldID: ""})
		kennelError(t, k.Add("x", true, "", ending("y", nil)), watchdog.ErrIDMismatch,
			"unable to add watchdog process: watchdog process id mismatch",
			map[string]any{watchdog.ErrorFieldID: "x", watchdog.ErrorFieldProcess: "y"})
		kennelError(t, k.Add("api", true, "", ending("api", nil)), watchdog.ErrDuplicateProcess,
			"unable to add watchdog process: watchdog process id already in use", map[string]any{watchdog.ErrorFieldID: "api"})

		err := k.Add("jobs", true, "unknown", ending("jobs", nil))
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Add() = %v, want it to wrap factory.ErrUnknownID", err)
		}
		testutil.Error(t, err, watchdog.ErrLogger, err.Error(),
			map[string]any{watchdog.ErrorFieldID: "jobs", watchdog.ErrorFieldLogger: "unknown"})
		testutil.Equal(t, "Has(jobs)", k.Has("jobs"), false)
	})
}

func Test_Kennel_Activate(t *testing.T) {
	a, _ := newApp(t, nil, ending("api", nil))
	k := kennel(t, a)
	testutil.NoError(t, k.Deactivate("api"))
	testutil.Equal(t, "IsActive()", k.IsActive("api"), false)
	testutil.NoError(t, k.Activate("api"))
	testutil.Equal(t, "IsActive()", k.IsActive("api"), true)
	kennelError(t, k.Activate("none"), watchdog.ErrUnknownProcess,
		"unable to activate watchdog process: unknown watchdog process", map[string]any{watchdog.ErrorFieldID: "none"})
	kennelError(t, k.Deactivate("none"), watchdog.ErrUnknownProcess,
		"unable to deactivate watchdog process: unknown watchdog process", map[string]any{watchdog.ErrorFieldID: "none"})
}

func Test_Kennel_Process(t *testing.T) {
	started := make(chan string, 1)
	api := waiting("api", started, false)
	a, _ := newApp(t, nil, api)
	k := kennel(t, a)

	got, ok := k.Process("api")
	testutil.Equal(t, "Process(api)", got, flam.IWatchdogProcess(api))
	testutil.Equal(t, "Process(api) ok", ok, true)
	got, ok = k.Process("none")
	testutil.Equal(t, "Process(none)", got, nil)
	testutil.Equal(t, "Process(none) ok", ok, false)

	testutil.Equal(t, "IsRunning() before", k.IsRunning(), false)
	done := make(chan error, 1)
	go func() { done <- run(a) }()
	<-started
	testutil.Equal(t, "IsRunning() during", k.IsRunning(), true)
	testutil.NoError(t, api.Close())
	testutil.NoError(t, <-done)
	testutil.Equal(t, "IsRunning() after", k.IsRunning(), false)
}

func Test_Run(t *testing.T) {
	t.Run("should run the active processes until they end", func(t *testing.T) {
		api, jobs, idle := ending("api", nil), ending("jobs", errBoom), ending("idle", nil)
		a, r := newApp(t, map[string]any{
			watchdog.DefaultLoggerConfigPath:                                   "rec",
			watchdog.ProcessConfigPath + ".idle." + watchdog.ConfigFieldActive: false,
		}, api, jobs, idle)

		err := run(a)
		if !errors.Is(err, errBoom) {
			t.Fatalf("Run() = %v, want errBoom", err)
		}
		testutil.Equal(t, "reports", r.sorted(), []string{"done:api", "done:jobs", "error:jobs", "start:api", "start:jobs"})
		for p, want := range map[*process]int{api: 1, jobs: 1, idle: 0} {
			runs, _ := p.counts()
			testutil.Equal(t, p.id+" runs", runs, want)
		}
	})

	t.Run("should run the processes again when they panic", func(t *testing.T) {
		p := newProcess("api", func(n int, _ context.Context, _ <-chan struct{}) error {
			switch n {
			case 1:
				panic(errBoom)
			case 2:
				panic("again")
			}
			return nil
		})
		a, r := newApp(t, map[string]any{watchdog.DefaultLoggerConfigPath: "rec"}, p)

		// a delay of 0 restarts at once the first time only, the second
		// restart waiting the least delay, so that it does not spin
		c := clock(t, a)
		done := make(chan error, 1)
		go func() { done <- run(a) }()
		advance(t, c, 10*time.Millisecond)
		testutil.NoError(t, <-done)
		runs, _ := p.counts()
		testutil.Equal(t, "runs", runs, 3)
		testutil.Equal(t, "reports", r.reports, []string{"start:api", "error:api", "error:api", "done:api"})
		panicError(t, r.errs[0], "unable to run watchdog process: watchdog process panicked: boom", errBoom)
		if !errors.Is(r.errs[0], errBoom) {
			t.Fatalf("Error() = %v, want it to wrap errBoom", r.errs[0])
		}
		panicError(t, r.errs[1], "unable to run watchdog process: watchdog process panicked: again", "again")
	})

	t.Run("should restart at once every time with a maximum delay of 0", func(t *testing.T) {
		p := newProcess("api", func(n int, _ context.Context, _ <-chan struct{}) error {
			if n < 4 {
				panic(errBoom)
			}
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.RestartMaxDelayConfigPath: "0s"}, p)
		testutil.NoError(t, run(a))
		runs, _ := p.counts()
		testutil.Equal(t, "runs", runs, 4)
	})

	t.Run("should run nothing when disabled or without active process", func(t *testing.T) {
		p := ending("api", nil)
		a, _ := newApp(t, map[string]any{watchdog.RunConfigPath: false}, p)
		testutil.NoError(t, run(a))
		a, _ = newApp(t, map[string]any{watchdog.ProcessConfigPath + ".api." + watchdog.ConfigFieldActive: false}, p)
		testutil.NoError(t, run(a))
		a, _ = newApp(t, map[string]any{watchdog.SignalsConfigPath: false}, p)
		testutil.NoError(t, run(a))
		runs, _ := p.counts()
		testutil.Equal(t, "runs", runs, 1)
	})

	t.Run("should refuse changes while running", func(t *testing.T) {
		var k flam.IKennel
		var errs []error
		p := newProcess("api", func(int, context.Context, <-chan struct{}) error {
			errs = append(errs, k.Activate("idle"), k.Deactivate("api"), k.Add("jobs", true, "", ending("jobs", nil)))
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.ProcessConfigPath + ".idle." + watchdog.ConfigFieldActive: false}, p, ending("idle", nil))
		k = kennel(t, a)
		testutil.NoError(t, run(a))
		kennelError(t, errs[0], watchdog.ErrRunning,
			"unable to activate watchdog process: watchdog kennel running", map[string]any{watchdog.ErrorFieldID: "idle"})
		kennelError(t, errs[1], watchdog.ErrRunning,
			"unable to deactivate watchdog process: watchdog kennel running", map[string]any{watchdog.ErrorFieldID: "api"})
		kennelError(t, errs[2], watchdog.ErrRunning,
			"unable to add watchdog process: watchdog kennel running", map[string]any{watchdog.ErrorFieldID: "jobs"})
		testutil.NoError(t, k.Deactivate("api"))
		testutil.NoError(t, k.Add("jobs", true, "", ending("jobs", nil)))
	})

	t.Run("should drop the panics of the logger adaptor", func(t *testing.T) {
		p := newProcess("api", func(n int, _ context.Context, _ <-chan struct{}) error {
			if n == 1 {
				panic("again")
			}
			return errBoom
		})
		a, r := newApp(t, map[string]any{watchdog.DefaultLoggerConfigPath: "rec"}, p)
		r.panics = true
		if err := run(a); !errors.Is(err, errBoom) {
			t.Fatalf("Run() = %v, want errBoom", err)
		}
		testutil.Equal(t, "reports", r.reports, []string{"start:api", "error:api", "error:api", "done:api"})
	})

	t.Run("should wait before restarting a process", func(t *testing.T) {
		var c *testutil.Clock
		p := newProcess("api", func(n int, _ context.Context, _ <-chan struct{}) error {
			switch n {
			case 4:
				c.Advance(2 * time.Second)
				fallthrough
			case 1, 2, 3:
				panic("again")
			}
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.RestartDelayConfigPath: "1s", watchdog.RestartMaxDelayConfigPath: "2s"}, p)
		c = clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		for _, delay := range []time.Duration{time.Second, 2 * time.Second, 2 * time.Second, time.Second} {
			advance(t, c, delay)
		}
		testutil.NoError(t, <-result)
		runs, _ := p.counts()
		testutil.Equal(t, "runs", runs, 5)
	})

	t.Run("should take the restart delay as the default maximum when longer", func(t *testing.T) {
		p := newProcess("api", func(n int, _ context.Context, _ <-chan struct{}) error {
			if n < 3 {
				panic("again")
			}
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.RestartDelayConfigPath: "1m"}, p)
		c := clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		advance(t, c, time.Minute)
		advance(t, c, time.Minute)
		testutil.NoError(t, <-result)
	})
}

// waiting returns a process of the id, signaling started on its first run,
// returning when closed, and panicking then on its n-th run when panics.
func waiting(id string, started chan<- string, panics bool) *process {
	return newProcess(id, func(n int, ctx context.Context, closed <-chan struct{}) error {
		if n == 1 {
			started <- id
		}
		<-closed
		if panics {
			panic(fmt.Sprintf("closed %d", n))
		}
		return nil
	})
}

func Test_Run_Signals(t *testing.T) {
	t.Run("should close the kennel on a signal", func(t *testing.T) {
		started := make(chan string, 2)
		api, jobs := waiting("api", started, false), waiting("jobs", started, false)
		a, r := newApp(t, map[string]any{watchdog.DefaultLoggerConfigPath: "rec"}, api, jobs)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		testutil.NoError(t, <-result)
		testutil.Equal(t, "reports", r.sorted(), []string{"done:api", "done:jobs", "start:api", "start:jobs"})

		testutil.NoError(t, closeBundle(a))
		for _, p := range []*process{api, jobs} {
			_, closes := p.counts()
			testutil.Equal(t, p.id+" closes", closes, 1)
		}
	})

	t.Run("should stop waiting to restart a process once closed", func(t *testing.T) {
		started := make(chan string, 1)
		p := newProcess("api", func(int, context.Context, <-chan struct{}) error {
			started <- "api"
			panic("again")
		})
		a, r := newApp(t, map[string]any{watchdog.DefaultLoggerConfigPath: "rec", watchdog.RestartDelayConfigPath: "1h"}, p)
		c := clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		for deadline := time.Now().Add(5 * time.Second); c.Timers() != 1; time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("no restart timer")
			}
		}
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		testutil.NoError(t, <-result)
		testutil.Equal(t, "reports", r.reports, []string{"start:api", "error:api", "done:api"})
		testutil.Equal(t, "timers", c.Timers(), 0)
	})

	t.Run("should cancel the context of the processes on a signal", func(t *testing.T) {
		started := make(chan string, 1)
		p := newProcess("api", func(_ int, ctx context.Context, _ <-chan struct{}) error {
			started <- "api"
			<-ctx.Done()
			return ctx.Err()
		})
		a, _ := newApp(t, nil, p)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		if err := <-result; !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() = %v, want context.Canceled", err)
		}
	})

	t.Run("should not listen to the signals the program ignores", func(t *testing.T) {
		signal.Ignore(syscall.SIGINT, syscall.SIGTERM)
		defer func() {
			// Reset keeps the signals ignored: listening to them, then
			// stopping, restores their default handling.
			restore := make(chan os.Signal, 1)
			signal.Notify(restore, syscall.SIGINT, syscall.SIGTERM)
			signal.Stop(restore)
			testutil.Equal(t, "ignored", signal.Ignored(syscall.SIGINT), false)
		}()
		started, release := make(chan string, 1), make(chan struct{})
		p := newProcess("api", func(int, context.Context, <-chan struct{}) error {
			started <- "api"
			<-release
			return nil
		})
		a, _ := newApp(t, nil, p)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		time.Sleep(50 * time.Millisecond)
		_, closes := p.counts()
		testutil.Equal(t, "closes", closes, 0)
		close(release)
		testutil.NoError(t, <-result)
	})

	t.Run("should return the errors of the closing but not the panics once closed", func(t *testing.T) {
		started := make(chan string, 1)
		p := waiting("api", started, true)
		p.closeErr = errBoom
		a, _ := newApp(t, nil, p)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
		err := <-result
		if !errors.Is(err, watchdog.ErrClose) || !errors.Is(err, errBoom) || errors.Is(err, watchdog.ErrPanic) {
			t.Fatalf("Run() = %v, want ErrClose and errBoom, but not ErrPanic", err)
		}
		runs, _ := p.counts()
		testutil.Equal(t, "runs", runs, 1)
	})
}

// waitTimers waits for the clock to hold n timers.
func waitTimers(t *testing.T, c *testutil.Clock, n int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); c.Timers() != n; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timers = %d, want %d", c.Timers(), n)
		}
	}
}

func Test_Run_Drain(t *testing.T) {
	t.Run("should not drain without signal", func(t *testing.T) {
		a, _ := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "5s"}, ending("api", nil))
		testutil.NoError(t, run(a))
		testutil.Equal(t, "IsDraining()", kennel(t, a).IsDraining(), false)
	})

	t.Run("should be draining on a signal without drain delay", func(t *testing.T) {
		started := make(chan string, 1)
		a, _ := newApp(t, nil, waiting("api", started, false))
		k := kennel(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.Equal(t, "IsDraining() before", k.IsDraining(), false)
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		testutil.NoError(t, <-result)
		testutil.Equal(t, "IsDraining() after", k.IsDraining(), true)
		testutil.Equal(t, "timers", clock(t, a).Timers(), 0)
	})

	t.Run("should drain for the delay before closing", func(t *testing.T) {
		started := make(chan string, 1)
		var cancelled atomic.Bool
		p := newProcess("api", func(_ int, ctx context.Context, closed <-chan struct{}) error {
			started <- "api"
			<-closed
			cancelled.Store(ctx.Err() != nil)
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "5s"}, p)
		k, c := kennel(t, a), clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		advance(t, c, 5*time.Second)
		testutil.NoError(t, <-result)
		testutil.Equal(t, "cancelled", cancelled.Load(), true)
		testutil.Equal(t, "IsDraining()", k.IsDraining(), true)
		testutil.Equal(t, "IsRunning()", k.IsRunning(), false)
		_, closes := p.counts()
		testutil.Equal(t, "closes", closes, 1)
	})

	t.Run("should keep the processes running while draining", func(t *testing.T) {
		started := make(chan string, 1)
		p := waiting("api", started, false)
		a, _ := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "5s"}, p)
		k, c := kennel(t, a), clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		waitTimers(t, c, 1)
		testutil.Equal(t, "IsDraining()", k.IsDraining(), true)
		testutil.Equal(t, "IsRunning()", k.IsRunning(), true)
		testutil.Equal(t, "process running", p.IsRunning(), true)
		_, closes := p.counts()
		testutil.Equal(t, "closes while draining", closes, 0)

		c.Advance(5 * time.Second)
		testutil.NoError(t, <-result)
		_, closes = p.counts()
		testutil.Equal(t, "closes", closes, 1)
	})

	t.Run("should cut the drain short on a second signal", func(t *testing.T) {
		started := make(chan string, 1)
		p := waiting("api", started, false)
		a, _ := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "1h"}, p)
		c := clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		waitTimers(t, c, 1)
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGINT))
		testutil.NoError(t, <-result)
		testutil.Equal(t, "timers", c.Timers(), 0)
		_, closes := p.counts()
		testutil.Equal(t, "closes", closes, 1)
	})

	t.Run("should not close the kennel when the processes end while draining", func(t *testing.T) {
		started, release := make(chan string, 1), make(chan struct{})
		p := newProcess("api", func(int, context.Context, <-chan struct{}) error {
			started <- "api"
			<-release
			return nil
		})
		a, _ := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "1h"}, p)
		k, c := kennel(t, a), clock(t, a)

		result := make(chan error, 1)
		go func() { result <- run(a) }()
		<-started
		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		waitTimers(t, c, 1)
		close(release)
		testutil.NoError(t, <-result)
		testutil.Equal(t, "timers", c.Timers(), 0)
		testutil.Equal(t, "IsDraining()", k.IsDraining(), true)
		_, closes := p.counts()
		testutil.Equal(t, "closes", closes, 0)
		testutil.NoError(t, closeBundle(a))
	})
}

func Test_Application(t *testing.T) {
	values := container.Bag{}
	testutil.NoError(t, values.Set(watchdog.ProcessConfigPath+".jobs."+watchdog.ConfigFieldActive, false))
	a := app.New(values)
	testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(loggers.NewBundle()))
	api, jobs := ending("api", nil), ending("jobs", nil)
	for _, p := range []*process{api, jobs} {
		testutil.NoError(t, a.Provide(func() flam.IWatchdogProcess { return p }, dig.Group(watchdog.ProcessGroup)))
	}
	testutil.NoError(t, a.Run())
	testutil.NoError(t, a.Close())
	for p, want := range map[*process][2]int{api: {1, 1}, jobs: {0, 1}} {
		runs, closes := p.counts()
		testutil.Equal(t, p.id+" counts", [2]int{runs, closes}, want)
	}
}
