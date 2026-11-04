package flam

import (
	"time"
)

// IClock gives access to the current time, to the timers and tickers, and
// to the time constructors and parsers of the [time] package. Code reading
// the time or waiting through an IClock, instead of calling the time package
// directly, can be tested with a fake IClock returning a controlled time,
// whose timers and tickers fire when the fake time advances rather than when
// the real time does: the timers and tickers are returned as [ITimer] and
// [ITicker] values for that reason.
//
// The IClock provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/clock] reads the system clock,
// and each of its methods behaves exactly as the time package function of
// the same name, including the edge cases documented below. A fake IClock
// should follow the same rules.
//
// An IClock is safe for concurrent use.
type IClock interface {
	// ParseDuration parses a duration string, as [time.ParseDuration]: a
	// possibly signed sequence of decimal numbers, each with an optional
	// fraction and a unit suffix among "ns", "us" (or "µs", with the micro
	// sign or the Greek letter mu), "ms", "s", "m" and "h", such as
	// "1h30m", "-1.5s" or "2h45m30.5s". "0" is the only number allowed
	// without a unit. An empty string, a number without a unit, an unknown
	// unit or a value overflowing a time.Duration (about 292 years) returns
	// an error.
	ParseDuration(s string) (time.Duration, error)

	// Since returns the time elapsed since t, as [time.Since]: Now
	// minus t, negative when t is in the future. When t holds a monotonic
	// clock reading, as the times returned by Now do, the result is
	// computed with it and is not affected by changes of the wall clock. A
	// result too large for a time.Duration, as for the zero time.Time, is
	// clamped to the maximum or minimum duration.
	Since(t time.Time) time.Duration

	// Until returns the duration until t, as [time.Until]: t minus Now,
	// negative when t is in the past. It uses the monotonic clock reading
	// of t, when present, and clamps a result too large for a
	// time.Duration, as Since does.
	Until(t time.Time) time.Duration

	// FixedZone returns a location always using the given name as zone
	// abbreviation and offset, in seconds east of UTC, as
	// [time.FixedZone]. The offset may be negative, zero or not a whole
	// number of hours, and the name may be empty.
	FixedZone(name string, offset int) *time.Location

	// LoadLocation returns the location with the given name, as
	// [time.LoadLocation]: "" and "UTC" return [time.UTC], "Local"
	// returns [time.Local], and any other name, such as
	// "America/New_York", is looked up in the IANA time zone database,
	// read, in this order, from the directory or zip file named by the
	// ZONEINFO environment variable, the system database, the
	// $GOROOT/lib/time/zoneinfo.zip file, or the copy embedded by importing
	// the time/tzdata package. An unknown or malformed name returns an
	// error.
	LoadLocation(name string) (*time.Location, error)

	// Date returns the time corresponding to the given date in loc, as
	// [time.Date]. Values outside their usual range are normalised, so
	// October 32 is November 1 and a nsec of 1e9 adds a second. A time
	// skipped or repeated by a daylight saving transition in loc is
	// resolved to one of its possible instants, without guarantee of
	// which. Date panics when loc is nil.
	Date(year int, month time.Month, day, hour, minute, sec, nsec int, loc *time.Location) time.Time

	// Now returns the current local time, as [time.Now], in [time.Local]
	// and holding a monotonic clock reading used by Since, Until and the
	// time.Time comparison methods. Add keeps the reading in its result,
	// while AddDate, Round, Truncate, In, Local and UTC drop it.
	Now() time.Time

	// Parse parses a formatted time string, as [time.Parse], following
	// layout, which is written with the reference time
	// "Mon Jan 2 15:04:05 MST 2006" as the predefined time.RFC3339 or
	// time.DateTime are. A time without zone information is returned in
	// UTC. A time with a zone offset is returned in a fixed zone with that
	// offset, or in time.Local when the offset is the local one. A zone
	// abbreviation is resolved against time.Local, or else gives a
	// fabricated zone with that name and a zero offset. A value not
	// matching the layout returns an error of type *time.ParseError.
	Parse(layout, value string) (time.Time, error)

	// ParseInLocation parses a formatted time string, as
	// [time.ParseInLocation]. It works as Parse, but a time without zone
	// information is interpreted in loc, and a zone offset or abbreviation
	// is resolved against loc rather than time.Local. ParseInLocation
	// panics when loc is nil and the value has no zone information.
	ParseInLocation(layout, value string, loc *time.Location) (time.Time, error)

	// Unix returns the local time corresponding to the given number of
	// seconds and nanoseconds since January 1, 1970 UTC, as [time.Unix].
	// Both may be negative, and nsec may be outside [0, 999999999], in
	// which case it is normalised into sec.
	Unix(sec int64, nsec int64) time.Time

	// UnixMicro returns the local time corresponding to the given number
	// of microseconds since January 1, 1970 UTC, as [time.UnixMicro]. The
	// value may be negative.
	UnixMicro(usec int64) time.Time

	// UnixMilli returns the local time corresponding to the given number
	// of milliseconds since January 1, 1970 UTC, as [time.UnixMilli]. The
	// value may be negative.
	UnixMilli(msec int64) time.Time

	// After waits for the duration to elapse and then sends the current
	// time on the returned channel, as [time.After]: it is the channel of
	// NewTimer(d), whose timer cannot be stopped. A duration of zero or
	// less sends at once.
	After(d time.Duration) <-chan time.Time

	// Tick returns the channel of a new ticker with the period d, as
	// [time.Tick], whose ticker cannot be stopped. Unlike NewTicker, it
	// returns nil, which blocks forever when received from, when d is zero
	// or less.
	Tick(d time.Duration) <-chan time.Time

	// Sleep pauses the calling goroutine for at least the duration, as
	// [time.Sleep]. A duration of zero or less returns at once.
	Sleep(d time.Duration)

	// NewTimer returns a new timer sending the current time on its channel
	// once d has elapsed, as [time.NewTimer]. A duration of zero or less
	// fires at once.
	NewTimer(d time.Duration) ITimer

	// AfterFunc returns a new timer calling f, in its own goroutine, once
	// d has elapsed, as [time.AfterFunc]. The timer has no channel: its C
	// method returns nil. A nil f is not refused, and crashes the program
	// when the timer fires, with a fatal error that cannot be recovered.
	AfterFunc(d time.Duration, f func()) ITimer

	// NewTicker returns a new ticker sending the current time on its
	// channel every period d, as [time.NewTicker]. The ticker adjusts the
	// interval or drops ticks to make up for slow receivers. NewTicker
	// panics when d is zero or less.
	NewTicker(d time.Duration) ITicker
}

