package base

import (
	"context"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/lock"
)

// heldLock is the [flam.ILock] acquired by a locker, watched by a
// goroutine of its own, which refreshes it when the locker auto refreshes
// its locks, and ends it once its ttl elapsed, until it ends.
type heldLock struct {
	locker *locker
	key    string
	token  string
	fence  int64

	ctx    context.Context // of the automatic refreshes, done once ended
	cancel context.CancelFunc
	once   sync.Once     // ends the lock
	lost   chan struct{} // closed once ended
	wake   chan struct{} // tells the watch the ttl changed, buffered

	mu       sync.Mutex // guards the fields below
	ttl      time.Duration
	deadline time.Time
}

// compile time check that *heldLock implements ILock
var _ flam.ILock = (*heldLock)(nil)

func (h *heldLock) Key() string {
	return h.key
}

func (h *heldLock) Token() string {
	return h.token
}

func (h *heldLock) Fence() int64 {
	return h.fence
}

func (h *heldLock) Lost() <-chan struct{} {
	return h.lost
}

func (h *heldLock) Refresh(
	ctx context.Context,
	ttl time.Duration,
) error {
	switch {
	case ttl < time.Millisecond:
		return h.err(lock.ErrInvalidTTL, lockRefreshMessage, nil)
	case h.ended():
		return h.err(lock.ErrNotHeld, lockRefreshMessage, nil)
	}

	start := h.locker.clock.Now()
	ok, err := h.locker.backend.Refresh(orBackground(ctx), h.key, h.token, ttl)
	switch {
	case err != nil:
		return h.err(lock.ErrStorage, lockRefreshMessage, err)
	case !ok:
		h.end()
		return h.err(lock.ErrNotHeld, lockRefreshMessage, nil)
	}
	h.prolong(start, ttl)
	select {
	case h.wake <- struct{}{}:
	default: // a wake is pending already
	}
	return nil
}

func (h *heldLock) Release(
	ctx context.Context,
) error {
	ok, err := h.locker.backend.Release(orBackground(ctx), h.key, h.token)
	if err != nil {
		return h.err(lock.ErrStorage, lockReleaseMessage, err)
	}
	h.end()
	if !ok {
		return h.err(lock.ErrNotHeld, lockReleaseMessage, nil)
	}
	return nil
}

// Messages of the errors returned by the lock methods.
const (
	lockRefreshMessage = "unable to refresh lock"
	lockReleaseMessage = "unable to release lock"
)

// newHeldLock returns the lock of the key acquired by the locker under the
// token, with the fence, for the ttl, held until the deadline.
func newHeldLock(
	l *locker,
	key string,
	token string,
	fence int64,
	ttl time.Duration,
	deadline time.Time,
) *heldLock {
	ctx, cancel := context.WithCancel(context.Background())
	return &heldLock{
		locker:   l,
		key:      key,
		token:    token,
		fence:    fence,
		ctx:      ctx,
		cancel:   cancel,
		lost:     make(chan struct{}),
		wake:     make(chan struct{}, 1),
		ttl:      ttl,
		deadline: deadline,
	}
}

// watch runs the steps of the lock until it ends.
func (h *heldLock) watch() {
	for h.step() {
	}
}

// step waits for the next automatic refresh, when due before the
// deadline, or else for the deadline, and runs it, unless the ttl changed
// or the lock ended meanwhile, reporting whether the lock may still be
// held.
func (h *heldLock) step() bool {
	h.mu.Lock()
	ttl, deadline := h.ttl, h.deadline
	h.mu.Unlock()

	wait := h.locker.clock.Until(deadline)
	refresh := h.locker.options.AutoRefresh && ttl/3 < wait
	if refresh {
		wait = ttl / 3
	}
	timer := h.locker.clock.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-h.lost:
		return false
	case <-h.wake:
		return true
	case <-timer.C():
	}

	if refresh {
		return h.autoRefresh()
	}
	return !h.expire()
}

// autoRefresh refreshes the lock for its ttl, as last set, ending it when
// the backend no longer holds it, and reports whether it may still be
// held. A refresh failing on an error of the backend is tried again at the
// next step.
func (h *heldLock) autoRefresh() bool {
	h.mu.Lock()
	ttl := h.ttl
	h.mu.Unlock()

	start := h.locker.clock.Now()
	ok, err := h.locker.backend.Refresh(h.ctx, h.key, h.token, ttl)
	switch {
	case err != nil:
		return true
	case !ok:
		h.end()
		return false
	}
	h.prolong(start, ttl)
	return true
}

// expire ends the lock when its deadline is reached, as a refresh may have
// moved it meanwhile, and reports whether it did.
func (h *heldLock) expire() bool {
	h.mu.Lock()
	due := !h.locker.clock.Now().Before(h.deadline)
	h.mu.Unlock()
	if due {
		h.end()
	}
	return due
}

// prolong sets the ttl of the lock, held until the ttl elapsed from the
// start of its refresh.
func (h *heldLock) prolong(
	start time.Time,
	ttl time.Duration,
) {
	h.mu.Lock()
	h.ttl, h.deadline = ttl, start.Add(ttl)
	h.mu.Unlock()
}

// end ends the lock, once: it stops its automatic refreshes, closes its
// lost channel and removes it from the locks of its locker.
func (h *heldLock) end() {
	h.once.Do(func() {
		h.cancel()
		close(h.lost)
		h.locker.forget(h)
	})
}

// ended reports whether the lock ended.
func (h *heldLock) ended() bool {
	select {
	case <-h.lost:
		return true
	default:
		return false
	}
}

// err returns the error wrapping the sentinel, joined with the cause when
// not nil, with the message and the id and key context fields.
func (h *heldLock) err(
	sentinel error,
	message string,
	cause error,
) *flam.Error {
	return newErrKey(sentinel, message, h.locker.id, h.key, cause)
}
