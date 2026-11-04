package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// RetryOptions are the options of [Retry], each zero value taking its
// default.
type RetryOptions struct {
	// Clock is the clock waited with between two attempts, such as the
	// [flam.IClock] of the container. It is required.
	Clock flam.IClock

	// Attempts is the number of attempts, the first one included,
	// [DefaultRetryAttempts] when 0 or less.
	Attempts int

	// Delay is the wait before the first retry, doubled before each next
	// one, [DefaultRetryDelay] when 0 or less.
	Delay time.Duration

	// MaxDelay is the longest wait between two attempts,
	// [DefaultRetryMaxDelay] when 0 or less, or Delay when shorter.
	MaxDelay time.Duration

	// IsTransient tells whether an error of an attempt is worth retrying,
	// [IsTransient] when nil.
	IsTransient func(err error) bool
}

// Retry calls fn with ctx until it succeeds, retrying it when it fails
// with a transient error, as told by the options IsTransient, such as a
// serialization failure or a deadlock, after a wait doubling before each
// retry, for the number of attempts of the options.
//
// fn is a whole unit of work, such as a transaction, that can be run
// again from its start:
//
//	err := database.Retry(ctx, func(ctx context.Context) error {
//		return conn.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
//			// ...
//		})
//	}, database.RetryOptions{Clock: clock})
//
// No statement is ever retried on its own, as a connection does not know
// whether it is safe to: only fn is.
//
// Retry returns nil once fn succeeds, and the error of fn, unchanged, once
// it fails with an error that is not transient. It fails with ErrRetry,
// along with the error of the last attempt, once the attempts are
// exhausted, and along with the error of the context when the context ends
// while waiting, both with the ErrorFieldAttempts context field. It fails
// with ErrNilRetryFunc when fn is nil and ErrNilClock when the options have
// no clock.
func Retry(
	ctx context.Context,
	fn func(ctx context.Context) error,
	opts RetryOptions,
) error {
	if fn == nil {
		return newErrRetryNil(ErrNilRetryFunc)
	}
	if opts.Clock == nil {
		return newErrRetryNil(ErrNilClock)
	}
	attempts := retryDefault(opts.Attempts, DefaultRetryAttempts)
	delay := retryDefault(opts.Delay, DefaultRetryDelay)
	wait := newBackoff(delay, max(delay, retryDefault(opts.MaxDelay, DefaultRetryMaxDelay)))
	transient := opts.IsTransient
	if transient == nil {
		transient = IsTransient
	}

	for attempt := 1; ; attempt++ {
		err := fn(ctx)
		if err == nil || !transient(err) {
			return err
		}
		if attempt == attempts {
			return newErrRetry(attempt, err)
		}
		timer := opts.Clock.NewTimer(wait.next())
		select {
		case <-ctx.Done():
			timer.Stop()
			return newErrRetry(attempt, ctx.Err(), err)
		case <-timer.C():
		}
	}
}

// IsTransient tells whether the error, of a statement or a transaction, is
// a transient one, which running the same work again may not meet:
//
//   - a PostgreSQL serialization failure, 40001, or deadlock, 40P01, as a
//     [*pgconn.PgError];
//   - a MySQL deadlock, 1213, or lock wait timeout, 1205, as a
//     [*mysql.MySQLError];
//   - a SQLite busy or locked database, told by the message of the error,
//     "database is locked" or "database table is locked", as the SQLite
//     driver needs cgo;
//   - a lost connection: [driver.ErrBadConn], [mysql.ErrInvalidConn], a
//     connection reset by the server, or a pgconn error safe to retry, as
//     told by [pgconn.SafeToRetry].
//
// It is the default classification of [Retry].
func IsTransient(
	err error,
) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return slices.Contains(retryPostgresCodes, pgErr.Code)
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		return slices.Contains(retryMySQLNumbers, myErr.Number)
	}
	if errors.Is(err, driver.ErrBadConn) ||
		errors.Is(err, mysql.ErrInvalidConn) ||
		errors.Is(err, syscall.ECONNRESET) ||
		pgconn.SafeToRetry(err) {
		return true
	}
	message := err.Error()
	return slices.ContainsFunc(retrySQLiteMessages, func(m string) bool {
		return strings.Contains(message, m)
	})
}

// backoff gives the waits between attempts, from a delay doubling on each
// call up to a limit.
type backoff struct {
	delay time.Duration
	limit time.Duration
}

// newBackoff returns the backoff starting at the delay, doubling up to the
// limit, which must not be shorter.
func newBackoff(
	delay time.Duration,
	limit time.Duration,
) *backoff {
	return &backoff{delay: delay, limit: limit}
}

// next returns the wait before the next attempt, and doubles the
// following one, up to the limit, without overflowing.
func (b *backoff) next() time.Duration {
	d := b.delay
	b.delay = b.limit
	if d < b.limit/2 {
		b.delay = 2 * d
	}
	return d
}

// retryDefault returns the value, or the default when the value is 0 or
// less.
func retryDefault[T int | time.Duration](
	value T,
	def T,
) T {
	if value <= 0 {
		return def
	}
	return value
}

// The transient errors of each dialect, as read by IsTransient.
var (
	retryPostgresCodes  = []string{"40001", "40P01"}
	retryMySQLNumbers   = []uint16{1205, 1213}
	retrySQLiteMessages = []string{"database is locked", "database table is locked"}
)

// retryMessage is the message of the errors returned by Retry.
const retryMessage = "unable to retry"
