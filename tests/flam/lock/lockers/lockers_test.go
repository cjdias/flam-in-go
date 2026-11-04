package lockers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/redis"
	flamredis "github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// now is the time of the clocks of the tests.
var now = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

// ttl is the ttl of the locks of the tests, a third of it being a second.
const ttl = 3 * time.Second

// newApp returns a testutil.App with the config, lock and locker driver
// bundles, the redis ones and a mini connection "main" without retries
// when withRedis, the clock, unless nil, and the values.
func newApp(t *testing.T, values map[string]any, clock flam.IClock, withRedis bool) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle()}
	if withRedis {
		bundles = append(bundles, flamredis.NewBundle(), mini.NewBundle())
	}
	bundles = append(bundles, lock.NewBundle(), lockers.NewBundle())
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(flamredis.ConnectionConfigPath+".main", container.Bag{
		factory.ConfigFieldDriver:       mini.Driver,
		flamredis.ConfigFieldMaxRetries: -1,
	}))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	if clock != nil {
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = lock.NewBundle().(flam.IClosableBundle).Close(app)
		if withRedis {
			_ = flamredis.NewBundle().(flam.IClosableBundle).Close(app)
		}
	})
	return app
}

// newLocker returns the locker "main" of the fields, created by the
// factory of an app with the clock, unless nil, the redis bundles when
// withRedis and the values.
func newLocker(
	t *testing.T, fields container.Bag, values map[string]any, clock flam.IClock, withRedis bool,
) (flam.ILocker, error) {
	t.Helper()
	all := map[string]any{lock.LockerConfigPath + ".main": fields}
	for path, value := range values {
		all[path] = value
	}
	app := newApp(t, all, clock, withRedis)
	var l flam.ILocker
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory) { l, err = f.Get("main") }))
	return l, err
}

// harness holds two lockers of a driver sharing their locks, the fake
// clock they read, and, for the redis driver, the server of their
// connection.
type harness struct {
	clock  *testutil.Clock
	server *miniredis.Miniredis // nil for the memory driver
	first  flam.ILocker
	second flam.ILocker // the first one for the memory driver
}

// drivers are the drivers the shared tests run with.
var drivers = []string{memory.Driver, redis.Driver}

// newHarness returns the harness of the driver, whose lockers wait 1s
// without jitter to try again, and do not refresh their locks on their
// own, unless the fields say otherwise.
func newHarness(t *testing.T, driver string, fields container.Bag) *harness {
	t.Helper()
	all := container.Bag{
		factory.ConfigFieldDriver:     driver,
		lock.ConfigFieldRetryInterval: "1s",
		lock.ConfigFieldRetryJitter:   "0s",
		lock.ConfigFieldAutoRefresh:   false,
		redis.ConfigFieldConnection:   "main",
		redis.ConfigFieldPrefix:       "test:",
	}
	for field, value := range fields {
		all[field] = value
	}
	h := &harness{clock: testutil.NewClock(now)}
	app := newApp(t, map[string]any{
		lock.LockerConfigPath + ".first":  all,
		lock.LockerConfigPath + ".second": all,
	}, h.clock, driver == redis.Driver)
	testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory) error {
		var err error
		h.first, err = f.Get("first")
		h.second = h.first
		return err
	}))
	if driver == redis.Driver {
		testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory, cf flam.IRedisConnectionFactory) error {
			var err error
			if h.second, err = f.Get("second"); err != nil {
				return err
			}
			conn, err := cf.Get("main")
			h.server, _ = mini.Server(conn)
			return err
		}))
	}
	return h
}

// autoRefresh are the fields of the lockers refreshing their locks.
var autoRefresh = container.Bag{lock.ConfigFieldAutoRefresh: true}

// expireBackend makes the backend expire the lock, without the redis
// locker knowing, its clock being left as is, while the memory locker,
// reading the same clock as its backend, ends it.
func expireBackend(t *testing.T, h *harness, l flam.ILock) {
	t.Helper()
	if h.server != nil {
		h.server.FastForward(ttl)
		return
	}
	h.clock.Advance(ttl)
	waitLost(t, l)
}

// expire makes the backend expire the keys whose ttl elapsed after d, as
// the memory lockers read the fake clock, and the redis server its own.
func (h *harness) expire(d time.Duration) {
	if h.server != nil {
		h.server.FastForward(d)
	}
	h.clock.Advance(d)
}

