package config

import (
	"sync"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// reloaderArgs are the dependencies of the reloader, taken from the
// container. The trigger factory is optional, as it is only required while
// the period is positive.
type reloaderArgs struct {
	dig.In

	Config   flam.IConfig
	Triggers flam.ITriggerFactory `optional:"true"`
}

// newReloader returns the reloader reloading the sources of the
// configuration with a trigger of the factory, which may be nil when no
// trigger factory is provided. It is a provider registered by the bundle
// returned from [NewBundle].
func newReloader(
	args reloaderArgs,
) *reloader {
	return &reloader{
		config:   args.Config,
		triggers: args.Triggers,
	}
}

// reloader reloads the sources of the configuration, with its
// ReloadSources method, at the period stored at [ReloadPeriodConfigPath],
// with a recurring trigger of id [ReloaderTriggerID] restarted when the
// period changes.
type reloader struct {
	config   flam.IConfig
	triggers flam.ITriggerFactory

	mu        sync.Mutex // guards observing, closed and trigger
	observing bool       // whether Boot added the observer
	closed    bool
	trigger   flam.ITrigger // the running one, or nil
}

// Boot observes the period, with an observer of id [ReloaderTriggerID],
// then starts the trigger when the period is positive. It fails with the
// error of AddObserver, ErrMissingTriggerFactory when the period is
// positive and no trigger factory is provided, or the error of the
// trigger creation, in which case it removes the observer.
func (r *reloader) Boot() error {
	// the observer is added holding the reloader, so that Close, once
	// started, removes it or prevents it
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	if err := r.config.AddObserver(ReloaderTriggerID, ReloadPeriodConfigPath, r.observe); err != nil {
		r.mu.Unlock()
		return newErrReloader(reloaderBootMessage, err)
	}
	r.observing = true
	r.mu.Unlock()

	if err := r.restart(reloaderBootMessage); err != nil {
		r.mu.Lock()
		r.observing = false
		r.mu.Unlock()
		_ = r.config.RemoveObserver(ReloaderTriggerID) // added above, unless by Close meanwhile
		return err
	}
	return nil
}

// Close stops the trigger and removes the observer of the period added by
// Boot, after which a change of the period starts nothing. It returns the
// error of the trigger Close. Closing a reloader never booted, or closed
// already, does nothing more.
func (r *reloader) Close() error {
	r.mu.Lock()
	r.closed = true
	trigger, observing := r.trigger, r.observing
	r.trigger, r.observing = nil, false
	r.mu.Unlock()

	var err error
	if trigger != nil {
		err = trigger.Close()
	}
	if observing {
		_ = r.config.RemoveObserver(ReloaderTriggerID) // added by Boot
	}
	return err
}

// Messages of the errors returned by the reloader.
const (
	reloaderBootMessage    = "unable to boot config reloader"
	reloaderObserveMessage = "unable to restart config reloader"
)

// observe is the observer of the period: it restarts the trigger at the
// new period. A failure, which leaves no trigger running, is dropped, as
// nobody waits for an observer.
func (r *reloader) observe(
	_, _ any,
) {
	_ = r.restart(reloaderObserveMessage)
}

// restart reads the period from the configuration, as the Duration getter
// does, and replaces the trigger with one of that period, or with none
// when it is not positive. It keeps the trigger when its period is the
// same, and does nothing once the reloader is closed.
func (r *reloader) restart(
	message string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}
	period := max(r.config.Duration(ReloadPeriodConfigPath, defaultReloadPeriod), 0)
	if r.trigger != nil {
		if r.trigger.IsRunning() && r.trigger.Delay() == period {
			return nil
		}
		_ = r.trigger.Close() // always nil, and the trigger is gone anyway
		r.trigger = nil
	}
	if period == time.Duration(0) {
		return nil
	}

	if r.triggers == nil {
		return newErrMissingTriggerFactory(message, period)
	}
	trigger, err := r.triggers.NewRecurring(ReloaderTriggerID, period, r.config.ReloadSources)
	if err != nil {
		return newErrReloader(message, err)
	}
	r.trigger = trigger
	return nil
}
