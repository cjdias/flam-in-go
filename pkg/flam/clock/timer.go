package clock

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// timerImpl is the [flam.ITimer] of the system clock, wrapping a time.Timer.
type timerImpl struct {
	timer *time.Timer
}

// compile time check that *timerImpl implements ITimer
var _ flam.ITimer = (*timerImpl)(nil)

func (t *timerImpl) C() <-chan time.Time {
	return t.timer.C
}

func (t *timerImpl) Stop() bool {
	return t.timer.Stop()
}

func (t *timerImpl) Reset(
	d time.Duration,
) bool {
	return t.timer.Reset(d)
}
