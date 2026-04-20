package flam

import (
	"sync"
	"time"
)

func newPulseTrigger(
	delay time.Duration,
	callback TriggerCallback,
) (Trigger, error) {
	if callback == nil {
		return nil, newErrNilReference("callback")
	}

	timer := time.NewTimer(delay)
	closeCh := make(chan struct{}, 1)
	var closeOnce sync.Once
	var cleanupMu sync.Mutex
	cleanupStarted := false

	var t *trigger
	t = &trigger{
		delay:     delay,
		isRunning: true,
		closer: func() error {
			t.mu.Lock()
			defer t.mu.Unlock()
			cleanupMu.Lock()
			defer cleanupMu.Unlock()
			if t.isRunning && !cleanupStarted {
				t.isRunning = false
				select {
				case closeCh <- struct{}{}:
				default:
				}
			}
			return nil
		},
		cleaner: func() error {
			closeOnce.Do(func() {
				cleanupMu.Lock()
				cleanupStarted = true
				cleanupMu.Unlock()
				if timer != nil {
					timer.Stop()
					close(closeCh)
				}
			})
			return nil
		},
	}

	go func(t *trigger) {
		if timer != nil {
			select {
			case <-timer.C:
				if e := callback(); e != nil {
					_ = t.cleaner()
				}
			case <-closeCh:
			}
		}
		_ = t.cleaner()
	}(t)

	return t, nil
}