// ITimer is a single event timer of an [IClock], as a [time.Timer]: it
// sends the current time on its channel, or calls its function for a timer
// made by AfterFunc, once its duration has elapsed, unless stopped first.
//
// The channel is unbuffered: a time is only sent while a receiver waits, so
// no stale time remains in it after Stop or Reset returns. A timer that is
// no longer referenced is recovered by the garbage collector, even when it
// has not fired or been stopped.
//
// An ITimer is safe for concurrent use.
type ITimer interface {
	// C returns the channel on which the timer sends the time it fires at,
	// the same on every call, or nil for a timer made by AfterFunc.
	C() <-chan time.Time

	// Stop prevents the timer from firing, as [time.Timer.Stop]. It returns
	// true when it stopped the timer, and false when the timer had already
	// fired or been stopped. A timer whose time was not received yet is
	// still running, so Stop returns true and the time is never received.
	// For a timer made by AfterFunc, false means that f was started in its
	// own goroutine; Stop does not wait for f to complete.
	Stop() bool

	// Reset changes the timer to fire once d has elapsed from now, as
	// [time.Timer.Reset], whether it was running, had fired or had been
	// stopped. It returns true when the timer was running, and false when it
	// had fired or been stopped. After Reset returns, no time from the
	// previous settings is received from the channel. For a timer made by
	// AfterFunc, false means that f is scheduled to run again, maybe at the
	// same time as its previous run, which Reset does not wait for.
	Reset(d time.Duration) bool
}

// ITicker is a periodic timer of an [IClock], as a [time.Ticker]: it sends
// the current time on its channel every period, until stopped.
//
// The channel is unbuffered, and the ticker drops the ticks a slow receiver
// is not ready for, rather than queueing them. A ticker that is no longer
// referenced is recovered by the garbage collector, even when it has not
// been stopped.
//
// An ITicker is safe for concurrent use.
type ITicker interface {
	// C returns the channel on which the ticker sends the time of each
	// tick, the same on every call.
	C() <-chan time.Time

	// Stop turns the ticker off, as [time.Ticker.Stop]: no tick is sent
	// after it returns. It does not close the channel, so that a receiver
	// does not read a zero time from it.
	Stop()

	// Reset stops the ticker and changes its period to d, as
	// [time.Ticker.Reset]: the next tick arrives once d has elapsed from
	// now. A stopped ticker restarts. Reset panics when d is zero or less.
	Reset(d time.Duration)
}
