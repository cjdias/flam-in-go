package checks

import (
	"context"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	kennelcheck "github.com/cjdias/flam-in-go/pkg/flam/health/checks/kennel"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// process is a flam.IWatchdogProcess telling whether it runs as set.
type process struct {
	id      string
	running atomic.Bool
}

func (p *process) Id() string                { return p.id }
func (p *process) Run(context.Context) error { return nil }
func (p *process) IsRunning() bool           { return p.running.Load() }
func (p *process) Close() error              { return nil }

// kennel is a flam.IKennel holding its processes, running as set.
type kennel struct {
	flam.IKennel

	processes map[string]*process
	inactive  map[string]bool
	running   atomic.Bool
	draining  atomic.Bool
}

func (k *kennel) Available() []string {
	return slices.Sorted(maps.Keys(k.processes))
}

func (k *kennel) Has(id string) bool {
	_, ok := k.processes[id]
	return ok
}

func (k *kennel) IsActive(id string) bool {
	return k.Has(id) && !k.inactive[id]
}

func (k *kennel) Process(id string) (flam.IWatchdogProcess, bool) {
	p, ok := k.processes[id]
	return p, ok
}

func (k *kennel) IsRunning() bool { return k.running.Load() }

func (k *kennel) IsDraining() bool { return k.draining.Load() }

func (k *kennel) Close() error { return nil }

// kennelBundle provides the kennel.
type kennelBundle struct{ k *kennel }

func (kennelBundle) Id() string { return "kennel" }

func (b kennelBundle) Register(a flam.IApp) error {
	return a.Provide(func() flam.IKennel { return b.k })
}

// kennelApp returns an app with a running kennel of the processes "api",
// "web" and "jobs", the last one inactive, and the kennel check driver.
func kennelApp(t *testing.T) (*testutil.App, *kennel) {
	t.Helper()
	k := &kennel{
		processes: map[string]*process{"api": {id: "api"}, "web": {id: "web"}, "jobs": {id: "jobs"}},
		inactive:  map[string]bool{"jobs": true},
	}
	k.running.Store(true)
	return newApp(t, nil, kennelBundle{k}, kennelcheck.NewBundle()), k
}

// realKennelApp returns an app with the watchdog bundle, whose kennel
// holds no process, and the kennel check driver.
func realKennelApp(t *testing.T) *testutil.App {
	t.Helper()
	return newApp(t, nil, watchdog.NewBundle(), kennelcheck.NewBundle())
}

// blockingProcess is a flam.IWatchdogProcess signaling started once it
// runs, and running until closed.
type blockingProcess struct {
	started chan struct{}
	closed  chan struct{}
	once    sync.Once
	running atomic.Bool
}

func (p *blockingProcess) Id() string { return "api" }

func (p *blockingProcess) Run(context.Context) error {
	p.running.Store(true)
	defer p.running.Store(false)
	close(p.started)
	<-p.closed
	return nil
}

func (p *blockingProcess) IsRunning() bool { return p.running.Load() }

func (p *blockingProcess) Close() error {
	p.once.Do(func() { close(p.closed) })
	return nil
}

// processBundle provides the process in the watchdog process group.
type processBundle struct{ p *blockingProcess }

func (processBundle) Id() string { return "process" }

func (b processBundle) Register(a flam.IApp) error {
	return a.Provide(func() flam.IWatchdogProcess { return b.p }, dig.Group(watchdog.ProcessGroup))
}

func Test_Kennel_Declarations(t *testing.T) {
	testutil.Equal(t, "Driver", kennelcheck.Driver, "flam.health.checks.driver.kennel")
	testutil.Equal(t, "ConfigFieldProcesses", kennelcheck.ConfigFieldProcesses, "processes")
	testutil.Equal(t, "ConfigFieldDraining", kennelcheck.ConfigFieldDraining, "draining")
	testutil.Equal(t, "ErrorFieldID", kennelcheck.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldField", kennelcheck.ErrorFieldField, "field")
	testutil.Equal(t, "ErrorFieldProcess", kennelcheck.ErrorFieldProcess, "process")
	testutil.Equal(t, "ErrorFieldProcesses", kennelcheck.ErrorFieldProcesses, "processes")
}

func Test_Kennel_Bundle(t *testing.T) {
	b := kennelcheck.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.checks.kennel")

	a := newApp(t, nil, b)
	c := creatorOf(t, a)
	testutil.Equal(t, "Accept(driver)", c.Accept(checkBag("main", kennelcheck.Driver, nil)), true)
	testutil.Equal(t, "Accept(other)", c.Accept(checkBag("main", "other", nil)), false)
}

func Test_Kennel_Create(t *testing.T) {
	t.Run("should refuse an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{health.ConfigFieldKind: "none"}, health.ConfigFieldKind},
			{container.Bag{kennelcheck.ConfigFieldProcesses: "api"}, kennelcheck.ConfigFieldProcesses},
			{container.Bag{kennelcheck.ConfigFieldProcesses: []any{"api", 1}}, kennelcheck.ConfigFieldProcesses},
			{container.Bag{kennelcheck.ConfigFieldDraining: "yes"}, kennelcheck.ConfigFieldDraining},
		} {
			a, _ := kennelApp(t)
			check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, tc.fields))
			testutil.Error(t, err, kennelcheck.ErrInvalidField,
				"unable to create kennel health check: invalid kennel health check field",
				map[string]any{kennelcheck.ErrorFieldID: "servers", kennelcheck.ErrorFieldField: tc.field})
			testutil.Equal(t, "check", check, nil)
		}
	})

	t.Run("should fail without kennel", func(t *testing.T) {
		a := newApp(t, nil, kennelcheck.NewBundle())
		_, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.Error(t, err, kennelcheck.ErrNoKennel,
			"unable to create kennel health check: no watchdog kennel",
			map[string]any{kennelcheck.ErrorFieldID: "servers"})
	})

	t.Run("should fail for a process the kennel does not hold", func(t *testing.T) {
		a, _ := kennelApp(t)
		_, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, container.Bag{
			kennelcheck.ConfigFieldProcesses: []any{"api", "other"},
		}))
		testutil.Error(t, err, kennelcheck.ErrUnknownProcess,
			"unable to create kennel health check: unknown watchdog process",
			map[string]any{kennelcheck.ErrorFieldID: "servers", kennelcheck.ErrorFieldProcess: "other"})
	})

	t.Run("should check the readiness unless configured", func(t *testing.T) {
		a, _ := kennelApp(t)
		c := creatorOf(t, a)

		check, err := c.Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", check.Id(), "servers")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)

		check, err = c.Create(checkBag("servers", kennelcheck.Driver, container.Bag{health.ConfigFieldKind: "both"}))
		testutil.NoError(t, err)
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthBoth)
	})
}

