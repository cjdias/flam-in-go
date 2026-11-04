package base

import (
	"context"
	"crypto/rand"
	"errors"
	randv2 "math/rand/v2"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
)

// Backend holds the keys of the locks of a driver, each under the token
// of its holder. Its methods report false when the key is not held, or not
// under the token, and fail with the errors of the backend only.
type Backend interface {
	// Acquire holds the key under the token for the ttl, unless held, and
	// returns its new fence.
	Acquire(ctx context.Context, key, token string, ttl time.Duration) (int64, bool, error)

	// Refresh holds the key under the token for the ttl from now, when it
	// is held under the token.
	Refresh(ctx context.Context, key, token string, ttl time.Duration) (bool, error)

	// Release removes the key, when it is held under the token.
	Release(ctx context.Context, key, token string) (bool, error)
}

// NewLocker returns the [flam.ILocker] of the id over the backend, waiting
// and measuring the ttl of its locks on the clock.
func NewLocker(
	id string,
	backend Backend,
	clock flam.IClock,
	options Options,
) flam.ILocker {
	return &locker{
		id:      id,
		backend: backend,
		clock:   clock,
		options: options,
		locks:   map[*heldLock]struct{}{},
	}
}

// locker is the [flam.ILocker] returned by [NewLocker].
type locker struct {
	id      string
	backend Backend
	clock   flam.IClock
	options Options

	// closing is held for reading by the acquisitions, from the check of
	// closed to the registration of their lock, and for writing by Close,
	// so that no lock is acquired once Close releases them
	closing sync.RWMutex
	closed  bool

	mu    sync.Mutex             // guards locks
	locks map[*heldLock]struct{} // held, to release on Close
}

func (l *locker) Id() string {
	return l.id
}

func (l *locker) Acquire(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (flam.ILock, error) {
	ctx = orBackground(ctx)
	for {
		if err := ctx.Err(); err != nil {
			return nil, newErrKey(lock.ErrNotAcquired, lockerAcquireMessage, l.id, key, err)
		}
		held, ok, err := l.try(ctx, lockerAcquireMessage, key, ttl)
		switch {
		case err != nil:
			return nil, err
		case ok:
			return held, nil
		}
		if err := l.wait(ctx); err != nil {
			return nil, newErrKey(lock.ErrNotAcquired, lockerAcquireMessage, l.id, key, err)
		}
	}
}

func (l *locker) TryAcquire(
	ctx context.Context,
	key string,
	ttl time.Duration,
) (flam.ILock, bool, error) {
	held, ok, err := l.try(orBackground(ctx), lockerTryAcquireMessage, key, ttl)
	if !ok {
		return nil, false, err
	}
	return held, true, nil
}

// Close releases the locks the locker still holds, ending them, and closes
// the locker, returning the errors of the releases joined. Calling it
// again is a no-op returning nil.
func (l *locker) Close() error {
	l.closing.Lock()
	closed := l.closed
	l.closed = true
	l.closing.Unlock()
	if closed {
		return nil
	}

	l.mu.Lock()
	held := make([]*heldLock, 0, len(l.locks))
	for h := range l.locks {
		held = append(held, h)
	}
	l.mu.Unlock()

	var errs []error
	for _, h := range held {
		if _, err := l.backend.Release(context.Background(), h.key, h.token); err != nil {
			errs = append(errs, newErrKey(lock.ErrStorage, lockerCloseMessage, l.id, h.key, err))
		}
		h.end()
	}
	return errors.Join(errs...)
}

// Messages of the errors returned by the locker methods.
const (
	lockerAcquireMessage    = "unable to acquire lock"
	lockerTryAcquireMessage = "unable to try to acquire lock"
	lockerCloseMessage      = "unable to release lock on close"
)

// orBackground returns the context, or context.Background when nil.
func orBackground(
	ctx context.Context,
) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// try tries once to acquire the lock of the key, registering it when
// acquired.
func (l *locker) try(
	ctx context.Context,
	message string,
	key string,
	ttl time.Duration,
) (*heldLock, bool, error) {
	switch {
	case key == "":
		return nil, false, newErrKey(lock.ErrInvalidKey, message, l.id, key, nil)
	case ttl < time.Millisecond:
		return nil, false, newErrKey(lock.ErrInvalidTTL, message, l.id, key, nil)
	}

	l.closing.RLock()
	defer l.closing.RUnlock()
	if l.closed {
		return nil, false, newErrKey(lock.ErrClosed, message, l.id, key, nil)
	}

	token := rand.Text()
	start := l.clock.Now()
	fence, ok, err := l.backend.Acquire(ctx, key, token, ttl)
	switch {
	case err != nil:
		return nil, false, newErrKey(lock.ErrStorage, message, l.id, key, err)
	case !ok:
		return nil, false, nil
	}

	h := newHeldLock(l, key, token, fence, ttl, start.Add(ttl))
	l.mu.Lock()
	l.locks[h] = struct{}{}
	l.mu.Unlock()
	go h.watch()
	return h, true, nil
}

// wait waits for the retry interval, plus a random jitter, or until the
// context is done, returning its error.
func (l *locker) wait(
	ctx context.Context,
) error {
	d := l.options.RetryInterval
	if l.options.RetryJitter > 0 {
		d += randv2.N(l.options.RetryJitter)
	}
	timer := l.clock.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C():
		return nil
	}
}

// forget removes the lock from the locks to release on Close.
func (l *locker) forget(
	h *heldLock,
) {
	l.mu.Lock()
	delete(l.locks, h)
	l.mu.Unlock()
}
