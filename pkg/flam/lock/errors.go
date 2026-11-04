package lock

import (
	"errors"
)

// The errors wrapped by the errors of the lockers and the locks, whatever
// the driver, so that their callers can tell them apart.
var (
	// ErrInvalidKey is the error wrapped by the error returned from the
	// Acquire and TryAcquire methods of a locker when the key is empty.
	ErrInvalidKey = errors.New("invalid lock key")

	// ErrInvalidTTL is the error wrapped by the error returned from the
	// Acquire and TryAcquire methods of a locker, and from the Refresh
	// method of a lock, when the ttl is shorter than 1ms.
	ErrInvalidTTL = errors.New("invalid lock ttl")

	// ErrNotAcquired is the error wrapped, along with the error of the
	// context, by the error returned from the Acquire method of a locker
	// when its context is done before the lock is acquired.
	ErrNotAcquired = errors.New("lock not acquired")

	// ErrNotHeld is the error wrapped by the error returned from the
	// Refresh method of a lock once it is released or lost, and from the
	// Refresh and Release methods of a lock the backend no longer holds
	// under its token.
	ErrNotHeld = errors.New("lock not held")

	// ErrStorage is the error wrapped, along with the error of the
	// backend, by the errors of the lockers and the locks when the backend
	// fails, as when the context given is done while it is reached.
	ErrStorage = errors.New("lock storage failure")

	// ErrClosed is the error wrapped by the errors returned from the
	// Acquire and TryAcquire methods of a closed locker.
	ErrClosed = errors.New("locker closed")
)
