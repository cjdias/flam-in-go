package scheduler

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/scheduler"
	"github.com/cjdias/flam-in-go/pkg/flam/trigger"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

const (
	patience = 2 * time.Second       // to wait for an expected event
	quiet    = 50 * time.Millisecond // to conclude that nothing happens
)

// start is the time of the fake clocks, a Thursday at midnight.
var start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

var errBoom = errors.New("boom")

// ---------------------------------------------------------------------------
// jobs
// ---------------------------------------------------------------------------

// job is a flam.IScheduledJob sending the context of each run on runs,
// then returning the result of fn, or nil when fn is nil.
type job struct {
	id   string
	fn   func(ctx context.Context) error
	runs chan context.Context
}

// newJob returns the job of the id running fn.
func newJob(id string, fn func(ctx context.Context) error) *job {
	return &job{id: id, fn: fn, runs: make(chan context.Context, 100)}
}

func (j *job) Id() string { return j.id }

func (j *job) Run(ctx context.Context) error {
	j.runs <- ctx
	if j.fn == nil {
		return nil
	}
	return j.fn(ctx)
}

// ---------------------------------------------------------------------------
// logger
// ---------------------------------------------------------------------------

// entry is a message signalled to the logger.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     container.Bag
}

// logger is a flam.ILogger sending the signalled messages on entries.
type logger struct {
	flam.ILogger

	entries chan entry
}

// newLogger returns an empty logger.
func newLogger() *logger {
	return &logger{entries: make(chan entry, 100)}
}

func (l *logger) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	l.entries <- entry{level, channel, message, ctx[0].(container.Bag)}
}

// next returns the next entry signalled, checking its level, channel,
// message and job.
func (l *logger) next(t *testing.T, level flam.LogLevel, message, jobId string) entry {
	t.Helper()
	e := receive(t, "log entry", l.entries)
	testutil.Equal(t, "level", e.level, level)
	testutil.Equal(t, "channel", e.channel, scheduler.DefaultLogChannel)
	testutil.Equal(t, "message", e.message, message)
	testutil.Equal(t, "job", e.ctx[scheduler.ContextFieldJob], jobId)
	return e
}

// failure returns the error of the next entry signalled, a failure of the
// job.
func (l *logger) failure(t *testing.T, jobId string) error {
	t.Helper()
	e := l.next(t, flam.LogError, "scheduled job failed", jobId)
	err, ok := e.ctx[scheduler.ContextFieldError].(error)
	if !ok {
		t.Fatalf("failure entry without error: %#v", e.ctx)
	}
	return err
}

// ---------------------------------------------------------------------------
// lockers
// ---------------------------------------------------------------------------

// tryCall is a call of TryAcquire.
type tryCall struct {
	key string
	ttl time.Duration
}

// locker is a flam.ILocker whose locks are held in held, failing to
// acquire with tryErr and to release with releaseErr when set.
type locker struct {
	flam.ILocker

	mu          sync.Mutex
	held        map[string]bool
	tryErr      error
	releaseErr  error
	tries       []tryCall
	releases    []string
	releaseErrs []error // the error of the context of each release
}

// newLocker returns a locker holding no lock.
func newLocker() *locker {
	return &locker{held: map[string]bool{}}
}

func (l *locker) TryAcquire(_ context.Context, key string, ttl time.Duration) (flam.ILock, bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.tries = append(l.tries, tryCall{key, ttl})
	switch {
	case l.tryErr != nil:
		return nil, false, l.tryErr
	case l.held[key]:
		return nil, false, nil
	}
	l.held[key] = true
	return &lock{locker: l, key: key}, true, nil
}

// hold makes the lock of the key held by another instance, or not.
func (l *locker) hold(key string, held bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.held[key] = held
}

// calls returns the calls of TryAcquire, and the keys released with the
// errors of their contexts.
func (l *locker) calls() ([]tryCall, []string, []error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]tryCall(nil), l.tries...), append([]string(nil), l.releases...),
		append([]error(nil), l.releaseErrs...)
}

// lock is a flam.ILock of a locker.
type lock struct {
	flam.ILock

	locker *locker
	key    string
}

func (k *lock) Release(ctx context.Context) error {
	k.locker.mu.Lock()
	defer k.locker.mu.Unlock()
	k.locker.releases = append(k.locker.releases, k.key)
	k.locker.releaseErrs = append(k.locker.releaseErrs, ctx.Err())
	delete(k.locker.held, k.key)
	return k.locker.releaseErr
}

