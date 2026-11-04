package trigger

import (
	"io"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// triggerConfig is what a trigger is created from.
type triggerConfig struct {
	id        string
	delay     time.Duration
	callback  flam.TriggerCallback
	handlers  []flam.TriggerErrorHandler
	recurring bool
	clock     flam.IClock
	release   func(id string, t io.Closer) // called once the trigger stops running
}

// triggerImpl is the [flam.ITrigger] created by the factory: a goroutine
// waiting for the ticks of a timer, for a pulse, or of a ticker, for a
// recurring trigger, and calling the callback on each.
type triggerImpl struct {
	triggerConfig

	tick <-chan time.Time
	stop func() // stops the timer or ticker

	// isRunning is cleared by Close, or by the goroutine when it ends, and
	// read by IsRunning from any goroutine.
	isRunning atomic.Bool

	// mu guards closed, which stops the callback calls: set by Close, which
	// also closes closeCh to wake the goroutine, or by the goroutine when
	// it ends.
	mu      sync.Mutex
	closed  bool
	closeCh chan struct{}
}

// compile time check that *triggerImpl implements ITrigger
var _ flam.ITrigger = (*triggerImpl)(nil)

// newTrigger returns a running trigger, with its timer or ticker started
// but its goroutine not yet: call start.
func newTrigger(
	config triggerConfig,
) *triggerImpl {
	t := &triggerImpl{
		triggerConfig: config,
		closeCh:       make(chan struct{}),
	}
	if config.recurring {
		ticker := config.clock.NewTicker(config.delay)
		t.tick, t.stop = ticker.C(), ticker.Stop
	} else {
		timer := config.clock.NewTimer(config.delay)
		t.tick, t.stop = timer.C(), func() { timer.Stop() }
	}
	t.isRunning.Store(true)
	return t
}

func (t *triggerImpl) Close() error {
	t.mu.Lock()
	t.isRunning.Store(false)
	if !t.closed {
		t.closed = true
		close(t.closeCh)
	}
	t.mu.Unlock()

	// stopped here too, as the goroutine stops it only once its callback
	// under way, if any, returns
	t.stop()
	t.release(t.id, t)
	return nil
}

func (t *triggerImpl) Id() string {
	return t.id
}

func (t *triggerImpl) IsRunning() bool {
	return t.isRunning.Load()
}

func (t *triggerImpl) Delay() time.Duration {
	return t.delay
}

// triggerCallbackMessage is the message of the callback failures.
const triggerCallbackMessage = "unable to run trigger callback"

// start starts the goroutine of the trigger.
func (t *triggerImpl) start() {
	go t.run()
}

// run calls the callback on each tick, until the trigger is closed, or
// after the single call of a pulse.
func (t *triggerImpl) run() {
	defer t.finish()

	for {
		select {
		case <-t.closeCh:
			return
		case <-t.tick:
			// the tick may be picked though the trigger got closed too
			if t.open() {
				t.fire()
			}
			if !t.recurring || !t.open() {
				return
			}
		}
	}
}

// finish ends the trigger from its goroutine, once: it stops the timer or
// ticker, marks the trigger closed and not running, and releases it from
// the factory.
func (t *triggerImpl) finish() {
	t.stop()

	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	// released first, so that the id is free once it no longer runs
	t.release(t.id, t)
	t.isRunning.Store(false)
}

// open reports whether the trigger may still call its callback. It is
// checked right before each call, as a tick and a Close can both be ready
// when the goroutine selects, and select picks either.
func (t *triggerImpl) open() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return !t.closed
}

// fire calls the callback, and reports its failure to the handlers.
func (t *triggerImpl) fire() {
	fire(t.id, t.callback, t.handlers)
}

// fire calls the callback of the trigger of the id, and reports its
// failure to the handlers.
func fire(
	id string,
	callback flam.TriggerCallback,
	handlers []flam.TriggerErrorHandler,
) {
	if err := call(id, callback); err != nil {
		for _, handler := range handlers {
			if handler != nil {
				handle(handler, err)
			}
		}
	}
}

// call calls the callback of the trigger of the id on a goroutine of its
// own, so that neither a panic nor a runtime.Goexit ends the trigger
// goroutine, and returns its failure, or nil. A nil error held by a
// non-nil error interface is not a failure.
func call(
	id string,
	callback flam.TriggerCallback,
) (err error) {
	done := make(chan struct{})
	go func() {
		defer close(done)

		returned := false
		defer func() {
			r := recover()
			switch {
			case r != nil:
				err = newErrCallbackPanic(triggerCallbackMessage, id, r, debug.Stack())
			case !returned:
				err = newErrCallbackExit(triggerCallbackMessage, id)
			}
		}()

		result := callback()
		returned = true
		if !value.IsNil(result) {
			err = newErrCallback(triggerCallbackMessage, id, result)
		}
	}()
	<-done
	return err
}

// handle calls the handler with the error on a goroutine of its own,
// dropping a panic or a runtime.Goexit of the handler.
func handle(
	handler flam.TriggerErrorHandler,
	err error,
) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { _ = recover() }()

		handler(err)
	}()
	<-done
}
