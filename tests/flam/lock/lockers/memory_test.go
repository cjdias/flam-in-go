package lockers

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/memory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Memory(t *testing.T) {
	t.Run("should declare the driver and fields", func(t *testing.T) {
		testutil.Equal(t, "Driver", memory.Driver, "flam.lock.lockers.driver.memory")
		testutil.Equal(t, "ErrorFieldField", memory.ErrorFieldField, "field")
		testutil.Equal(t, "ErrInvalidField", memory.ErrInvalidField.Error(), "invalid memory locker field")
		testutil.Equal(t, "ErrClock", memory.ErrClock.Error(), "unable to get memory locker clock")
	})

	t.Run("should create the locker of the defaults", func(t *testing.T) {
		clock := testutil.NewClock(now)
		l, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: memory.Driver}, map[string]any{
			lock.DefaultRetryIntervalConfigPath: "2s",
			lock.DefaultRetryJitterConfigPath:   time.Duration(0),
			lock.DefaultAutoRefreshConfigPath:   false,
		}, clock, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", l.Id(), "main")

		held := tryAcquire(t, l, "job")
		done := make(chan error, 1)
		go func() {
			_, err := l.Acquire(context.Background(), "job", ttl)
			done <- err
		}()
		waitTimers(t, clock, 2)
		testutil.NoError(t, held.Release(context.Background()))
		waitTimers(t, clock, 1)
		clock.Advance(time.Second) // short of the 2s interval
		select {
		case err := <-done:
			t.Fatalf("Acquire() = %v before the retry interval elapsed", err)
		default:
		}
		clock.Advance(time.Second)
		testutil.NoError(t, <-done)
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for field, value := range map[string]any{
			lock.ConfigFieldRetryInterval: "0s",
			lock.ConfigFieldRetryJitter:   "-1s",
			lock.ConfigFieldAutoRefresh:   "yes",
		} {
			_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: memory.Driver, field: value}, nil, testutil.NewClock(now), false)
			testutil.Error(t, errors.Unwrap(err), memory.ErrInvalidField, "unable to create memory locker: invalid memory locker field",
				map[string]any{lock.ErrorFieldID: "main", memory.ErrorFieldField: field})
		}
		_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: memory.Driver, lock.ConfigFieldRetryInterval: "x"}, nil, testutil.NewClock(now), false)
		testutil.Error(t, errors.Unwrap(err), memory.ErrInvalidField, "unable to create memory locker: invalid memory locker field",
			map[string]any{lock.ErrorFieldID: "main", memory.ErrorFieldField: lock.ConfigFieldRetryInterval})
	})

	t.Run("should fail without a clock", func(t *testing.T) {
		_, err := newLocker(t, container.Bag{factory.ConfigFieldDriver: memory.Driver}, nil, nil, false)
		testutil.Error(t, errors.Unwrap(err), memory.ErrClock, "unable to create memory locker: unable to get memory locker clock",
			map[string]any{lock.ErrorFieldID: "main"})
	})

	t.Run("should fail with a done context", func(t *testing.T) {
		h := newHarness(t, memory.Driver, nil)
		held := tryAcquire(t, h.first, "job")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, ok, err := h.first.TryAcquire(ctx, "other", ttl)
		testutil.Error(t, err, lock.ErrStorage, "unable to try to acquire lock: lock storage failure: context canceled", keyErr("first", "other"))
		testutil.Equal(t, "ok", ok, false)
		err = held.Refresh(ctx, ttl)
		testutil.Error(t, err, lock.ErrStorage, "unable to refresh lock: lock storage failure: context canceled", keyErr("first", "job"))
		err = held.Release(ctx)
		testutil.Error(t, err, lock.ErrStorage, "unable to release lock: lock storage failure: context canceled", keyErr("first", "job"))
		if isLost(held) {
			t.Fatal("a failed release ended the lock")
		}
		testutil.NoError(t, held.Release(context.Background()))
	})

	t.Run("should lose a lock its automatic refresh finds expired", func(t *testing.T) {
		h := newHarness(t, memory.Driver, autoRefresh)
		held := tryAcquire(t, h.first, "job")
		waitTimers(t, h.clock, 1)
		h.clock.Advance(10 * time.Second) // the refresh due at 1s reads 10s
		waitLost(t, held)
		testutil.Equal(t, "Fence()", tryAcquire(t, h.first, "job").Fence(), int64(2))
	})
}
