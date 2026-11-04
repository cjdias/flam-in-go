package trigger

import (
	"io"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newFactory returns the factory reading the time from the clock, both as
// a [flam.ITriggerFactory] and as a [flam.ICronTriggerFactory]. It is the
// provider registered by the bundle returned from [NewBundle].
func newFactory(
	clock flam.IClock,
) (flam.ITriggerFactory, flam.ICronTriggerFactory) {
	f := &factoryImpl{
		clock:    clock,
		triggers: map[string]io.Closer{},
	}
	return f, f
}

// factoryImpl is the [flam.ICronTriggerFactory] returned by newFactory.
type factoryImpl struct {
	clock flam.IClock

	mu       sync.Mutex
	closed   bool
	triggers map[string]io.Closer // the running ones, by id
}

// compile time check that *factoryImpl implements ICronTriggerFactory
var _ flam.ICronTriggerFactory = (*factoryImpl)(nil)

func (f *factoryImpl) Close() error {
	f.mu.Lock()
	f.closed = true
	triggers := slices.Collect(maps.Values(f.triggers))
	clear(f.triggers)
	f.mu.Unlock()

	for _, t := range triggers {
		_ = t.Close() // always nil
	}
	return nil
}

func (f *factoryImpl) NewPulse(
	id string,
	delay time.Duration,
	callback flam.TriggerCallback,
	onError ...flam.TriggerErrorHandler,
) (flam.ITrigger, error) {
	return f.create(factoryNewPulseMessage, id, delay, callback, onError, false)
}

func (f *factoryImpl) NewRecurring(
	id string,
	delay time.Duration,
	callback flam.TriggerCallback,
	onError ...flam.TriggerErrorHandler,
) (flam.ITrigger, error) {
	return f.create(factoryNewRecurringMessage, id, delay, callback, onError, true)
}

func (f *factoryImpl) NewCron(
	id string,
	schedule flam.CronSchedule,
	callback flam.TriggerCallback,
	onError ...flam.TriggerErrorHandler,
) (flam.ICronTrigger, error) {
	switch {
	case id == "":
		return nil, newErrInvalidID(factoryNewCronMessage)
	case callback == nil:
		return nil, newErrNilCallback(factoryNewCronMessage, id)
	}
	parsed, err := parseSchedule(f.clock, schedule)
	if err != nil {
		return nil, newErrInvalidSchedule(factoryNewCronMessage, id, schedule, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkLocked(factoryNewCronMessage, id); err != nil {
		return nil, err
	}
	t := newCronTrigger(cronTriggerConfig{
		id:       id,
		schedule: schedule,
		parsed:   parsed,
		callback: callback,
		handlers: slices.Clone(onError),
		clock:    f.clock,
		release:  f.release,
	})
	f.triggers[id] = t
	t.start()
	return t, nil
}

// create checks the arguments and the factory state, then starts and
// tracks a trigger.
func (f *factoryImpl) create(
	message string,
	id string,
	delay time.Duration,
	callback flam.TriggerCallback,
	onError []flam.TriggerErrorHandler,
	recurring bool,
) (flam.ITrigger, error) {
	switch {
	case id == "":
		return nil, newErrInvalidID(message)
	case callback == nil:
		return nil, newErrNilCallback(message, id)
	case delay <= 0:
		return nil, newErrInvalidDelay(message, id, delay)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkLocked(message, id); err != nil {
		return nil, err
	}
	t := newTrigger(triggerConfig{
		id:        id,
		delay:     delay,
		callback:  callback,
		handlers:  slices.Clone(onError),
		recurring: recurring,
		clock:     f.clock,
		release:   f.release,
	})
	f.triggers[id] = t
	t.start()
	return t, nil
}

// checkLocked returns the error refusing a new trigger of the id, as the
// factory is closed or a running trigger uses the id, or nil.
func (f *factoryImpl) checkLocked(
	message string,
	id string,
) error {
	switch {
	case f.closed:
		return newErrClosed(message, id)
	case f.triggers[id] != nil:
		return newErrDuplicateID(message, id)
	}
	return nil
}

// release stops tracking the trigger of the id, once it stopped running.
func (f *factoryImpl) release(
	id string,
	t io.Closer,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.triggers[id] == t {
		delete(f.triggers, id)
	}
}

// Messages of the errors returned by the factory methods.
const (
	factoryNewPulseMessage     = "unable to create pulse trigger"
	factoryNewRecurringMessage = "unable to create recurring trigger"
	factoryNewCronMessage      = "unable to create cron trigger"
)
