package flam

import (
	"time"
)

type TriggerFactory interface {
	NewPulse(delay time.Duration, callback TriggerCallback) (Trigger, error)
	NewRecurring(delay time.Duration, callback TriggerCallback) (Trigger, error)
}

type triggerFactory struct{}

func newTriggerFactory() TriggerFactory {
	return &triggerFactory{}
}

func (factory *triggerFactory) NewPulse(
	delay time.Duration,
	callback TriggerCallback,
) (Trigger, error) {
	return newPulseTrigger(delay, callback)
}

func (factory *triggerFactory) NewRecurring(
	delay time.Duration,
	callback TriggerCallback,
) (Trigger, error) {
	return newRecurringTrigger(delay, callback)
}