func Test_Kennel_Check(t *testing.T) {
	t.Run("should fail while active processes are not running", func(t *testing.T) {
		a, k := kennelApp(t)
		processes := k.processes
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)

		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrNotRunning,
			"kennel health check failed: watchdog process not running: api, web",
			map[string]any{kennelcheck.ErrorFieldID: "servers", kennelcheck.ErrorFieldProcesses: []string{"api", "web"}})

		processes["api"].running.Store(true)
		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrNotRunning,
			"kennel health check failed: watchdog process not running: web",
			map[string]any{kennelcheck.ErrorFieldID: "servers", kennelcheck.ErrorFieldProcesses: []string{"web"}})

		processes["web"].running.Store(true)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should watch the processes of its field only", func(t *testing.T) {
		a, k := kennelApp(t)
		processes := k.processes
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, container.Bag{
			kennelcheck.ConfigFieldProcesses: []string{"jobs", "api"},
		}))
		testutil.NoError(t, err)

		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrNotRunning,
			"kennel health check failed: watchdog process not running: api",
			map[string]any{kennelcheck.ErrorFieldID: "servers", kennelcheck.ErrorFieldProcesses: []string{"api"}})

		processes["api"].running.Store(true)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should pass while the kennel does not run", func(t *testing.T) {
		a, k := kennelApp(t)
		k.running.Store(false)
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should fail while the kennel drains", func(t *testing.T) {
		a, k := kennelApp(t)
		for _, p := range k.processes {
			p.running.Store(true)
		}
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))

		k.draining.Store(true)
		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrDraining,
			"kennel health check failed: watchdog kennel draining",
			map[string]any{kennelcheck.ErrorFieldID: "servers"})

		k.running.Store(false)
		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrDraining,
			"kennel health check failed: watchdog kennel draining",
			map[string]any{kennelcheck.ErrorFieldID: "servers"})
	})

	t.Run("should ignore the drain when configured", func(t *testing.T) {
		a, k := kennelApp(t)
		for _, p := range k.processes {
			p.running.Store(true)
		}
		k.draining.Store(true)
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, container.Bag{
			kennelcheck.ConfigFieldDraining: false,
		}))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})

	t.Run("should fail while the kennel of the watchdog drains", func(t *testing.T) {
		p := &blockingProcess{started: make(chan struct{}), closed: make(chan struct{})}
		a := newApp(t, map[string]any{watchdog.DrainDelayConfigPath: "5s"},
			watchdog.NewBundle(), processBundle{p}, kennelcheck.NewBundle())
		check, err := creatorOf(t, a).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)
		var c flam.IClock
		testutil.NoError(t, a.Invoke(func(clock flam.IClock) { c = clock }))
		clock := c.(*testutil.Clock)

		result := make(chan error, 1)
		go func() { result <- watchdog.NewBundle().(flam.IRunnableBundle).Run(a) }()
		<-p.started
		testutil.NoError(t, check.Check(context.Background()))

		testutil.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGTERM))
		for deadline := time.Now().Add(5 * time.Second); clock.Timers() != 1; time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("no drain timer")
			}
		}
		testutil.Equal(t, "running", p.IsRunning(), true)
		testutil.Error(t, check.Check(context.Background()), kennelcheck.ErrDraining,
			"kennel health check failed: watchdog kennel draining",
			map[string]any{kennelcheck.ErrorFieldID: "servers"})

		clock.Advance(5 * time.Second)
		testutil.NoError(t, <-result)
		testutil.Equal(t, "running", p.IsRunning(), false)
		testutil.NoError(t, watchdog.NewBundle().(flam.IClosableBundle).Close(a))
	})

	t.Run("should check the kennel of the watchdog", func(t *testing.T) {
		check, err := creatorOf(t, realKennelApp(t)).Create(checkBag("servers", kennelcheck.Driver, nil))
		testutil.NoError(t, err)
		testutil.NoError(t, check.Check(context.Background()))
	})
}
