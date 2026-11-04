package lockers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/redis"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Redis(t *testing.T) {
	t.Run("should declare the driver, fields and paths", func(t *testing.T) {
		testutil.Equal(t, "Driver", redis.Driver, "flam.lock.lockers.driver.redis")
		testutil.Equal(t, "ConfigFieldConnection", redis.ConfigFieldConnection, "connection")
		testutil.Equal(t, "ConfigFieldPrefix", redis.ConfigFieldPrefix, "prefix")
		testutil.Equal(t, "DefaultConnectionConfigPath", redis.DefaultConnectionConfigPath, "flam.lock.defaults.lockers.redis.connection")
		testutil.Equal(t, "DefaultPrefixConfigPath", redis.DefaultPrefixConfigPath, "flam.lock.defaults.lockers.redis.prefix")
		testutil.Equal(t, "FenceSuffix", redis.FenceSuffix, ":fence")
		testutil.Equal(t, "ErrorFieldField", redis.ErrorFieldField, "field")
		testutil.Equal(t, "ErrorFieldConnection", redis.ErrorFieldConnection, "connection")
		for sentinel, text := range map[error]string{
			redis.ErrInvalidField: "invalid redis locker field",
			redis.ErrConnection:   "unable to get redis locker connection",
			redis.ErrClock:        "unable to get redis locker clock",
		} {
			testutil.Equal(t, "sentinel", sentinel.Error(), text)
		}
	})

	t.Run("should hold the locks under the keys of the prefix", func(t *testing.T) {
		h := newHarness(t, redis.Driver, nil)
		held := tryAcquire(t, h.first, "job")
		value, err := h.server.Get("test:job")
		testutil.NoError(t, err)
		testutil.Equal(t, "value", value, held.Token())
		testutil.Equal(t, "TTL", h.server.TTL("test:job"), ttl)
		fence, err := h.server.Get("test:job:fence")
		testutil.NoError(t, err)
		testutil.Equal(t, "fence", fence, "1")
		testutil.Equal(t, "fence TTL", h.server.TTL("test:job:fence"), time.Duration(0))

		h.server.FastForward(2 * time.Second)
		testutil.NoError(t, held.Refresh(context.Background(), 5*time.Second))
		testutil.Equal(t, "TTL", h.server.TTL("test:job"), 5*time.Second)
		testutil.NoError(t, held.Release(context.Background()))
		testutil.Equal(t, "Exists", h.server.Exists("test:job"), false)
		testutil.Equal(t, "fence Exists", h.server.Exists("test:job:fence"), true)
	})

	t.Run("should create the locker of the defaults", func(t *testing.T) {
		clock := testutil.NewClock(now)
		l, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: redis.Driver}, map[string]any{
			redis.DefaultConnectionConfigPath: "main",
			redis.DefaultPrefixConfigPath:     "",
		}, clock, true)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", l.Id(), "main")
		tryAcquire(t, l, "job")
		notAcquired(t, l, "job")

		_, err = newLocker(t, container.Bag{factory.ConfigFieldDriver: redis.Driver}, nil, clock, true)
		if !errors.Is(err, redis.ErrConnection) || !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want redis.ErrConnection and factory.ErrUnknownID", err)
		}
		var e *flam.Error
		errors.As(errors.Unwrap(err), &e)
		testutil.Context(t, e, map[string]any{lock.ErrorFieldID: "main", redis.ErrorFieldConnection: "default"})
	})

	t.Run("should contend with the lockers sharing the server", func(t *testing.T) {
		h := newHarness(t, redis.Driver, container.Bag{redis.ConfigFieldPrefix: "shared:"})
		var fences []int64
		for i := range 4 {
			l := h.first
			if i%2 == 1 {
				l = h.second
			}
			held := tryAcquire(t, l, "job")
			if i%2 == 0 {
				notAcquired(t, h.second, "job")
			} else {
				notAcquired(t, h.first, "job")
			}
			fences = append(fences, held.Fence())
			testutil.NoError(t, held.Release(context.Background()))
		}
		testutil.Equal(t, "fences", fences, []int64{1, 2, 3, 4})
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, value := range map[string]any{
			redis.ConfigFieldConnection:   "",
			redis.ConfigFieldPrefix:       1,
			lock.ConfigFieldRetryInterval: "x",
		} {
			_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: redis.Driver, field: value}, nil, testutil.NewClock(now), true)
			testutil.Error(t, errors.Unwrap(err), redis.ErrInvalidField, "unable to create redis locker: invalid redis locker field",
				map[string]any{lock.ErrorFieldID: "main", redis.ErrorFieldField: field})
		}
	})

	t.Run("should fail without a clock or a connection factory", func(t *testing.T) {
		_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: redis.Driver}, nil, nil, true)
		testutil.Error(t, errors.Unwrap(err), redis.ErrClock, "unable to create redis locker: unable to get redis locker clock",
			map[string]any{lock.ErrorFieldID: "main"})
		_, err = newLocker(t, container.Bag{factory.ConfigFieldDriver: redis.Driver}, nil, testutil.NewClock(now), false)
		testutil.Error(t, errors.Unwrap(err), redis.ErrConnection, "unable to create redis locker: unable to get redis locker connection: "+
			"no redis connection factory, register the redis bundle",
			map[string]any{lock.ErrorFieldID: "main", redis.ErrorFieldConnection: "default"})
	})

	t.Run("should fail when the server fails", func(t *testing.T) {
		h := newHarness(t, redis.Driver, nil)
		held := tryAcquire(t, h.first, "job")
		h.server.Close()

		_, ok, err := h.first.TryAcquire(context.Background(), "other", ttl)
		storageErr(t, err, "unable to try to acquire lock", "other")
		testutil.Equal(t, "ok", ok, false)
		_, err = h.first.Acquire(context.Background(), "other", ttl)
		storageErr(t, err, "unable to acquire lock", "other")
		storageErr(t, held.Refresh(context.Background(), ttl), "unable to refresh lock", "job")
		storageErr(t, held.Release(context.Background()), "unable to release lock", "job")
		if isLost(held) {
			t.Fatal("a failed release ended the lock")
		}
		storageErr(t, h.first.Close(), "unable to release lock on close", "job")
		if !isLost(held) {
			t.Fatal("closing the locker did not end the lock")
		}
	})

	t.Run("should keep trying to refresh until the ttl elapsed", func(t *testing.T) {
		h := newHarness(t, redis.Driver, autoRefresh)
		held := tryAcquire(t, h.first, "job")
		h.server.Close()
		for range 2 { // the refreshes at 1s and 2s fail
			waitTimers(t, h.clock, 1)
			h.clock.Advance(time.Second)
		}
		waitTimers(t, h.clock, 1)
		if isLost(held) {
			t.Fatal("the lock is lost before its ttl elapsed")
		}
		h.clock.Advance(time.Second)
		waitLost(t, held)
	})

	t.Run("should lose a lock its automatic refresh finds removed", func(t *testing.T) {
		h := newHarness(t, redis.Driver, autoRefresh)
		held := tryAcquire(t, h.first, "job")
		waitTimers(t, h.clock, 1)
		h.server.Del("test:job")
		h.clock.Advance(time.Second)
		waitLost(t, held)
	})
}

// storageErr fails the test unless the error wraps lock.ErrStorage with
// the message, for the key of the locker "first".
func storageErr(t *testing.T, err error, message, key string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: error = nil, want lock.ErrStorage", message)
	}
	if !errors.Is(err, lock.ErrStorage) {
		t.Fatalf("error = %v, want it to wrap lock.ErrStorage", err)
	}
	testutil.Error(t, err, lock.ErrStorage, err.Error(), keyErr("first", key))
	if want := message + ": lock storage failure: "; len(err.Error()) <= len(want) || err.Error()[:len(want)] != want {
		t.Errorf("Error() = %q, want it to start with %q", err.Error(), want)
	}
}
