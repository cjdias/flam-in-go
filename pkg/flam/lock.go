package flam

import (
	"context"
	"io"
	"time"
)

// ILock is a lock acquired from an [ILocker], held under its key by the
// holder of its token until released, or lost: when its ttl elapsed
// without being refreshed, as when the holder stalled, or when the
// backend no longer holds it under its token, as when it was removed
// there. It may be used from several goroutines.
//
// A lock only excludes the holders playing by its rules: a holder that
// stalled past its ttl may still act while another holds the lock. The
// writes a lock guards should carry its Fence, the resource refusing a
// fence lower than the last one it saw, so that a stale holder cannot
// overwrite the work of the next one.
type ILock interface {
	// Key returns the key of the lock, as given to the locker.
	Key() string

	// Token returns the random token of the lock, unique to each
	// acquisition, which the backend holds the key under, so that only
	// this acquisition refreshes or releases it.
	Token() string

	// Fence returns the fencing token of the lock: a number, 1 or more,
	// incremented by the backend at each acquisition of the key, so that
	// a later acquisition always has a higher fence than an earlier one.
	Fence() int64

	// Refresh extends the lock to expire once the ttl, 1ms or more,
	// elapsed from now, provided the backend still holds it under its
	// token, the following automatic refreshes keeping that ttl. It fails
	// with [github.com/cjdias/flam-in-go/pkg/flam/lock.ErrNotHeld] once
	// the lock is released or lost, the lock then being lost when it was
	// not yet.
	Refresh(ctx context.Context, ttl time.Duration) error

	// Release releases the lock, removing it from the backend provided
	// the backend still holds it under its token, so that a lock lost and
	// acquired by another holder is never released by this one, and ends
	// the lock. It fails with
	// [github.com/cjdias/flam-in-go/pkg/flam/lock.ErrNotHeld] when the
	// backend no longer holds it under its token, as once released or
	// expired, the lock then being lost when it was not yet. A lock whose
	// release failed otherwise, as on an error of the backend, ends all
	// the same, its automatic refreshes stopping: its key then expires
	// from the backend once its ttl elapsed, unless released again
	// meanwhile.
	Release(ctx context.Context) error

	// Lost returns a channel, the same on every call, closed once the
	// lock is no longer held: released, lost or ended by the closing of
	// its locker. The locker tells a lock lost when its ttl elapsed, as
	// measured from the start of its acquisition or of its last
	// successful refresh, or when a refresh finds the backend no longer
	// holds it, so that the channel may close slightly before the backend
	// expires the key, but never after.
	Lost() <-chan struct{}
}

// ILocker acquires the [ILock] values of a backend, such as a Redis
// server, mutually exclusive between the lockers of a backend, such as
// those of the instances of an application, for the keys they share. It
// is held by an [ILockerFactory], which closes it when removing it.
//
// A locker may refresh the locks it holds on its own, as configured for
// its driver, so that a lock stays held while its holder lives, its ttl
// bounding the time a crashed holder keeps it. Closing a locker releases
// the locks it still holds, ending them, after which its methods fail
// with [github.com/cjdias/flam-in-go/pkg/flam/lock.ErrClosed].
//
// The errors are [*Error] values wrapping the sentinels of
// [github.com/cjdias/flam-in-go/pkg/flam/lock], holding the locker id and
// the key, so that the callers can tell them apart whatever the driver. A
// nil context is taken as [context.Background]. A locker is safe for
// concurrent use.
type ILocker interface {
	io.Closer
	IResource

	// Acquire acquires the lock of the key, non-empty, expiring once the
	// ttl, 1ms or more, elapsed, waiting while another holds it and trying
	// again after the retry interval of the locker, until acquired or the
	// context is done, which fails with
	// [github.com/cjdias/flam-in-go/pkg/flam/lock.ErrNotAcquired]. It
	// returns at once the errors of the backend.
	Acquire(ctx context.Context, key string, ttl time.Duration) (ILock, error)

	// TryAcquire acquires the lock of the key, as Acquire, when no other
	// holds it, and returns false at once, with no error, otherwise.
	TryAcquire(ctx context.Context, key string, ttl time.Duration) (ILock, bool, error)
}

// ILockerCreator creates the lockers of a driver, for the
// [ILockerFactory].
type ILockerCreator = IResourceCreator[ILocker]

// ILockerFactory creates and holds the lockers, one per id.
type ILockerFactory = IFactory[ILocker]

// ILeaderProcess is the [IWatchdogProcess] of a leader election, made by
// [github.com/cjdias/flam-in-go/pkg/flam/lock/leader.NewProcess]: among
// the processes campaigning for a key of the lockers of a backend, such
// as those of the instances of an application, a single one, the leader,
// runs its function at a time.
type ILeaderProcess interface {
	IWatchdogProcess

	// IsLeader reports whether the process holds the leadership, its
	// function running with it.
	IsLeader() bool
}
