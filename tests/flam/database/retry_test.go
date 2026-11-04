package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"

	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// safeToRetryError is an error of pgconn telling whether it is safe to
// retry.
type safeToRetryError bool

func (e safeToRetryError) Error() string     { return "pgconn" }
func (e safeToRetryError) SafeToRetry() bool { return bool(e) }

var errTransient = &pgconn.PgError{Code: "40001", Message: "could not serialize access"}

// attempts is a function to retry, recording its calls, failing with the
// errors in order, then succeeding.
type attempts struct {
	calls int
	errs  []error
}

func (a *attempts) fn(context.Context) error {
	a.calls++
	if a.calls <= len(a.errs) {
		return a.errs[a.calls-1]
	}
	return nil
}

// retryAsync runs Retry on its own goroutine, returning the channel of its
// result.
func retryAsync(ctx context.Context, fn func(context.Context) error, opts database.RetryOptions) <-chan error {
	done := make(chan error, 1)
	go func() { done <- database.Retry(ctx, fn, opts) }()
	return done
}

func Test_Retry(t *testing.T) {
	t.Run("should fail without function or clock", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		testutil.Error(t, database.Retry(context.Background(), nil, database.RetryOptions{Clock: clock}),
			database.ErrNilRetryFunc, "unable to retry: nil function to retry", map[string]any{})
		testutil.Error(t, database.Retry(context.Background(), (&attempts{}).fn, database.RetryOptions{}),
			database.ErrNilClock, "unable to retry: nil clock", map[string]any{})
	})

	t.Run("should call the function once when it succeeds", func(t *testing.T) {
		a := &attempts{}
		testutil.NoError(t, database.Retry(context.Background(), a.fn, database.RetryOptions{Clock: testutil.NewClock(time.Unix(0, 0))}))
		testutil.Equal(t, "calls", a.calls, 1)
	})

	t.Run("should return an error that is not transient unchanged", func(t *testing.T) {
		errOther := errors.New("other")
		a := &attempts{errs: []error{errOther}}
		err := database.Retry(context.Background(), a.fn, database.RetryOptions{Clock: testutil.NewClock(time.Unix(0, 0))})
		testutil.Equal(t, "error", err, errOther)
		testutil.Equal(t, "calls", a.calls, 1)
	})

	t.Run("should pass the context to the function", func(t *testing.T) {
		type key struct{}
		ctx := context.WithValue(context.Background(), key{}, "value")
		var got any
		testutil.NoError(t, database.Retry(ctx, func(ctx context.Context) error {
			got = ctx.Value(key{})
			return nil
		}, database.RetryOptions{Clock: testutil.NewClock(time.Unix(0, 0))}))
		testutil.Equal(t, "value", got, "value")
	})

	t.Run("should retry the transient errors with the default doubling delays", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		a := &attempts{errs: []error{errTransient, errTransient}}
		done := retryAsync(context.Background(), a.fn, database.RetryOptions{Clock: clock})

		waitTimer(t, clock)
		clock.Advance(database.DefaultRetryDelay)
		waitTimer(t, clock)
		clock.Advance(2*database.DefaultRetryDelay - time.Nanosecond)
		testutil.Equal(t, "timers before the doubled delay", clock.Timers(), 1)
		clock.Advance(time.Nanosecond)

		testutil.NoError(t, <-done)
		testutil.Equal(t, "calls", a.calls, database.DefaultRetryAttempts)
	})

	t.Run("should fail once the attempts are exhausted, waiting at most the max delay", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		a := &attempts{errs: []error{errTransient, errTransient, errTransient, errTransient}}
		done := retryAsync(context.Background(), a.fn, database.RetryOptions{
			Clock:    clock,
			Attempts: 4,
			Delay:    10 * time.Millisecond,
			MaxDelay: 15 * time.Millisecond,
		})

		for _, d := range []time.Duration{10 * time.Millisecond, 15 * time.Millisecond, 15 * time.Millisecond} {
			waitTimer(t, clock)
			clock.Advance(d)
		}

		err := <-done
		testutil.Error(t, err, database.ErrRetry,
			"unable to retry: unable to complete with retries: "+errTransient.Error(), map[string]any{
				database.ErrorFieldAttempts: 4,
			})
		if !errors.Is(err, errTransient) {
			t.Fatalf("Retry() = %v, want it to wrap the last error", err)
		}
		testutil.Equal(t, "calls", a.calls, 4)
	})

	t.Run("should take the delay as the max delay when longer", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		a := &attempts{errs: []error{errTransient, errTransient}}
		done := retryAsync(context.Background(), a.fn, database.RetryOptions{
			Clock:    clock,
			Delay:    time.Minute,
			MaxDelay: time.Second,
		})

		waitTimer(t, clock)
		clock.Advance(time.Minute)
		waitTimer(t, clock)
		clock.Advance(time.Minute)

		testutil.NoError(t, <-done)
	})

	t.Run("should stop waiting when the context ends", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		ctx, cancel := context.WithCancel(context.Background())
		a := &attempts{errs: []error{errTransient}}
		done := retryAsync(ctx, a.fn, database.RetryOptions{Clock: clock})

		waitTimer(t, clock)
		cancel()

		err := <-done
		testutil.Error(t, err, database.ErrRetry,
			"unable to retry: unable to complete with retries: context canceled\n"+errTransient.Error(), map[string]any{
				database.ErrorFieldAttempts: 1,
			})
		if !errors.Is(err, context.Canceled) || !errors.Is(err, errTransient) {
			t.Fatalf("Retry() = %v, want it to wrap the context and last errors", err)
		}
		testutil.Equal(t, "calls", a.calls, 1)
		testutil.Equal(t, "timers", clock.Timers(), 0)
	})

	t.Run("should not call the function when the context is already done", func(t *testing.T) {
		for name, tc := range map[string]struct {
			ctx  func() context.Context
			want error
			text string
		}{
			"canceled": {
				ctx: func() context.Context {
					ctx, cancel := context.WithCancel(context.Background())
					cancel()
					return ctx
				},
				want: context.Canceled,
				text: "context canceled",
			},
			"deadline exceeded": {
				ctx: func() context.Context {
					ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
					t.Cleanup(cancel)
					return ctx
				},
				want: context.DeadlineExceeded,
				text: "context deadline exceeded",
			},
		} {
			t.Run(name, func(t *testing.T) {
				clock := testutil.NewClock(time.Unix(0, 0))
				a := &attempts{errs: []error{errTransient}}

				err := database.Retry(tc.ctx(), a.fn, database.RetryOptions{Clock: clock})
				testutil.Error(t, err, database.ErrRetry,
					"unable to retry: unable to complete with retries: "+tc.text, map[string]any{
						database.ErrorFieldAttempts: 0,
					})
				if !errors.Is(err, tc.want) {
					t.Fatalf("Retry() = %v, want it to wrap %v", err, tc.want)
				}
				testutil.Equal(t, "calls", a.calls, 0)
				testutil.Equal(t, "timers", clock.Timers(), 0)
			})
		}
	})

	t.Run("should classify with the given function", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		errMine := errors.New("mine")
		a := &attempts{errs: []error{errMine}}
		done := retryAsync(context.Background(), a.fn, database.RetryOptions{
			Clock:       clock,
			IsTransient: func(err error) bool { return errors.Is(err, errMine) },
		})

		waitTimer(t, clock)
		clock.Advance(database.DefaultRetryDelay)

		testutil.NoError(t, <-done)
		testutil.Equal(t, "calls", a.calls, 2)

		a = &attempts{errs: []error{errTransient}}
		err := database.Retry(context.Background(), a.fn, database.RetryOptions{
			Clock:       clock,
			IsTransient: func(error) bool { return false },
		})
		testutil.Equal(t, "error", err, error(errTransient))
	})
}

