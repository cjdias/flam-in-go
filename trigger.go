package flam

import (
	"io"
	"sync"
	"time"
)

type TriggerCallback func() error

type Trigger interface {
	io.Closer

	IsRunning() bool
	Delay() time.Duration
}

type trigger struct {
	mu        sync.Mutex
	delay     time.Duration
	isRunning bool
	closer    func() error
	cleaner   func() error
}

var _ Trigger = (*trigger)(nil)

func (trigger *trigger) Close() error {
	return trigger.closer()
}

func (trigger *trigger) IsRunning() bool {
	trigger.mu.Lock()
	defer trigger.mu.Unlock()
	return trigger.isRunning
}

func (trigger *trigger) Delay() time.Duration {
	return trigger.delay
}
