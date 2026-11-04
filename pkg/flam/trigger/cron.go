package trigger

import (
	"errors"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// cronTriggerConfig is what a cron trigger is created from.
type cronTriggerConfig struct {
	id       string
	schedule flam.CronSchedule
	parsed   cron.Schedule
	callback flam.TriggerCallback
	handlers []flam.TriggerErrorHandler
	clock    flam.IClock
	release  func(id string, t io.Closer) // called once the trigger stops running
}

// cronTriggerImpl is the [flam.ICronTrigger] created by the factory: a
// goroutine waiting, with a timer of the clock, for the next time matched
// by the schedule, and calling the callback then.
type cronTriggerImpl struct {
	cronTriggerConfig

	// isRunning is cleared by Close, or by the goroutine when it ends, and
	// read by IsRunning from any goroutine.
	isRunning atomic.Bool

	// mu guards closed, which stops the callback calls and the timers, set
	// by Close, which also closes closeCh to wake the goroutine, or by the
	// goroutine when it ends, and the timer of the next fire, at next.
	mu      sync.Mutex
	closed  bool
	closeCh chan struct{}
	timer   flam.ITimer
	next    time.Time
}

// compile time check that *cronTriggerImpl implements ICronTrigger
var _ flam.ICronTrigger = (*cronTriggerImpl)(nil)

// newCronTrigger returns a running cron trigger, whose goroutine is not
// yet started: call start.
func newCronTrigger(
	config cronTriggerConfig,
) *cronTriggerImpl {
	t := &cronTriggerImpl{
		cronTriggerConfig: config,
		closeCh:           make(chan struct{}),
	}
	t.isRunning.Store(true)
	return t
}

func (t *cronTriggerImpl) Close() error {
	t.mu.Lock()
	t.isRunning.Store(false)
	if !t.closed {
		t.closed = true
		close(t.closeCh)
	}
	t.stopLocked()
	t.mu.Unlock()

	t.release(t.id, t)
	return nil
}

func (t *cronTriggerImpl) Id() string {
	return t.id
}

func (t *cronTriggerImpl) IsRunning() bool {
	return t.isRunning.Load()
}

func (t *cronTriggerImpl) Schedule() flam.CronSchedule {
	return t.schedule
}

func (t *cronTriggerImpl) Next() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.next
}

// start starts the goroutine of the trigger.
func (t *cronTriggerImpl) start() {
	go t.run()
}

// run waits for each time matched by the schedule, and calls the callback
// then, until the trigger is closed or the schedule matches no time
// anymore. The next time is looked for from the current time once the
// callback returns, so that the times matched meanwhile are skipped, and
// never before the last time fired, should the wall clock go back.
func (t *cronTriggerImpl) run() {
	defer t.finish()

	var last time.Time
	for {
		now := t.clock.Now()
		if now.Before(last) {
			now = last
		}
		next := t.parsed.Next(now)
		if next.IsZero() {
			return
		}

		tick, ok := t.arm(next, next.Sub(now))
		if !ok {
			return
		}
		select {
		case <-t.closeCh:
			return
		case <-tick:
			// the tick may be picked though the trigger got closed too
			if !t.fire() {
				return
			}
			last = next
		}
	}
}

// arm starts the timer of the next fire, at next, after the delay, and
// returns its channel, unless the trigger is closed.
func (t *cronTriggerImpl) arm(
	next time.Time,
	delay time.Duration,
) (<-chan time.Time, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, false
	}
	t.timer = t.clock.NewTimer(delay)
	t.next = next
	return t.timer.C(), true
}

// fire calls the callback, unless the trigger is closed, and reports
// whether the trigger is still open afterwards.
func (t *cronTriggerImpl) fire() bool {
	t.mu.Lock()
	closed := t.closed
	t.next = time.Time{}
	t.timer = nil
	t.mu.Unlock()

	if closed {
		return false
	}
	fire(t.id, t.callback, t.handlers)

	t.mu.Lock()
	defer t.mu.Unlock()

	return !t.closed
}

// finish ends the trigger from its goroutine, once: it stops the timer,
// marks the trigger closed and not running, and releases it from the
// factory.
func (t *cronTriggerImpl) finish() {
	t.mu.Lock()
	t.closed = true
	t.stopLocked()
	t.mu.Unlock()

	// released first, so that the id is free once it no longer runs
	t.release(t.id, t)
	t.isRunning.Store(false)
}

// stopLocked stops the timer of the next fire, if any.
func (t *cronTriggerImpl) stopLocked() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.next = time.Time{}
}

// parseSchedule returns the cron schedule of the expression, read in the
// time zone of its prefix, or else of the location, loaded with the clock,
// or else of the times it is given.
func parseSchedule(
	clock flam.IClock,
	schedule flam.CronSchedule,
) (cron.Schedule, error) {
	spec := strings.TrimSpace(schedule.Spec)
	zone, hasZone := schedule.Location, schedule.Location != ""
	for _, prefix := range cronZonePrefixes {
		if rest, found := strings.CutPrefix(spec, prefix); found {
			name, expr, hasExpr := strings.Cut(rest, " ")
			if !hasExpr {
				return nil, errCronNoExpression
			}
			spec, zone, hasZone = strings.TrimSpace(expr), name, true
			break
		}
	}

	options := cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor
	if schedule.Seconds {
		options |= cron.SecondOptional
	}
	parsed, err := cron.NewParser(options).Parse(spec)
	if err != nil {
		return nil, err
	}
	if hasZone {
		loc, err := clock.LoadLocation(zone)
		if err != nil {
			return nil, err
		}
		if specSchedule, ok := parsed.(*cron.SpecSchedule); ok {
			specSchedule.Location = loc
		}
	}
	return parsed, nil
}

// cronZonePrefixes are the prefixes of a cron expression naming its time
// zone.
var cronZonePrefixes = []string{"CRON_TZ=", "TZ="}

// errCronNoExpression is the parsing error of a cron expression holding
// only a time zone prefix.
var errCronNoExpression = errors.New("missing cron expression after the time zone")
