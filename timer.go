package flam

import (
	"time"
)

type Timer interface {
	ParseDuration(s string) (time.Duration, error)
	Since(t time.Time) time.Duration
	Until(t time.Time) time.Duration
	FixedZone(name string, offset int) *time.Location
	LoadLocation(name string) (*time.Location, error)
	Date(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) time.Time
	Now() time.Time
	Parse(layout, value string) (time.Time, error)
	ParseInLocation(layout, value string, loc *time.Location) (time.Time, error)
	Unix(sec int64, nsec int64) time.Time
	UnixMicro(usec int64) time.Time
	UnixMilli(msec int64) time.Time
}

type timer struct{}

var _ Timer = (*timer)(nil)

func newTimer() Timer {
	return &timer{}
}

func (timer timer) ParseDuration(
	s string,
) (time.Duration, error) {
	return time.ParseDuration(s)
}

func (timer timer) Since(
	t time.Time,
) time.Duration {
	return time.Since(t)
}

func (timer timer) Until(
	t time.Time,
) time.Duration {
	return time.Until(t)
}

func (timer timer) FixedZone(
	name string,
	offset int,
) *time.Location {
	return time.FixedZone(name, offset)
}

func (timer timer) LoadLocation(
	name string,
) (*time.Location, error) {
	return time.LoadLocation(name)
}

func (timer timer) Date(
	year int,
	month time.Month,
	day, hour, min, sec, nsec int,
	loc *time.Location,
) time.Time {
	return time.Date(year, month, day, hour, min, sec, nsec, loc)
}

func (timer timer) Now() time.Time {
	return time.Now()
}

func (timer timer) Parse(
	layout, value string,
) (time.Time, error) {
	return time.Parse(layout, value)
}

func (timer timer) ParseInLocation(
	layout, value string,
	loc *time.Location,
) (time.Time, error) {
	return time.ParseInLocation(layout, value, loc)
}

func (timer timer) Unix(
	sec int64,
	nsec int64,
) time.Time {
	return time.Unix(sec, nsec)
}

func (timer timer) UnixMicro(
	usec int64,
) time.Time {
	return time.UnixMicro(usec)
}

func (timer timer) UnixMilli(
	msec int64,
) time.Time {
	return time.UnixMilli(msec)
}
