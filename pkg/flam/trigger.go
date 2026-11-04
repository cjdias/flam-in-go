package flam

import (
	"io"
	"time"
)

// TriggerCallback is the function an [ITrigger] calls each time it fires.
// A non-nil error it returns is passed, wrapped, to the
// [TriggerErrorHandler] values of the trigger.
type TriggerCallback func() error

// TriggerErrorHandler receives the failures of the callback of an
// [ITrigger]: an error it returned, a panic or a [runtime.Goexit], as
// errors wrapping a sentinel error of
// [github.com/cjdias/flam-in-go/pkg/flam/trigger].
//
// A trigger fires on a goroutine of its own, where nobody waits for its
// callback, so the handlers given to the [ITriggerFactory] are the only
// way to learn that a callback fails. A trigger without handlers drops the
// failures.
type TriggerErrorHandler func(err error)

// ITrigger is a timer calling a callback once, for a pulse trigger, or
// periodically, for a recurring one, on a goroutine of its own, created by
// an [ITriggerFactory]. As an [IResource], its Id method returns the id it
// was created with, unique among the running triggers of its factory.
//
// A trigger runs until it is closed, or, for a pulse, until its single
// callback and the handlers of its failure have returned. A failure of the
// callback does not stop a recurring trigger: it fires again on the next
// period.
//
// Once Close has returned, the trigger is not running, and a tick still
// pending does not call the callback. Close does not wait for a callback
// already under way, which may even be the caller: a callback may close its
// own trigger.
//
// An ITrigger is safe for concurrent use.
type ITrigger interface {
	io.Closer
	IResource

	// IsRunning reports whether the trigger is running.
	IsRunning() bool

	// Delay returns the delay the trigger was created with: the time
	// before a pulse fires, or the period of a recurring trigger.
	Delay() time.Duration
}

// ITriggerFactory creates the triggers of an application, reading the time
// from an [IClock], so that a fake clock controls when they fire, and
// keeps track of the running ones: their ids are unique among them, and
// closing the factory closes them.
//
// The creation methods check, from the first, that the id is not empty,
// that the callback is not nil, that the delay is positive, that the
// factory is not closed, and that no running trigger uses the id, failing
// with an [*Error] wrapping the matching sentinel error of
// [github.com/cjdias/flam-in-go/pkg/flam/trigger]. The id of a trigger that
// stopped running can be used again.
//
// The handlers, which may be left out, receive each failure of the
// callback, in order; a nil handler is skipped, and a handler that panics
// or calls [runtime.Goexit] is skipped as well, without being reported.
//
// An ITriggerFactory is safe for concurrent use.
type ITriggerFactory interface {
	// Close closes the running triggers and the factory, after which
	// every creation fails. It returns nil, and calling it again is a
	// no-op.
	io.Closer

	// NewPulse returns a running trigger calling the callback once, after
	// the delay.
	NewPulse(
		id string,
		delay time.Duration,
		callback TriggerCallback,
		onError ...TriggerErrorHandler,
	) (ITrigger, error)

	// NewRecurring returns a running trigger calling the callback each
	// time the delay elapses, until it is closed. Like a ticker, it drops
	// the ticks elapsing while its callback runs: a slow callback is
	// called less often, never twice at once.
	NewRecurring(
		id string,
		delay time.Duration,
		callback TriggerCallback,
		onError ...TriggerErrorHandler,
	) (ITrigger, error)
}
