package testutil

import (
	"slices"
	"sync"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Clock is a fake flam.IClock whose time only moves with Advance, which
// fires the timers and tickers it made as their deadlines are reached.
// Its methods building, parsing and converting times delegate to the time
// package.
//
// As with the time package since Go 1.23, a timer or ticker channel never
// holds a stale value after Stop or Reset, a ticker drops the ticks its
// receiver is not ready for, and a timer or ticker of a non-positive
// duration fires at once, or panics for NewTicker. A Clock is safe for
// concurrent use.
type Clock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*clockTimer // the active ones
}

// compile time check that *Clock implements flam.IClock
var _ flam.IClock = (*Clock)(nil)

// NewClock returns a Clock reading now.
func NewClock(now time.Time) *Clock {
	return &Clock{now: now}
}

// Advance moves the time forward by d, firing, in deadline order, the
// timers and tickers whose deadline is reached, each at its deadline: a
// ticker fires once per period elapsed, dropping the ticks its receiver is
// not ready for.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	target := c.now.Add(d)
	for {
		next := c.nextLocked(target)
		if next == nil {
			break
		}
		c.now = next.when
		next.fireLocked()
	}
	c.now = target
}

// Timers returns the number of timers and tickers that may still fire.
func (c *Clock) Timers() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.timers)
}

// Now returns the fake time.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Since returns the fake time elapsed since t.
func (c *Clock) Since(t time.Time) time.Duration { return c.Now().Sub(t) }

// Until returns the fake time left until t.
func (c *Clock) Until(t time.Time) time.Duration { return t.Sub(c.Now()) }

// ParseDuration delegates to time.ParseDuration.
func (c *Clock) ParseDuration(s string) (time.Duration, error) { return time.ParseDuration(s) }

// FixedZone delegates to time.FixedZone.
func (c *Clock) FixedZone(name string, offset int) *time.Location {
	return time.FixedZone(name, offset)
}

// LoadLocation delegates to time.LoadLocation.
func (c *Clock) LoadLocation(name string) (*time.Location, error) {
	return time.LoadLocation(name)
}

// Date delegates to time.Date.
func (c *Clock) Date(
	year int, month time.Month, day, hour, minute, sec, nsec int, loc *time.Location,
) time.Time {
	return time.Date(year, month, day, hour, minute, sec, nsec, loc)
}

// Parse delegates to time.Parse.
func (c *Clock) Parse(layout, value string) (time.Time, error) {
	return time.Parse(layout, value)
}

// ParseInLocation delegates to time.ParseInLocation.
func (c *Clock) ParseInLocation(layout, value string, loc *time.Location) (time.Time, error) {
	return time.ParseInLocation(layout, value, loc)
}

// Unix delegates to time.Unix.
func (c *Clock) Unix(sec, nsec int64) time.Time { return time.Unix(sec, nsec) }

// UnixMicro delegates to time.UnixMicro.
func (c *Clock) UnixMicro(usec int64) time.Time { return time.UnixMicro(usec) }

// UnixMilli delegates to time.UnixMilli.
func (c *Clock) UnixMilli(msec int64) time.Time { return time.UnixMilli(msec) }

// After returns the channel of a new timer of d.
func (c *Clock) After(d time.Duration) <-chan time.Time { return c.NewTimer(d).C() }

// Tick returns the channel of a new ticker of d, or nil when d is not
// positive.
func (c *Clock) Tick(d time.Duration) <-chan time.Time {
	if d <= 0 {
		return nil
	}
	return c.NewTicker(d).C()
}

// Sleep blocks until Advance reaches d from now, or returns at once when
// d is not positive.
func (c *Clock) Sleep(d time.Duration) {
	if d > 0 {
		<-c.After(d)
	}
}

// NewTimer returns a timer sending the fake time once Advance reaches d
// from now.
func (c *Clock) NewTimer(d time.Duration) flam.ITimer {
	return c.start(&clockTimer{clock: c, ch: make(chan time.Time, 1)}, d)
}

// AfterFunc returns a timer calling f on a goroutine of its own once
// Advance reaches d from now; its C method returns nil.
func (c *Clock) AfterFunc(d time.Duration, f func()) flam.ITimer {
	return c.start(&clockTimer{clock: c, f: f}, d)
}

// NewTicker returns a ticker sending the fake time each time Advance
// reaches a period of d, panicking when d is not positive.
func (c *Clock) NewTicker(d time.Duration) flam.ITicker {
	if d <= 0 {
		panic("non-positive interval for NewTicker")
	}
	return &clockTicker{c.start(&clockTimer{clock: c, ch: make(chan time.Time, 1), period: d}, d)}
}

// start schedules the timer d from now, firing it at once when d is not
// positive.
func (c *Clock) start(t *clockTimer, d time.Duration) *clockTimer {
	c.mu.Lock()
	defer c.mu.Unlock()
	t.scheduleLocked(d)
	return t
}

// nextLocked returns the active timer with the earliest deadline not after
// target, or nil.
func (c *Clock) nextLocked(target time.Time) *clockTimer {
	var next *clockTimer
	for _, t := range c.timers {
		if !t.when.After(target) && (next == nil || t.when.Before(next.when)) {
			next = t
		}
	}
	return next
}

// clockTimer is a timer, when period is zero, or a ticker of a Clock. Its
// fields are guarded by the mutex of the clock.
type clockTimer struct {
	clock  *Clock
	ch     chan time.Time // nil for an AfterFunc timer
	f      func()
	period time.Duration
	when   time.Time
}

func (t *clockTimer) C() <-chan time.Time { return t.ch }

func (t *clockTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	return t.stopLocked()
}

func (t *clockTimer) Reset(d time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	active := t.stopLocked()
	if t.period > 0 {
		t.period = d
	}
	t.scheduleLocked(d)
	return active
}

// clockTicker is the flam.ITicker view of a ticker clockTimer.
type clockTicker struct {
	*clockTimer
}

func (t *clockTicker) Stop() { t.clockTimer.Stop() }

func (t *clockTicker) Reset(d time.Duration) {
	if d <= 0 {
		panic("non-positive interval for Ticker.Reset")
	}
	t.clockTimer.Reset(d)
}

// stopLocked deactivates the timer and drops the value its channel holds,
// reporting whether it was active, a value not received counting as such.
func (t *clockTimer) stopLocked() bool {
	active := t.activeLocked()
	t.clock.timers = slices.DeleteFunc(t.clock.timers, func(o *clockTimer) bool { return o == t })
	if t.ch != nil {
		select {
		case <-t.ch:
			active = true
		default:
		}
	}
	return active
}

// scheduleLocked activates the timer d from now, firing it at once when d
// is not positive.
func (t *clockTimer) scheduleLocked(d time.Duration) {
	t.when = t.clock.now.Add(d)
	t.clock.timers = append(t.clock.timers, t)
	if d <= 0 {
		t.fireLocked()
	}
}

// fireLocked fires the timer at its deadline: it sends the time without
// blocking, or calls the function on a goroutine of its own, then
// reschedules a ticker one period later, or deactivates a timer.
func (t *clockTimer) fireLocked() {
	if t.f != nil {
		go t.f()
	} else {
		select {
		case t.ch <- t.when:
		default: // the receiver is not ready: the tick is dropped
		}
	}

	if t.period > 0 {
		t.when = t.when.Add(t.period)
		return
	}
	t.clock.timers = slices.DeleteFunc(t.clock.timers, func(o *clockTimer) bool { return o == t })
}

// activeLocked reports whether the timer may still fire.
func (t *clockTimer) activeLocked() bool {
	return slices.Contains(t.clock.timers, t)
}
