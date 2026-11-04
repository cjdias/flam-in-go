package clock

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newClock returns a new [flam.IClock] reading the system clock. It is the
// provider registered by the bundle returned from [NewBundle].
func newClock() flam.IClock {
	return &clockImpl{}
}

// clockImpl is the [flam.IClock] reading the system clock, delegating every
// method to the time package.
type clockImpl struct{}

// compile time check that *clockImpl implements IClock
var _ flam.IClock = (*clockImpl)(nil)

func (c *clockImpl) ParseDuration(
	s string,
) (time.Duration, error) {
	return time.ParseDuration(s)
}

func (c *clockImpl) Since(
	t time.Time,
) time.Duration {
	return time.Since(t)
}

func (c *clockImpl) Until(
	t time.Time,
) time.Duration {
	return time.Until(t)
}

func (c *clockImpl) FixedZone(
	name string,
	offset int,
) *time.Location {
	return time.FixedZone(name, offset)
}

func (c *clockImpl) LoadLocation(
	name string,
) (*time.Location, error) {
	return time.LoadLocation(name)
}

func (c *clockImpl) Date(
	year int,
	month time.Month,
	day, hour, minute, sec, nsec int,
	loc *time.Location,
) time.Time {
	return time.Date(year, month, day, hour, minute, sec, nsec, loc)
}

func (c *clockImpl) Now() time.Time {
	return time.Now()
}

func (c *clockImpl) Parse(
	layout, value string,
) (time.Time, error) {
	return time.Parse(layout, value)
}

func (c *clockImpl) ParseInLocation(
	layout, value string,
	loc *time.Location,
) (time.Time, error) {
	return time.ParseInLocation(layout, value, loc)
}

func (c *clockImpl) Unix(
	sec int64,
	nsec int64,
) time.Time {
	return time.Unix(sec, nsec)
}

func (c *clockImpl) UnixMicro(
	usec int64,
) time.Time {
	return time.UnixMicro(usec)
}

func (c *clockImpl) UnixMilli(
	msec int64,
) time.Time {
	return time.UnixMilli(msec)
}

func (c *clockImpl) After(
	d time.Duration,
) <-chan time.Time {
	return time.After(d)
}

func (c *clockImpl) Tick(
	d time.Duration,
) <-chan time.Time {
	return time.Tick(d)
}

func (c *clockImpl) Sleep(
	d time.Duration,
) {
	time.Sleep(d)
}

func (c *clockImpl) NewTimer(
	d time.Duration,
) flam.ITimer {
	return &timerImpl{timer: time.NewTimer(d)}
}

func (c *clockImpl) AfterFunc(
	d time.Duration,
	f func(),
) flam.ITimer {
	return &timerImpl{timer: time.AfterFunc(d, f)}
}

func (c *clockImpl) NewTicker(
	d time.Duration,
) flam.ITicker {
	return &tickerImpl{ticker: time.NewTicker(d)}
}
