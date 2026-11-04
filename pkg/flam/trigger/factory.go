package trigger

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newFactory returns the [flam.ITriggerFactory] reading the time from the
// clock. It is the provider registered by the bundle returned from
// [NewBundle].
func newFactory(
	clock flam.IClock,
) flam.ITriggerFactory {
	return &factoryImpl{
		clock:    clock,
		triggers: map[string]*triggerImpl{},
	}
}

// factoryImpl is the [flam.ITriggerFactory] returned by newFactory.
type factoryImpl struct {
	clock flam.IClock

	mu       sync.Mutex
	closed   bool
	triggers map[string]*triggerImpl // the running ones, by id
}

// compile time check that *factoryImpl implements ITriggerFactory
var _ flam.ITriggerFactory = (*factoryImpl)(nil)

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

	switch {
	case f.closed:
		return nil, newErrClosed(message, id)
	case f.triggers[id] != nil:
		return nil, newErrDuplicateID(message, id)
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

// release stops tracking the trigger, once it stopped running.
func (f *factoryImpl) release(
	t *triggerImpl,
) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.triggers[t.id] == t {
		delete(f.triggers, t.id)
	}
}

// Messages of the errors returned by the factory methods.
const (
	factoryNewPulseMessage     = "unable to create pulse trigger"
	factoryNewRecurringMessage = "unable to create recurring trigger"
)