func Test_IsTransient(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"other", errors.New("other"), false},
		{"postgres serialization failure", &pgconn.PgError{Code: "40001"}, true},
		{"postgres deadlock", fmt.Errorf("wrapped: %w", &pgconn.PgError{Code: "40P01"}), true},
		{"postgres unique violation", &pgconn.PgError{Code: "23505"}, false},
		{"mysql deadlock", &mysql.MySQLError{Number: 1213}, true},
		{"mysql lock wait timeout", fmt.Errorf("wrapped: %w", &mysql.MySQLError{Number: 1205}), true},
		{"mysql duplicate entry", &mysql.MySQLError{Number: 1062}, false},
		{"sqlite busy", sqlite3.Error{Code: sqlite3.ErrBusy}, true},
		{"sqlite locked", fmt.Errorf("wrapped: %w", sqlite3.Error{Code: sqlite3.ErrLocked}), true},
		{"sqlite constraint", sqlite3.Error{Code: sqlite3.ErrConstraint}, false},
		{"bad connection", fmt.Errorf("wrapped: %w", driver.ErrBadConn), true},
		{"mysql invalid connection", mysql.ErrInvalidConn, true},
		{"connection reset", &net.OpError{Op: "read", Err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}}, true},
		{"pgconn safe to retry", fmt.Errorf("wrapped: %w", safeToRetryError(true)), true},
		{"pgconn not safe to retry", safeToRetryError(false), false},
	} {
		t.Run("should classify "+tc.name, func(t *testing.T) {
			testutil.Equal(t, "IsTransient()", database.IsTransient(tc.err), tc.want)
		})
	}
}
