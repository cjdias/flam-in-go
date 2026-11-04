package log

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
)

// flusherArgs are the dependencies of the flusher, taken from the
// container. The trigger factory is optional, as it is only required while
// the period is positive.
type flusherArgs struct {
	dig.In

	Config   flam.IConfig
	Logger   flam.ILogger
	Triggers flam.ITriggerFactory `optional:"true"`
}

// newFlusher returns the flusher flushing the logger with a trigger of the
// factory, which may be nil when no trigger factory is provided. It is a
// provider registered by the bundle returned from [NewBundle].
func newFlusher(
	args flusherArgs,
) *flusher {
	return &flusher{
		config:   args.Config,
		logger:   args.Logger,
		triggers: args.Triggers,
	}
}

// flusher flushes the logger at the period stored at
// [FlusherPeriodConfigPath], with a recurring trigger of id
// [FlusherTriggerID] restarted when the period changes.
type flusher struct {
	config   flam.IConfig
	logger   flam.ILogger
	triggers flam.ITriggerFactory

	mu        sync.Mutex // guards observing, closed and trigger
	observing bool       // whether Boot added the observer
	closed    bool
	trigger   flam.ITrigger // the running one, or nil
}

// Boot observes the period, with an observer of id [FlusherTriggerID],
// then starts the trigger when the period is positive. It fails with the
// error of AddObserver, ErrMissingTriggerFactory when the period is
// positive and no trigger factory is provided, or the error of the
// trigger creation, in which case it removes the observer.
func (f *flusher) Boot() error {
	// the observer is added holding the flusher, so that Close, once
	// started, removes it or prevents it
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return nil
	}
	if err := f.config.AddObserver(FlusherTriggerID, FlusherPeriodConfigPath, f.observe); err != nil {
		f.mu.Unlock()
		return newErrFlusher(flusherBootMessage, err)
	}
	f.observing = true
	f.mu.Unlock()

	if err := f.restart(flusherBootMessage); err != nil {
		f.mu.Lock()
		f.observing = false
		f.mu.Unlock()
		_ = f.config.RemoveObserver(FlusherTriggerID) // added above, unless by Close meanwhile
		return err
	}
	return nil
}

// Close stops the trigger and removes the observer of the period added by
// Boot, after which a change of the period starts nothing, then flushes
// the logger a last time, so that no entry is lost. It returns the errors
// of the trigger Close and of the flush, joined. Closing a flusher closed
// already only flushes the logger.
func (f *flusher) Close() error {
	f.mu.Lock()
	f.closed = true
	trigger, observing := f.trigger, f.observing
	f.trigger, f.observing = nil, false
	f.mu.Unlock()

	var errs []error
	if trigger != nil {
		errs = append(errs, trigger.Close())
	}
	if observing {
		_ = f.config.RemoveObserver(FlusherTriggerID) // added by Boot
	}
	if err := f.logger.Flush(); err != nil {
		errs = append(errs, newErrFlusher(flusherCloseMessage, err))
	}
	return errors.Join(errs...)
}

// Messages of the errors returned and logged by the flusher.
const (
	flusherBootMessage    = "unable to boot log flusher"
	flusherObserveMessage = "unable to restart log flusher"
	flusherCloseMessage   = "unable to flush log on close"
	flusherFlushMessage   = "unable to flush log"
)

// observe is the observer of the period: it restarts the trigger at the
// new period. A failure, which leaves no trigger running, is broadcast as
// an error entry, as nobody waits for an observer.
func (f *flusher) observe(
	_, _ any,
) {
	if err := f.restart(flusherObserveMessage); err != nil {
		f.logger.BroadcastError(flusherObserveMessage, container.Bag{ErrorFieldError: err})
	}
}

// flush is the callback of the trigger, flushing the logger, unless the
// flusher was closed since the trigger fired, as Close flushes it last.
func (f *flusher) flush() error {
	if f.isClosed() {
		return nil
	}
	return f.logger.Flush()
}

// isClosed reports whether the flusher is closed.
func (f *flusher) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// onError is the error handler of the trigger. It reports the failed flush
// through the logger that could not be flushed, so that the entry is
// written by the next flush to the streams that still work: the flusher
// carries on rather than going quiet.
func (f *flusher) onError(
	err error,
) {
	if !f.isClosed() { // or no flush would write the entry
		f.logger.BroadcastError(flusherFlushMessage, container.Bag{ErrorFieldError: err})
	}
}

// restart reads the period from the configuration, as the Duration getter
// does, and replaces the trigger with one of that period, or with none
// when it is not positive. It keeps the trigger when its period is the
// same, and does nothing once the flusher is closed.
func (f *flusher) restart(
	message string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return nil
	}
	period := max(f.config.Duration(FlusherPeriodConfigPath, defaultFlushPeriod), 0)
	if f.trigger != nil {
		if f.trigger.IsRunning() && f.trigger.Delay() == period {
			return nil
		}
		_ = f.trigger.Close() // always nil, and the trigger is gone anyway
		f.trigger = nil
	}
	if period == time.Duration(0) {
		return nil
	}

	if f.triggers == nil {
		return newErrMissingTriggerFactory(message, period)
	}
	trigger, err := f.triggers.NewRecurring(FlusherTriggerID, period, f.flush, f.onError)
	if err != nil {
		return newErrFlusher(message, err)
	}
	f.trigger = trigger
	return nil
}