// waitTimers waits until the clock holds n timers, as the lock watches and
// the waits of Acquire create them from goroutines of their own.
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

// isLost reports whether the lost channel of the lock is closed.
func isLost(l flam.ILock) bool {
	select {
	case <-l.Lost():
		return true
	default:
		return false
	}
}

// waitLost waits until the lock is lost.
func waitLost(t *testing.T, l flam.ILock) {
	t.Helper()
	select {
	case <-l.Lost():
	case <-time.After(5 * time.Second):
		t.Fatalf("lock %q not lost", l.Key())
	}
}

// advanceUntilLost waits until the lock is lost, advancing the time a
// second at a time, as the watch of a lock refreshed by the test may arm
// its timer from a time read before the last advance of the clock.
func advanceUntilLost(t *testing.T, h *harness, l flam.ILock) {
	t.Helper()
	for range 5 {
		select {
		case <-l.Lost():
			return
		case <-time.After(50 * time.Millisecond):
			h.expire(time.Second)
		}
	}
	waitLost(t, l)
}

// tryAcquire acquires the key of the locker for the ttl, failing the test
// unless acquired.
func tryAcquire(t *testing.T, l flam.ILocker, key string) flam.ILock {
	t.Helper()
	held, ok, err := l.TryAcquire(context.Background(), key, ttl)
	testutil.NoError(t, err)
	if !ok || held == nil {
		t.Fatalf("TryAcquire(%q) = %v, %v, want a lock", key, held, ok)
	}
	return held
}

// notAcquired fails the test unless the key of the locker is held by
// another.
func notAcquired(t *testing.T, l flam.ILocker, key string) {
	t.Helper()
	held, ok, err := l.TryAcquire(context.Background(), key, ttl)
	testutil.NoError(t, err)
	if ok || held != nil {
		t.Fatalf("TryAcquire(%q) = %v, %v, want none", key, held, ok)
	}
}

// keyErr returns the context of the errors of a key of a locker.
func keyErr(id, key string) map[string]any {
	return map[string]any{lock.ErrorFieldID: id, lock.ErrorFieldKey: key}
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "LockerConfigPath", lock.LockerConfigPath, "flam.lock.lockers")
	testutil.Equal(t, "LockerCreatorGroup", lock.LockerCreatorGroup, "flam.lock.lockers.creator")
	testutil.Equal(t, "ConfigFieldRetryInterval", lock.ConfigFieldRetryInterval, "retry_interval")
	testutil.Equal(t, "ConfigFieldRetryJitter", lock.ConfigFieldRetryJitter, "retry_jitter")
	testutil.Equal(t, "ConfigFieldAutoRefresh", lock.ConfigFieldAutoRefresh, "auto_refresh")
	testutil.Equal(t, "DefaultRetryIntervalConfigPath", lock.DefaultRetryIntervalConfigPath, "flam.lock.defaults.lockers.retry_interval")
	testutil.Equal(t, "DefaultRetryJitterConfigPath", lock.DefaultRetryJitterConfigPath, "flam.lock.defaults.lockers.retry_jitter")
	testutil.Equal(t, "DefaultAutoRefreshConfigPath", lock.DefaultAutoRefreshConfigPath, "flam.lock.defaults.lockers.auto_refresh")
	testutil.Equal(t, "DefaultRetryInterval", lock.DefaultRetryInterval, 100*time.Millisecond)
	testutil.Equal(t, "DefaultRetryJitter", lock.DefaultRetryJitter, 50*time.Millisecond)
	testutil.Equal(t, "DefaultAutoRefresh", lock.DefaultAutoRefresh, true)
	testutil.Equal(t, "ErrorFieldID", lock.ErrorFieldID, "id")
	testutil.Equal(t, "ErrorFieldKey", lock.ErrorFieldKey, "key")
	for sentinel, text := range map[error]string{
		lock.ErrInvalidKey:  "invalid lock key",
		lock.ErrInvalidTTL:  "invalid lock ttl",
		lock.ErrNotAcquired: "lock not acquired",
		lock.ErrNotHeld:     "lock not held",
		lock.ErrStorage:     "lock storage failure",
		lock.ErrClosed:      "locker closed",
	} {
		testutil.Equal(t, "sentinel", sentinel.Error(), text)
	}
}

