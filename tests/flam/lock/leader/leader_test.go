package leader

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/leader"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/memory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ttl is the ttl of the leadership locks of the tests.
const ttl = 3 * time.Second

var errBoom = errors.New("boom")

// newLocker returns a memory locker waiting 1s without jitter to try
// again, refreshing its locks on its own when autoRefresh, and the fake
// clock it reads.
func newLocker(t *testing.T, autoRefresh bool) (flam.ILocker, *testutil.Clock) {
	t.Helper()
	clock := testutil.NewClock(time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC))
	bundles := []flam.IBundle{config.NewBundle(), lock.NewBundle(), lockers.NewBundle()}
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(lock.LockerConfigPath+".main", container.Bag{
		factory.ConfigFieldDriver:     memory.Driver,
		lock.ConfigFieldRetryInterval: "1s",
		lock.ConfigFieldRetryJitter:   "0s",
		lock.ConfigFieldAutoRefresh:   autoRefresh,
	}))
	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() { _ = lock.NewBundle().(flam.IClosableBundle).Close(app) })
	var l flam.ILocker
	testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory) (err error) {
		l, err = f.Get("main")
		return err
	}))
	return l, clock
}

// leading is a function of a leader process, telling the test the
// context of each of its runs, and leading until it is done.
type leading struct {
	started chan context.Context
}

func newLeading() *leading {
	return &leading{started: make(chan context.Context, 10)}
}

func (l *leading) run(ctx context.Context) error {
	l.started <- ctx
	<-ctx.Done()
	return nil
}

// next returns the context of the next run, failing the test after 5s.
func (l *leading) next(t *testing.T) context.Context {
	t.Helper()
	select {
	case ctx := <-l.started:
		return ctx
	case <-time.After(5 * time.Second):
		t.Fatal("the function did not run")
		return nil
	}
}

// newProcess returns the process of the id leading for the key "leader"
// of the locker, failing the test on error.
func newProcess(t *testing.T, id string, l flam.ILocker, run func(context.Context) error) flam.ILeaderProcess {
	t.Helper()
	p, err := leader.NewProcess(id, l, "leader", ttl, run)
	testutil.NoError(t, err)
	testutil.Equal(t, "Id()", p.Id(), id)
	return p
}

// start runs the process in a goroutine of its own, returning the channel
// of the error it returns.
func start(ctx context.Context, p flam.IWatchdogProcess) <-chan error {
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	return done
}

// result returns the error the process returned, failing the test after
// 5s.
func result(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run() did not return")
		return nil
	}
}

// waitTimers waits until the clock holds n timers.
func waitTimers(t *testing.T, clock *testutil.Clock, n int) {
	t.Helper()
	eventually(t, fmt.Sprintf("%d timers", n), func() bool { return clock.Timers() == n })
}

// eventually waits for the condition, failing the test after 5s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

// fence returns the fence of the context, failing the test without one.
func fence(t *testing.T, ctx context.Context) int64 {
	t.Helper()
	f, ok := leader.FenceOf(ctx)
	if !ok {
		t.Fatal("FenceOf() found no fence")
	}
	return f
}

// processErr returns the context of the errors of the process "p".
var processErr = map[string]any{leader.ErrorFieldID: "p", leader.ErrorFieldKey: "leader"}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "ErrorFieldID", leader.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldKey", leader.ErrorFieldKey, "key")
	for sentinel, text := range map[error]string{
		leader.ErrNilLocker:  "nil leader locker",
		leader.ErrNilRun:     "nil leader function",
		leader.ErrInvalidKey: "invalid leader key",
		leader.ErrInvalidTTL: "invalid leader ttl",
		leader.ErrRunning:    "leader process running",
	} {
		testutil.Equal(t, "sentinel", sentinel.Error(), text)
	}
}

func Test_NewProcess(t *testing.T) {
	l, _ := newLocker(t, true)
	run := newLeading().run
	for _, c := range []struct {
		locker   flam.ILocker
		run      func(context.Context) error
		key      string
		ttl      time.Duration
		sentinel error
	}{
		{nil, run, "leader", ttl, leader.ErrNilLocker},
		{l, nil, "leader", ttl, leader.ErrNilRun},
		{l, run, "", ttl, leader.ErrInvalidKey},
		{l, run, "leader", time.Millisecond - 1, leader.ErrInvalidTTL},
	} {
		p, err := leader.NewProcess("p", c.locker, c.key, c.ttl, c.run)
		testutil.Error(t, err, c.sentinel, "unable to create leader process: "+c.sentinel.Error(),
			map[string]any{leader.ErrorFieldID: "p", leader.ErrorFieldKey: c.key})
		if p != nil {
			t.Errorf("NewProcess() = %v, want nil", p)
		}
	}
}

func Test_FenceOf(t *testing.T) {
	var ctx context.Context
	f, ok := leader.FenceOf(ctx)
	testutil.Equal(t, "fence", f, int64(0))
	testutil.Equal(t, "ok", ok, false)
	f, ok = leader.FenceOf(context.Background())
	testutil.Equal(t, "fence", f, int64(0))
	testutil.Equal(t, "ok", ok, false)
}