// lockers is a flam.ILockerFactory of lockers, by id.
type lockers struct {
	flam.ILockerFactory

	mu      sync.Mutex
	lockers map[string]flam.ILocker
	gets    []string
}

func (f *lockers) Get(id string) (flam.ILocker, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gets = append(f.gets, id)
	if l, ok := f.lockers[id]; ok {
		return l, nil
	}
	return nil, errBoom
}

// ---------------------------------------------------------------------------
// application
// ---------------------------------------------------------------------------

// options are the parts of the application of a test.
type options struct {
	values  map[string]any
	jobs    []flam.IScheduledJob
	logger  flam.ILogger
	lockers flam.ILockerFactory
}

// newApp returns a testutil.App with the config, trigger and scheduler
// bundles, a fake clock at start, the jobs in the job group, the logger
// and the locker factory when not nil, and the configuration holding the
// values.
func newApp(t *testing.T, o options) (*testutil.App, *testutil.Clock) {
	t.Helper()
	a := testutil.NewApp()
	clock := testutil.NewClock(start)
	testutil.NoError(t, a.Provide(func() flam.IClock { return clock }))
	testutil.NoError(t, a.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(trigger.NewBundle()))
	testutil.NoError(t, a.RegisterBundle(scheduler.NewBundle()))
	for _, j := range o.jobs {
		testutil.NoError(t, a.Provide(func() flam.IScheduledJob { return j }, dig.Group(scheduler.JobGroup)))
	}
	if o.logger != nil {
		testutil.NoError(t, a.Provide(func() flam.ILogger { return o.logger }))
	}
	if o.lockers != nil {
		testutil.NoError(t, a.Provide(func() flam.ILockerFactory { return o.lockers }))
	}

	bag := container.Bag{}
	for path, value := range o.values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = a.Invoke(func(f flam.ITriggerFactory) error { return f.Close() })
	})
	return a, clock
}

// processesIn takes the processes of the watchdog group.
type processesIn struct {
	dig.In

	Processes []flam.IWatchdogProcess `group:"flam.watchdog.processes"`
}

// processes returns the processes provided in the watchdog group, or
// the error of their providers.
func processes(a *testutil.App) ([]flam.IWatchdogProcess, error) {
	var ps []flam.IWatchdogProcess
	err := a.Invoke(func(in processesIn) { ps = in.Processes })
	return ps, err
}

// newProcess returns the scheduler process of an application of the
// options, and its clock.
func newProcess(t *testing.T, o options) (flam.IWatchdogProcess, *testutil.Clock) {
	t.Helper()
	a, clock := newApp(t, o)
	ps, err := processes(a)
	testutil.NoError(t, err)
	if len(ps) != 1 {
		t.Fatalf("%d processes, want 1", len(ps))
	}
	t.Cleanup(func() { _ = ps[0].Close() })
	return ps[0], clock
}

// provideErr returns the error of the provider of the processes of an
// application of the options.
func provideErr(t *testing.T, o options) error {
	t.Helper()
	a, _ := newApp(t, o)
	_, err := processes(a)
	if err == nil {
		t.Fatal("processes() = nil error, want one")
	}
	return err
}

// run runs the process on a goroutine of its own, returning the cancel
// function of its context and the channel its result is sent on.
func run(t *testing.T, p flam.IWatchdogProcess) (context.CancelFunc, chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- p.Run(ctx) }()
	t.Cleanup(cancel)
	return cancel, result
}

// timers waits for the clock to hold n timers.
func timers(t *testing.T, clock *testutil.Clock, n int) {
	t.Helper()
	eventually(t, "timers", func() bool { return clock.Timers() == n })
}

// ---------------------------------------------------------------------------
// waiting
// ---------------------------------------------------------------------------

func receive[T any](t *testing.T, name string, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(patience):
		t.Fatalf("%s: nothing received", name)
		panic("unreachable")
	}
}

func staysQuiet[T any](t *testing.T, name string, ch <-chan T) {
	t.Helper()
	select {
	case v := <-ch:
		t.Fatalf("%s: unexpected %v", name, v)
	case <-time.After(quiet):
	}
}

func eventually(t *testing.T, name string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(patience)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: condition never met", name)
		}
		runtime.Gosched()
	}
}

func asFlamError(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("%v (%T) is not a *flam.Error", err, err)
	}
	return e
}