func Test_Bundles(t *testing.T) {
	t.Run("should provide a single locker factory, closed with the bundle", func(t *testing.T) {
		b := lock.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.lock")
		if b == lock.NewBundle() {
			t.Fatal("NewBundle() returned the same bundle twice")
		}

		clock := testutil.NewClock(now)
		app := newApp(t, map[string]any{
			lock.LockerConfigPath + ".main": container.Bag{factory.ConfigFieldDriver: memory.Driver},
		}, clock, false)
		var first, second flam.ILockerFactory
		testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory) { first = f }))
		testutil.NoError(t, app.Invoke(func(f flam.ILockerFactory) { second = f }))
		if first == nil || first != second {
			t.Fatal("the container did not provide a single ILockerFactory")
		}
		testutil.Equal(t, "Available()", first.Available(), []string{"main"})
		l, err := first.Get("main")
		testutil.NoError(t, err)
		held := tryAcquire(t, l, "job")

		testutil.NoError(t, b.(flam.IClosableBundle).Close(app))
		if !isLost(held) {
			t.Error("closing the bundle did not end the lock")
		}
		if _, err := first.Get("main"); !errors.Is(err, factory.ErrClosed) {
			t.Errorf("Get() = %v, want factory.ErrClosed", err)
		}
	})

	t.Run("should register its provider", func(t *testing.T) {
		app := testutil.NewApp()
		testutil.NoError(t, lock.NewBundle().Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0"})
	})

	t.Run("should fail to close when the factory cannot be created", func(t *testing.T) {
		app := testutil.NewApp()
		b := lock.NewBundle()
		testutil.NoError(t, b.Register(app))
		if err := b.(flam.IClosableBundle).Close(app); err == nil {
			t.Fatal("Close() did not fail without configuration")
		}
	})

	t.Run("should register every locker driver", func(t *testing.T) {
		b := lockers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.lock.lockers")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.lock.lockers", "bundle:flam.lock.lockers.redis", "provider/1",
			"bundle:flam.lock.lockers.memory", "provider/1",
		})
	})

	t.Run("should provide the creators", func(t *testing.T) {
		for id, b := range map[string]flam.IBundle{
			"flam.lock.lockers.redis":  redis.NewBundle(),
			"flam.lock.lockers.memory": memory.NewBundle(),
		} {
			testutil.Equal(t, "Id()", b.Id(), id)
			app := testutil.NewApp()
			testutil.NoError(t, app.RegisterBundle(b))
			testutil.Equal(t, "calls", app.Calls(), []string{"bundle:" + id, "provider/1"})
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, testutil.NewClock(now), true)
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_Locker(t *testing.T) {
	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			t.Run("should acquire a free key with a unique token and an increasing fence", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				first := tryAcquire(t, h.first, "job")
				testutil.Equal(t, "Key()", first.Key(), "job")
				testutil.Equal(t, "Fence()", first.Fence(), int64(1))
				if first.Token() == "" {
					t.Fatal("Token() is empty")
				}
				if isLost(first) {
					t.Fatal("the lock is lost once acquired")
				}
				notAcquired(t, h.second, "job")
				other := tryAcquire(t, h.second, "other")
				testutil.Equal(t, "Fence()", other.Fence(), int64(1))

				testutil.NoError(t, first.Release(context.Background()))
				if !isLost(first) {
					t.Fatal("the lock is not ended once released")
				}
				second := tryAcquire(t, h.second, "job")
				testutil.Equal(t, "Fence()", second.Fence(), int64(2))
				if second.Token() == first.Token() {
					t.Fatal("two acquisitions have the same token")
				}
				testutil.NoError(t, second.Release(context.Background()))
				third := tryAcquire(t, h.first, "job")
				testutil.Equal(t, "Fence()", third.Fence(), int64(3))
			})

			t.Run("should take a nil context as context.Background", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				var ctx context.Context
				held, ok, err := h.first.TryAcquire(ctx, "job", ttl)
				testutil.NoError(t, err)
				testutil.Equal(t, "ok", ok, true)
				testutil.NoError(t, held.Refresh(ctx, ttl))
				testutil.NoError(t, held.Release(ctx))
				held, err = h.first.Acquire(ctx, "job", ttl)
				testutil.NoError(t, err)
				testutil.Equal(t, "Fence()", held.Fence(), int64(2))
			})

			t.Run("should fail with an invalid key or ttl", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				_, ok, err := h.first.TryAcquire(context.Background(), "", ttl)
				testutil.Error(t, err, lock.ErrInvalidKey, "unable to try to acquire lock: invalid lock key", keyErr("first", ""))
				testutil.Equal(t, "ok", ok, false)
				_, err = h.first.Acquire(context.Background(), "job", time.Millisecond-1)
				testutil.Error(t, err, lock.ErrInvalidTTL, "unable to acquire lock: invalid lock ttl", keyErr("first", "job"))

				held := tryAcquire(t, h.first, "job")
				err = held.Refresh(context.Background(), 0)
				testutil.Error(t, err, lock.ErrInvalidTTL, "unable to refresh lock: invalid lock ttl", keyErr("first", "job"))
				if isLost(held) {
					t.Fatal("an invalid refresh ended the lock")
				}
			})

			t.Run("should wait for the key to be released", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				first := tryAcquire(t, h.first, "job")
				waitTimers(t, h.clock, 1) // the watch of the lock

				type result struct {
					lock flam.ILock
					err  error
				}
				done := make(chan result, 1)
				go func() {
					l, err := h.second.Acquire(context.Background(), "job", ttl)
					done <- result{l, err}
				}()
				waitTimers(t, h.clock, 2) // and the wait of Acquire
				h.clock.Advance(time.Second)
				waitTimers(t, h.clock, 2) // waiting again, as still held
				testutil.NoError(t, first.Release(context.Background()))
				waitTimers(t, h.clock, 1) // the watch of the first lock ended
				h.clock.Advance(time.Second)

				r := <-done
				testutil.NoError(t, r.err)
				testutil.Equal(t, "Fence()", r.lock.Fence(), int64(2))
				testutil.Equal(t, "Key()", r.lock.Key(), "job")
			})

			t.Run("should stop waiting once the context is done", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				tryAcquire(t, h.first, "job")
				waitTimers(t, h.clock, 1)

				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() {
					l, err := h.second.Acquire(ctx, "job", ttl)
					if l != nil {
						err = errors.New("acquired")
					}
					done <- err
				}()
				waitTimers(t, h.clock, 2)
				cancel()
				err := <-done
				testutil.Error(t, err, lock.ErrNotAcquired, "unable to acquire lock: lock not acquired: context canceled", keyErr(h.second.Id(), "job"))
				if !errors.Is(err, context.Canceled) {
					t.Errorf("error = %v, want it to wrap context.Canceled", err)
				}
				waitTimers(t, h.clock, 1) // the wait stopped its timer

				_, err = h.second.Acquire(ctx, "job", ttl)
				testutil.Error(t, err, lock.ErrNotAcquired, "unable to acquire lock: lock not acquired: context canceled", keyErr(h.second.Id(), "job"))
			})

			t.Run("should wait with a random jitter", func(t *testing.T) {
				h := newHarness(t, driver, container.Bag{lock.ConfigFieldRetryJitter: "500ms"})
				l := h.first
				first := tryAcquire(t, l, "job")
				waitTimers(t, h.clock, 1)

				done := make(chan error, 1)
				go func() {
					_, err := l.Acquire(context.Background(), "job", ttl)
					done <- err
				}()
				waitTimers(t, h.clock, 2)
				testutil.NoError(t, first.Release(context.Background()))
				waitTimers(t, h.clock, 1)
				h.clock.Advance(1500 * time.Millisecond)
				testutil.NoError(t, <-done)
			})

			t.Run("should be lost once its ttl elapsed unless refreshed", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				held := tryAcquire(t, h.first, "job")
				waitTimers(t, h.clock, 1)
				h.expire(2 * time.Second)
				testutil.NoError(t, held.Refresh(context.Background(), ttl)) // until 5s
				h.expire(2 * time.Second)
				if isLost(held) {
					t.Fatal("the lock is lost before the ttl of its refresh elapsed")
				}
				notAcquired(t, h.second, "job")
				h.expire(time.Second)
				advanceUntilLost(t, h, held)

				err := held.Refresh(context.Background(), ttl)
				testutil.Error(t, err, lock.ErrNotHeld, "unable to refresh lock: lock not held", keyErr("first", "job"))
				err = held.Release(context.Background())
				testutil.Error(t, err, lock.ErrNotHeld, "unable to release lock: lock not held", keyErr("first", "job"))
				next := tryAcquire(t, h.second, "job")
				testutil.Equal(t, "Fence()", next.Fence(), int64(2))
			})

			t.Run("should only refresh or release the lock of its token", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				first := tryAcquire(t, h.first, "job")
				expireBackend(t, h, first)
				second := tryAcquire(t, h.second, "job")
				testutil.Equal(t, "Fence()", second.Fence(), int64(2))

				err := first.Release(context.Background())
				testutil.Error(t, err, lock.ErrNotHeld, "unable to release lock: lock not held", keyErr("first", "job"))
				if !isLost(first) {
					t.Fatal("the lock is not lost")
				}
				err = first.Refresh(context.Background(), ttl)
				testutil.Error(t, err, lock.ErrNotHeld, "unable to refresh lock: lock not held", keyErr("first", "job"))
				notAcquired(t, h.first, "job")
				testutil.NoError(t, second.Refresh(context.Background(), ttl))
				testutil.NoError(t, second.Release(context.Background()))
			})

			t.Run("should be lost when a refresh finds it expired", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				held := tryAcquire(t, h.first, "job")
				expireBackend(t, h, held)
				err := held.Refresh(context.Background(), ttl)
				testutil.Error(t, err, lock.ErrNotHeld, "unable to refresh lock: lock not held", keyErr("first", "job"))
				if !isLost(held) {
					t.Fatal("the lock is not lost")
				}
			})

			t.Run("should be lost when a release finds it expired", func(t *testing.T) {
				h := newHarness(t, driver, nil)
				held := tryAcquire(t, h.first, "job")
				expireBackend(t, h, held)
				err := held.Release(context.Background())
				testutil.Error(t, err, lock.ErrNotHeld, "unable to release lock: lock not held", keyErr("first", "job"))
				if !isLost(held) {
					t.Fatal("the lock is not lost")
				}
			})

			t.Run("should refresh its locks on its own", func(t *testing.T) {
				h := newHarness(t, driver, autoRefresh)
				held := tryAcquire(t, h.first, "job")
				for range 6 { // 6s, twice the ttl
					waitTimers(t, h.clock, 1)
					h.expire(time.Second)
				}
				waitTimers(t, h.clock, 1)
				if isLost(held) {
					t.Fatal("the lock refreshed on its own is lost")
				}
				notAcquired(t, h.second, "job")
				testutil.NoError(t, held.Release(context.Background()))
				waitTimers(t, h.clock, 0)
				tryAcquire(t, h.second, "job")
			})

			t.Run("should keep refreshing with the ttl of the last refresh", func(t *testing.T) {
				h := newHarness(t, driver, autoRefresh)
				held := tryAcquire(t, h.first, "job")
				waitTimers(t, h.clock, 1)
				testutil.NoError(t, held.Refresh(context.Background(), 30*time.Second))
				h.expire(4 * time.Second) // past the first ttl, the watch waiting 10s
				waitTimers(t, h.clock, 1)
				h.expire(8 * time.Second) // and a refresh
				notAcquired(t, h.second, "job")
				if isLost(held) {
					t.Fatal("the lock is lost before the ttl of its refresh elapsed")
				}
			})

			t.Run("should release its locks when closed", func(t *testing.T) {
				h := newHarness(t, driver, autoRefresh)
				l := h.first
				first := tryAcquire(t, l, "a")
				second := tryAcquire(t, l, "b")
				released := tryAcquire(t, l, "c")
				testutil.NoError(t, released.Release(context.Background()))

				testutil.NoError(t, l.Close())
				if !isLost(first) || !isLost(second) {
					t.Fatal("closing the locker did not end its locks")
				}
				testutil.NoError(t, l.Close())
				waitTimers(t, h.clock, 0)

				_, ok, err := l.TryAcquire(context.Background(), "a", ttl)
				testutil.Error(t, err, lock.ErrClosed, "unable to try to acquire lock: locker closed", keyErr("first", "a"))
				testutil.Equal(t, "ok", ok, false)
				_, err = l.Acquire(context.Background(), "a", ttl)
				testutil.Error(t, err, lock.ErrClosed, "unable to acquire lock: locker closed", keyErr("first", "a"))
				err = first.Release(context.Background())
				testutil.Error(t, err, lock.ErrNotHeld, "unable to release lock: lock not held", keyErr("first", "a"))
			})
		})
	}
}