func Test_Process(t *testing.T) {
	t.Run("should lead alone, then fail over once closed", func(t *testing.T) {
		l, clock := newLocker(t, true)
		first, second := newLeading(), newLeading()
		p1 := newProcess(t, "p1", l, first.run)
		p2 := newProcess(t, "p2", l, second.run)
		if p1.IsRunning() || p1.IsLeader() {
			t.Fatal("the process runs or leads before Run")
		}

		done1 := start(context.Background(), p1)
		ctx1 := first.next(t)
		testutil.Equal(t, "fence", fence(t, ctx1), int64(1))
		testutil.Equal(t, "IsLeader()", p1.IsLeader(), true)
		testutil.Equal(t, "IsRunning()", p1.IsRunning(), true)

		done2 := start(context.Background(), p2)
		waitTimers(t, clock, 2) // the refresh of the lock of p1, the wait of p2
		testutil.Equal(t, "IsLeader()", p2.IsLeader(), false)
		testutil.Equal(t, "IsRunning()", p2.IsRunning(), true)
		for range 3 { // p1 keeps the leadership past its ttl
			clock.Advance(time.Second)
			waitTimers(t, clock, 2)
		}
		select {
		case <-second.started:
			t.Fatal("p2 leads along with p1")
		default:
		}

		testutil.NoError(t, p1.Close())
		testutil.NoError(t, p1.Close())
		testutil.NoError(t, result(t, done1))
		if ctx1.Err() == nil {
			t.Fatal("the context of the function of p1 is not done")
		}
		testutil.Equal(t, "IsLeader()", p1.IsLeader(), false)
		testutil.Equal(t, "IsRunning()", p1.IsRunning(), false)
		testutil.NoError(t, p1.Run(context.Background())) // returns at once once closed

		waitTimers(t, clock, 1) // the lock of p1 released
		clock.Advance(time.Second)
		ctx2 := second.next(t)
		testutil.Equal(t, "fence", fence(t, ctx2), int64(2))
		testutil.Equal(t, "IsLeader()", p2.IsLeader(), true)
		testutil.NoError(t, p2.Close())
		testutil.NoError(t, result(t, done2))
	})

	t.Run("should campaign again once the leadership is lost", func(t *testing.T) {
		l, clock := newLocker(t, false)
		leading := newLeading()
		p := newProcess(t, "p", l, leading.run)
		ctx, cancel := context.WithCancel(context.Background())
		done := start(ctx, p)

		first := leading.next(t)
		testutil.Equal(t, "fence", fence(t, first), int64(1))
		waitTimers(t, clock, 1)
		clock.Advance(ttl) // the lock is not refreshed
		second := leading.next(t)
		if first.Err() == nil {
			t.Fatal("the context of the lost leadership is not done")
		}
		testutil.Equal(t, "fence", fence(t, second), int64(2))
		eventually(t, "the leadership", p.IsLeader)

		cancel()
		testutil.NoError(t, result(t, done))
		testutil.Equal(t, "IsLeader()", p.IsLeader(), false)
	})

	t.Run("should take a nil context as never done", func(t *testing.T) {
		l, _ := newLocker(t, true)
		leading := newLeading()
		p := newProcess(t, "p", l, leading.run)
		var ctx context.Context
		done := start(ctx, p)
		leading.next(t)
		testutil.NoError(t, p.Close())
		testutil.NoError(t, result(t, done))
	})

	t.Run("should return the error of the function, releasing the lock", func(t *testing.T) {
		l, _ := newLocker(t, true)
		p := newProcess(t, "p", l, func(ctx context.Context) error {
			if _, ok := leader.FenceOf(ctx); !ok {
				return errors.New("no fence")
			}
			return errBoom
		})
		err := p.Run(context.Background())
		if !errors.Is(err, errBoom) {
			t.Fatalf("Run() = %v, want errBoom", err)
		}
		testutil.Equal(t, "IsLeader()", p.IsLeader(), false)
		held, ok, err := l.TryAcquire(context.Background(), "leader", ttl)
		testutil.NoError(t, err)
		testutil.Equal(t, "ok", ok, true)
		testutil.Equal(t, "Fence()", held.Fence(), int64(2))
	})

	t.Run("should release the lock when the function panics", func(t *testing.T) {
		l, _ := newLocker(t, true)
		p := newProcess(t, "p", l, func(context.Context) error { panic(errBoom) })
		testutil.Panics(t, "Run()", func() { _ = p.Run(context.Background()) })
		testutil.Equal(t, "IsLeader()", p.IsLeader(), false)
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
		_, ok, err := l.TryAcquire(context.Background(), "leader", ttl)
		testutil.NoError(t, err)
		testutil.Equal(t, "ok", ok, true)
	})

	t.Run("should fail when running already", func(t *testing.T) {
		l, _ := newLocker(t, true)
		leading := newLeading()
		p := newProcess(t, "p", l, leading.run)
		done := start(context.Background(), p)
		leading.next(t)
		err := p.Run(context.Background())
		testutil.Error(t, err, leader.ErrRunning, "unable to run leader process: leader process running", processErr)
		testutil.NoError(t, p.Close())
		testutil.NoError(t, result(t, done))
	})

	t.Run("should return the errors of the locker", func(t *testing.T) {
		l, _ := newLocker(t, true)
		testutil.NoError(t, l.Close())
		p := newProcess(t, "p", l, newLeading().run)
		err := p.Run(context.Background())
		testutil.Error(t, err, lock.ErrClosed, "unable to acquire lock: locker closed",
			map[string]any{lock.ErrorFieldID: "main", lock.ErrorFieldKey: "leader"})
		testutil.Equal(t, "IsRunning()", p.IsRunning(), false)
	})
}
