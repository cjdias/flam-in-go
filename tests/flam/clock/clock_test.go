package clock

import (
	"errors"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	_ "time/tzdata" // the IANA database, whatever the system holds

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/clock"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newClock returns the IClock provided by the clock bundle.
func newClock(t *testing.T) flam.IClock {
	t.Helper()
	app := testutil.NewApp()
	testutil.NoError(t, clock.NewBundle().Register(app))

	var c flam.IClock
	testutil.NoError(t, app.Invoke(func(provided flam.IClock) { c = provided }))
	return c
}

// hasMonotonic reports whether t holds a monotonic clock reading.
func hasMonotonic(t time.Time) bool {
	return strings.Contains(t.String(), " m=")
}

// ---------------------------------------------------------------------------
// bundle
// ---------------------------------------------------------------------------

func Test_Bundle(t *testing.T) {
	t.Run("should provide a single IClock", func(t *testing.T) {
		b := clock.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.clock")

		app := testutil.NewApp()
		testutil.NoError(t, b.Register(app))
		testutil.Equal(t, "calls", app.Calls(), []string{"provider/0"})

		var first, second flam.IClock
		testutil.NoError(t, app.Invoke(func(c flam.IClock) { first = c }))
		testutil.NoError(t, app.Invoke(func(c flam.IClock) { second = c }))
		if first == nil || first != second {
			t.Fatal("the container did not provide a single IClock")
		}
	})

	t.Run("should have nothing to close", func(t *testing.T) {
		if _, ok := clock.NewBundle().(flam.IClosableBundle); ok {
			t.Error("the bundle implements IClosableBundle")
		}
	})
}

// ---------------------------------------------------------------------------
// IClock
// ---------------------------------------------------------------------------

func Test_Now(t *testing.T) {
	c := newClock(t)
	before := time.Now()
	now := c.Now()
	after := time.Now()

	if now.Before(before) || now.After(after) {
		t.Errorf("Now() = %v, not between %v and %v", now, before, after)
	}
	testutil.Equal(t, "location", now.Location(), time.Local)
	testutil.Equal(t, "monotonic", hasMonotonic(now), true)
	testutil.Equal(t, "Add keeps the reading", hasMonotonic(now.Add(time.Second)), true)
	for name, dropped := range map[string]time.Time{
		"AddDate": now.AddDate(0, 0, 1), "Round": now.Round(0), "Truncate": now.Truncate(time.Second),
		"In": now.In(time.UTC), "Local": now.Local(), "UTC": now.UTC(),
	} {
		if hasMonotonic(dropped) {
			t.Errorf("%s kept the monotonic reading", name)
		}
	}
}

func Test_SinceUntil(t *testing.T) {
	c := newClock(t)
	past, future := time.Now().Add(-time.Hour), time.Now().Add(time.Hour)

	if d := c.Since(past); d < time.Hour || d > time.Hour+time.Minute {
		t.Errorf("Since(an hour ago) = %v", d)
	}
	if d := c.Since(future); d > -time.Hour+time.Minute || d < -time.Hour {
		t.Errorf("Since(in an hour) = %v, want about -1h", d)
	}
	if d := c.Until(future); d > time.Hour || d < time.Hour-time.Minute {
		t.Errorf("Until(in an hour) = %v", d)
	}
	if d := c.Until(past); d > -time.Hour || d < -time.Hour-time.Minute {
		t.Errorf("Until(an hour ago) = %v, want about -1h", d)
	}
	testutil.Equal(t, "Since(zero)", c.Since(time.Time{}), time.Duration(math.MaxInt64))
	testutil.Equal(t, "Until(zero)", c.Until(time.Time{}), time.Duration(math.MinInt64))
}

func Test_ParseDuration(t *testing.T) {
	c := newClock(t)
	for s, want := range map[string]time.Duration{
		"1h30m": 90 * time.Minute, "-1.5s": -1500 * time.Millisecond, "2h45m30.5s": 2*time.Hour + 45*time.Minute + 30500*time.Millisecond,
		"0": 0, "+0": 0, "1us": time.Microsecond, "1µs": time.Microsecond, "1μs": time.Microsecond,
	} {
		got, err := c.ParseDuration(s)
		testutil.NoError(t, err)
		testutil.Equal(t, "ParseDuration("+s+")", got, want)
	}
	for _, s := range []string{"", "0.5", "1", "1x", "3000000h"} {
		_, err := c.ParseDuration(s)
		_, wantErr := time.ParseDuration(s)
		if err == nil || err.Error() != wantErr.Error() {
			t.Errorf("ParseDuration(%q) error = %v, want %v", s, err, wantErr)
		}
	}
}

func Test_Locations(t *testing.T) {
	c := newClock(t)

	zone := c.FixedZone("", -(5*3600 + 30*60))
	name, offset := time.Date(2024, 1, 1, 0, 0, 0, 0, zone).Zone()
	testutil.Equal(t, "FixedZone name", name, "")
	testutil.Equal(t, "FixedZone offset", offset, -(5*3600 + 30*60))

	for name, want := range map[string]*time.Location{"": time.UTC, "UTC": time.UTC, "Local": time.Local} {
		loc, err := c.LoadLocation(name)
		testutil.NoError(t, err)
		if loc != want {
			t.Errorf("LoadLocation(%q) = %v, want %v", name, loc, want)
		}
	}
	for _, name := range []string{"Not/AZone", "../etc/passwd"} {
		if _, err := c.LoadLocation(name); err == nil {
			t.Errorf("LoadLocation(%q) = nil error", name)
		}
	}
}

func Test_Date(t *testing.T) {
	c := newClock(t)
	testutil.Equal(t, "October 32", c.Date(2024, time.October, 32, 0, 0, 0, 0, time.UTC),
		time.Date(2024, time.November, 1, 0, 0, 0, 0, time.UTC))
	testutil.Equal(t, "nsec 1e9", c.Date(2024, time.January, 1, 0, 0, 0, 1e9, time.UTC),
		time.Date(2024, time.January, 1, 0, 0, 1, 0, time.UTC))
	testutil.Panics(t, "Date(nil location)", func() { c.Date(2024, time.January, 1, 0, 0, 0, 0, nil) })
}

func Test_Parse(t *testing.T) {
	c := newClock(t)

	got, err := c.Parse(time.DateTime, "2024-01-02 03:04:05")
	testutil.NoError(t, err)
	testutil.Equal(t, "no zone", got, time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC))

	got, err = c.Parse(time.RFC3339, "2024-01-02T03:04:05+02:00")
	testutil.NoError(t, err)
	want, _ := time.Parse(time.RFC3339, "2024-01-02T03:04:05+02:00")
	if !got.Equal(want) || got.Location().String() != want.Location().String() {
		t.Errorf("offset = %v, want %v", got, want)
	}

	_, err = c.Parse(time.DateOnly, "2024-02-30")
	var parseErr *time.ParseError
	if !errors.As(err, &parseErr) {
		t.Errorf("Parse(out of range) error = %T, want a *time.ParseError", err)
	}

	loc := time.FixedZone("X", 3600)
	got, err = c.ParseInLocation(time.DateTime, "2024-01-02 03:04:05", loc)
	testutil.NoError(t, err)
	testutil.Equal(t, "ParseInLocation", got, time.Date(2024, 1, 2, 3, 4, 5, 0, loc))

	testutil.Panics(t, "ParseInLocation(nil, no zone)", func() {
		_, _ = c.ParseInLocation(time.DateTime, "2024-01-02 03:04:05", nil)
	})
	testutil.NotPanics(t, "ParseInLocation(nil, offset)", func() {
		_, err := c.ParseInLocation(time.RFC3339, "2024-01-02T03:04:05+02:00", nil)
		testutil.NoError(t, err)
	})
}

func Test_Unix(t *testing.T) {
	c := newClock(t)
	testutil.Equal(t, "Unix", c.Unix(-1, 1_500_000_000), time.Unix(0, 500_000_000))
	testutil.Equal(t, "Unix location", c.Unix(0, 0).Location(), time.Local)
	testutil.Equal(t, "UnixMicro", c.UnixMicro(-1_500_000), time.UnixMicro(-1_500_000))
	testutil.Equal(t, "UnixMilli", c.UnixMilli(1_700_000_000_123), time.UnixMilli(1_700_000_000_123))
}

// ---------------------------------------------------------------------------
// timers and tickers
// ---------------------------------------------------------------------------

// patience is how long a test waits for a time it expects, generous enough
// for a loaded machine; quiet is how long it waits to conclude that no time
// arrives.
const (
	patience = 2 * time.Second
	quiet    = 50 * time.Millisecond
)

// receives reports whether a time is received from ch within patience.
func receives(ch <-chan time.Time) bool {
	select {
	case <-ch:
		return true
	case <-time.After(patience):
		return false
	}
}

// staysQuiet reports whether no time is received from ch within quiet.
func staysQuiet(ch <-chan time.Time) bool {
	select {
	case <-ch:
		return false
	case <-time.After(quiet):
		return true
	}
}

func Test_After(t *testing.T) {
	c := newClock(t)
	testutil.Equal(t, "After(10ms)", receives(c.After(10*time.Millisecond)), true)
	testutil.Equal(t, "After(-1)", receives(c.After(-1)), true)
}

func Test_Tick(t *testing.T) {
	c := newClock(t)
	for _, d := range []time.Duration{0, -time.Second} {
		if c.Tick(d) != nil {
			t.Errorf("Tick(%v) is not nil", d)
		}
	}
	ch := c.Tick(5 * time.Millisecond)
	testutil.Equal(t, "first tick", receives(ch), true)
	testutil.Equal(t, "second tick", receives(ch), true)
}

func Test_Sleep(t *testing.T) {
	c := newClock(t)
	start := time.Now()
	c.Sleep(20 * time.Millisecond)
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Errorf("Sleep(20ms) returned after %v", elapsed)
	}

	start = time.Now()
	c.Sleep(-time.Second)
	if elapsed := time.Since(start); elapsed > quiet {
		t.Errorf("Sleep(-1s) returned after %v", elapsed)
	}
}

func Test_NewTimer(t *testing.T) {
	c := newClock(t)

	t.Run("should fire once, on a stable channel", func(t *testing.T) {
		timer := c.NewTimer(10 * time.Millisecond)
		if first, second := timer.C(), timer.C(); first != second {
			t.Fatal("C() changed between calls")
		}
		testutil.Equal(t, "fired", receives(timer.C()), true)
		testutil.Equal(t, "fired twice", staysQuiet(timer.C()), true)
		testutil.Equal(t, "fired at once", receives(c.NewTimer(-1).C()), true)
	})

	t.Run("should not fire once stopped", func(t *testing.T) {
		timer := c.NewTimer(10 * time.Millisecond)
		testutil.Equal(t, "Stop()", timer.Stop(), true)
		testutil.Equal(t, "fired", staysQuiet(timer.C()), true)
		testutil.Equal(t, "Stop() again", timer.Stop(), false)
	})

	t.Run("should stop a timer past its time whose time was not received", func(t *testing.T) {
		timer := c.NewTimer(time.Millisecond)
		time.Sleep(20 * time.Millisecond)
		testutil.Equal(t, "Stop()", timer.Stop(), true)
		testutil.Equal(t, "stale time", staysQuiet(timer.C()), true)
	})

	t.Run("should fire again once reset", func(t *testing.T) {
		timer := c.NewTimer(time.Millisecond)
		testutil.Equal(t, "fired", receives(timer.C()), true)
		testutil.Equal(t, "Stop() after firing", timer.Stop(), false)
		testutil.Equal(t, "Reset() after firing", timer.Reset(5*time.Millisecond), false)
		testutil.Equal(t, "fired again", receives(timer.C()), true)

		running := c.NewTimer(time.Hour)
		testutil.Equal(t, "Reset() while running", running.Reset(5*time.Millisecond), true)
		testutil.Equal(t, "fired after reset", receives(running.C()), true)
	})
}

func Test_AfterFunc(t *testing.T) {
	c := newClock(t)

	t.Run("should call f, without a channel", func(t *testing.T) {
		called := make(chan time.Time, 2)
		timer := c.AfterFunc(5*time.Millisecond, func() { called <- time.Now() })
		if timer.C() != nil {
			t.Error("C() is not nil")
		}
		testutil.Equal(t, "called", receives(called), true)
		testutil.Equal(t, "Stop() after the call", timer.Stop(), false)

		testutil.Equal(t, "Reset() after the call", timer.Reset(5*time.Millisecond), false)
		testutil.Equal(t, "called again", receives(called), true)
	})

	t.Run("should not call f once stopped", func(t *testing.T) {
		called := make(chan time.Time, 1)
		timer := c.AfterFunc(10*time.Millisecond, func() { called <- time.Now() })
		testutil.Equal(t, "Stop()", timer.Stop(), true)
		testutil.Equal(t, "called", staysQuiet(called), true)
	})
}

func Test_NewTicker(t *testing.T) {
	c := newClock(t)

	t.Run("should tick until stopped, on a stable channel", func(t *testing.T) {
		ticker := c.NewTicker(5 * time.Millisecond)
		if first, second := ticker.C(), ticker.C(); first != second {
			t.Fatal("C() changed between calls")
		}
		for i := range 3 {
			if !receives(ticker.C()) {
				t.Fatalf("tick %d not received", i)
			}
		}
		ticker.Stop()
		testutil.Equal(t, "ticked after Stop()", staysQuiet(ticker.C()), true)
	})

	t.Run("should change its period, and restart, once reset", func(t *testing.T) {
		ticker := c.NewTicker(time.Hour)
		ticker.Reset(5 * time.Millisecond)
		testutil.Equal(t, "ticked after Reset()", receives(ticker.C()), true)

		ticker.Stop()
		ticker.Reset(5 * time.Millisecond)
		testutil.Equal(t, "ticked after Stop() then Reset()", receives(ticker.C()), true)
		ticker.Stop()
	})

	t.Run("should panic on a period of zero or less", func(t *testing.T) {
		testutil.Panics(t, "NewTicker(0)", func() { c.NewTicker(0) })
		testutil.Panics(t, "NewTicker(-1)", func() { c.NewTicker(-1) })
		ticker := c.NewTicker(time.Hour)
		testutil.Panics(t, "Reset(0)", func() { ticker.Reset(0) })
		ticker.Stop()
	})
}

// ---------------------------------------------------------------------------
// bundle failures
// ---------------------------------------------------------------------------

func Test_BundleFailures(t *testing.T) {
	t.Run("should fail to register on a nil application", func(t *testing.T) {
		var typedNil *testutil.App
		for _, app := range []flam.IApp{nil, typedNil} {
			testutil.Error(t, clock.NewBundle().Register(app), bundle.ErrNilApplication,
				"unable to register: nil registerer application", map[string]any{})
		}
	})

	t.Run("should return the error of the application providing the clock", func(t *testing.T) {
		cause := errors.New("boom")
		app := testutil.NewApp()
		app.Fail = func(string) error { return cause }
		testutil.Error(t, clock.NewBundle().Register(app), cause,
			"unable to register: boom", map[string]any{"index": 0, "kind": "provider"})
	})
}

// ---------------------------------------------------------------------------
// IClock edge cases
// ---------------------------------------------------------------------------

func Test_ParseDurationCases(t *testing.T) {
	c := newClock(t)
	for _, tc := range []struct {
		s    string
		want time.Duration
	}{
		{"1ns", time.Nanosecond},
		{"1us", time.Microsecond},
		{"1µs", time.Microsecond}, // micro sign
		{"1μs", time.Microsecond}, // Greek letter mu
		{"1ms", time.Millisecond},
		{"1s", time.Second},
		{"1m", time.Minute},
		{"1h", time.Hour},
		{"+1.5h", 90 * time.Minute},
		{"-1.5s", -1500 * time.Millisecond},
		{".5s", 500 * time.Millisecond},
		{"1.s", time.Second},
		{"1h30m", 90 * time.Minute},
		{"1m1h", 61 * time.Minute},
		{"0", 0},
		{"-0", 0},
		{"2562047h47m16.854775807s", math.MaxInt64},
		{"-2562047h47m16.854775808s", math.MinInt64},
	} {
		t.Run("should parse "+tc.s, func(t *testing.T) {
			got, err := c.ParseDuration(tc.s)
			testutil.NoError(t, err)
			testutil.Equal(t, "ParseDuration()", got, tc.want)
		})
	}
	for _, tc := range []struct{ name, s string }{
		{"an empty string", ""},
		{"a sign alone", "-"},
		{"a number without a unit", "1"},
		{"a fraction without a unit", "0.5"},
		{"a unit without a number", "s"},
		{"a dot without digits", ".s"},
		{"an unknown unit", "1d"},
		{"an upper case unit", "1S"},
		{"a space", "1 s"},
		{"an overflowing value", "2562047h47m16.854775808s"},
		{"an overflowing number of hours", "3000000h"},
	} {
		t.Run("should fail on "+tc.name, func(t *testing.T) {
			got, err := c.ParseDuration(tc.s)
			_, wantErr := time.ParseDuration(tc.s)
			if err == nil || wantErr == nil || err.Error() != wantErr.Error() {
				t.Errorf("ParseDuration(%q) error = %v, want %v", tc.s, err, wantErr)
			}
			testutil.Equal(t, "ParseDuration() on error", got, time.Duration(0))
		})
	}
}

func Test_SinceUntilCases(t *testing.T) {
	c := newClock(t)
	farFuture := time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("should be zero or positive and small since now", func(t *testing.T) {
		if d := c.Since(time.Now()); d < 0 || d > time.Minute {
			t.Errorf("Since(now) = %v", d)
		}
	})

	t.Run("should use the monotonic reading, keeping Since and Until coherent", func(t *testing.T) {
		start := c.Now()
		time.Sleep(time.Millisecond)
		since := c.Since(start)
		until := c.Until(start)
		if since <= 0 || until >= 0 {
			t.Errorf("Since() = %v, Until() = %v, want positive and negative", since, until)
		}
	})

	t.Run("should work on a time without monotonic reading", func(t *testing.T) {
		wall := time.Now().Add(-time.Hour).Round(0)
		if hasMonotonic(wall) {
			t.Fatal("Round(0) kept the monotonic reading")
		}
		if d := c.Since(wall); d < time.Hour || d > time.Hour+time.Minute {
			t.Errorf("Since(wall clock an hour ago) = %v", d)
		}
		if d := c.Until(wall); d > -time.Hour || d < -time.Hour-time.Minute {
			t.Errorf("Until(wall clock an hour ago) = %v", d)
		}
	})

	t.Run("should clamp a result too large for a time.Duration", func(t *testing.T) {
		testutil.Equal(t, "Since(zero)", c.Since(time.Time{}), time.Duration(math.MaxInt64))
		testutil.Equal(t, "Since(year 9999)", c.Since(farFuture), time.Duration(math.MinInt64))
		testutil.Equal(t, "Until(zero)", c.Until(time.Time{}), time.Duration(math.MinInt64))
		testutil.Equal(t, "Until(year 9999)", c.Until(farFuture), time.Duration(math.MaxInt64))
	})
}

func Test_FixedZoneCases(t *testing.T) {
	c := newClock(t)
	for _, tc := range []struct {
		name   string
		zone   string
		offset int
	}{
		{"a positive offset", "P", 2 * 3600},
		{"a negative offset", "N", -8 * 3600},
		{"a zero offset", "Z", 0},
		{"an offset that is not a whole number of hours", "H", 5*3600 + 45*60 + 30},
		{"an empty name", "", 3600},
	} {
		t.Run("should accept "+tc.name, func(t *testing.T) {
			loc := c.FixedZone(tc.zone, tc.offset)
			testutil.Equal(t, "String()", loc.String(), tc.zone)
			for _, instant := range []time.Time{
				time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2200, 12, 31, 0, 0, 0, 0, time.UTC),
			} {
				name, offset := instant.In(loc).Zone()
				testutil.Equal(t, "zone name", name, tc.zone)
				testutil.Equal(t, "zone offset", offset, tc.offset)
			}
		})
	}
}

func Test_LoadLocationCases(t *testing.T) {
	c := newClock(t)

	t.Run("should look a name up in the IANA time zone database", func(t *testing.T) {
		loc, err := c.LoadLocation("America/New_York")
		testutil.NoError(t, err)
		testutil.Equal(t, "String()", loc.String(), "America/New_York")
		name, offset := time.Date(2024, 1, 15, 12, 0, 0, 0, loc).Zone()
		testutil.Equal(t, "winter zone", []any{name, offset}, []any{"EST", -5 * 3600})
		name, offset = time.Date(2024, 7, 15, 12, 0, 0, 0, loc).Zone()
		testutil.Equal(t, "summer zone", []any{name, offset}, []any{"EDT", -4 * 3600})
	})

	for _, name := range []string{"Not/AZone", "../etc/passwd", "/etc/localtime", "America\\New_York", "Europe/"} {
		t.Run("should fail on "+name+", as the time package does", func(t *testing.T) {
			loc, err := c.LoadLocation(name)
			_, wantErr := time.LoadLocation(name)
			if err == nil || wantErr == nil || err.Error() != wantErr.Error() {
				t.Errorf("LoadLocation(%q) error = %v, want %v", name, err, wantErr)
			}
			if loc != nil {
				t.Errorf("LoadLocation(%q) = %v, want nil", name, loc)
			}
		})
	}
}

func Test_DateCases(t *testing.T) {
	c := newClock(t)
	utc := func(y int, m time.Month, d, h, mi, s, ns int) time.Time {
		return time.Date(y, m, d, h, mi, s, ns, time.UTC)
	}
	for _, tc := range []struct {
		name string
		got  time.Time
		want time.Time
	}{
		{"should keep a valid date", c.Date(2024, time.February, 29, 23, 59, 59, 999_999_999, time.UTC),
			utc(2024, time.February, 29, 23, 59, 59, 999_999_999)},
		{"should normalise October 32 to November 1", c.Date(2024, time.October, 32, 0, 0, 0, 0, time.UTC),
			utc(2024, time.November, 1, 0, 0, 0, 0)},
		{"should normalise February 29 of a common year", c.Date(2023, time.February, 29, 0, 0, 0, 0, time.UTC),
			utc(2023, time.March, 1, 0, 0, 0, 0)},
		{"should normalise a month 13", c.Date(2024, 13, 1, 0, 0, 0, 0, time.UTC),
			utc(2025, time.January, 1, 0, 0, 0, 0)},
		{"should normalise a month 0", c.Date(2024, 0, 1, 0, 0, 0, 0, time.UTC),
			utc(2023, time.December, 1, 0, 0, 0, 0)},
		{"should normalise a day 0", c.Date(2024, time.March, 0, 0, 0, 0, 0, time.UTC),
			utc(2024, time.February, 29, 0, 0, 0, 0)},
		{"should normalise a negative day", c.Date(2024, time.January, -1, 0, 0, 0, 0, time.UTC),
			utc(2023, time.December, 30, 0, 0, 0, 0)},
		{"should normalise an hour 24", c.Date(2024, time.December, 31, 24, 0, 0, 0, time.UTC),
			utc(2025, time.January, 1, 0, 0, 0, 0)},
		{"should normalise 60 minutes and 60 seconds", c.Date(2024, time.January, 1, 0, 60, 60, 0, time.UTC),
			utc(2024, time.January, 1, 1, 1, 0, 0)},
		{"should normalise a nsec of 1e9", c.Date(2024, time.January, 1, 0, 0, 0, 1e9, time.UTC),
			utc(2024, time.January, 1, 0, 0, 1, 0)},
		{"should normalise a negative nsec", c.Date(2024, time.January, 1, 0, 0, 0, -1, time.UTC),
			utc(2023, time.December, 31, 23, 59, 59, 999_999_999)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Equal(t, "Date()", tc.got, tc.want)
		})
	}

	t.Run("should return the time in the given location", func(t *testing.T) {
		loc := time.FixedZone("X", -3*3600)
		got := c.Date(2024, time.January, 1, 12, 0, 0, 0, loc)
		testutil.Equal(t, "location", got.Location(), loc)
		testutil.Equal(t, "UTC", got.UTC(), utc(2024, time.January, 1, 15, 0, 0, 0))
	})

	t.Run("should resolve the times of a daylight saving transition as the time package does", func(t *testing.T) {
		loc, err := time.LoadLocation("America/New_York")
		testutil.NoError(t, err)
		skipped := c.Date(2024, time.March, 10, 2, 30, 0, 0, loc)
		testutil.Equal(t, "skipped", skipped, time.Date(2024, time.March, 10, 2, 30, 0, 0, loc))
		if !skipped.Equal(utc(2024, time.March, 10, 6, 30, 0, 0)) && !skipped.Equal(utc(2024, time.March, 10, 7, 30, 0, 0)) {
			t.Errorf("skipped time = %v, not one of its possible instants", skipped.UTC())
		}
		repeated := c.Date(2024, time.November, 3, 1, 30, 0, 0, loc)
		testutil.Equal(t, "repeated", repeated, time.Date(2024, time.November, 3, 1, 30, 0, 0, loc))
		if !repeated.Equal(utc(2024, time.November, 3, 5, 30, 0, 0)) && !repeated.Equal(utc(2024, time.November, 3, 6, 30, 0, 0)) {
			t.Errorf("repeated time = %v, not one of its possible instants", repeated.UTC())
		}
	})

	t.Run("should panic on a nil location", func(t *testing.T) {
		testutil.Panics(t, "Date(nil location)", func() { c.Date(2024, time.January, 1, 0, 0, 0, 0, nil) })
	})
}

func Test_ParseCases(t *testing.T) {
	c := newClock(t)

	t.Run("should return a time without zone information in UTC", func(t *testing.T) {
		got, err := c.Parse(time.DateOnly, "2024-01-02")
		testutil.NoError(t, err)
		testutil.Equal(t, "Parse()", got, time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC))
		testutil.Equal(t, "location", got.Location(), time.UTC)
	})

	t.Run("should return a time with a zone offset in a fixed zone of that offset", func(t *testing.T) {
		at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		_, localOffset := at.In(time.Local).Zone()
		offset := localOffset + 3600 + 1800 // never the local one
		value := at.In(time.FixedZone("", offset)).Format(time.RFC3339)
		got, err := c.Parse(time.RFC3339, value)
		testutil.NoError(t, err)
		if !got.Equal(at) {
			t.Errorf("Parse() = %v, want %v", got, at)
		}
		if got.Location() == time.Local || got.Location() == time.UTC {
			t.Errorf("location = %v, want a fixed zone", got.Location())
		}
		_, gotOffset := got.Zone()
		testutil.Equal(t, "offset", gotOffset, offset)
	})

	t.Run("should return a time with the local zone offset in time.Local", func(t *testing.T) {
		const layout = "2006-01-02T15:04:05-07:00" // "+00:00" rather than "Z" for a zero offset
		at := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
		value := at.In(time.Local).Format(layout)
		got, err := c.Parse(layout, value)
		testutil.NoError(t, err)
		if !got.Equal(at) {
			t.Errorf("Parse() = %v, want %v", got, at)
		}
		testutil.Equal(t, "location", got.Location(), time.Local)
	})

	t.Run("should fabricate a zone with a zero offset for an abbreviation unknown to time.Local", func(t *testing.T) {
		got, err := c.Parse(time.RFC1123, "Tue, 02 Jan 2024 03:04:05 QQQ")
		testutil.NoError(t, err)
		name, offset := got.Zone()
		testutil.Equal(t, "zone name", name, "QQQ")
		testutil.Equal(t, "zone offset", offset, 0)
	})

	for _, tc := range []struct{ name, layout, value string }{
		{"a value not matching the layout", time.DateTime, "not a time"},
		{"an out of range value", time.DateOnly, "2024-02-30"},
		{"extra text after the value", time.DateOnly, "2024-01-02 extra"},
		{"an empty value", time.DateOnly, ""},
	} {
		t.Run("should return a *time.ParseError on "+tc.name, func(t *testing.T) {
			_, err := c.Parse(tc.layout, tc.value)
			var parseErr *time.ParseError
			if !errors.As(err, &parseErr) {
				t.Fatalf("Parse() error = %T, want a *time.ParseError", err)
			}
			_, wantErr := time.Parse(tc.layout, tc.value)
			testutil.Equal(t, "Error()", err.Error(), wantErr.Error())
		})
	}
}

func Test_ParseInLocationCases(t *testing.T) {
	c := newClock(t)
	ny, err := time.LoadLocation("America/New_York")
	testutil.NoError(t, err)

	t.Run("should interpret a time without zone information in the location", func(t *testing.T) {
		got, err := c.ParseInLocation(time.DateTime, "2024-07-01 12:00:00", ny)
		testutil.NoError(t, err)
		testutil.Equal(t, "ParseInLocation()", got, time.Date(2024, 7, 1, 12, 0, 0, 0, ny))
	})

	t.Run("should resolve a zone offset against the location", func(t *testing.T) {
		got, err := c.ParseInLocation(time.RFC3339, "2024-01-02T03:04:05-05:00", ny)
		testutil.NoError(t, err)
		testutil.Equal(t, "location", got.Location(), ny)
		testutil.Equal(t, "UTC", got.UTC(), time.Date(2024, 1, 2, 8, 4, 5, 0, time.UTC))
	})

	t.Run("should resolve a zone abbreviation against the location", func(t *testing.T) {
		got, err := c.ParseInLocation(time.RFC1123, "Tue, 02 Jan 2024 03:04:05 EST", ny)
		testutil.NoError(t, err)
		testutil.Equal(t, "location", got.Location(), ny)
		testutil.Equal(t, "UTC", got.UTC(), time.Date(2024, 1, 2, 8, 4, 5, 0, time.UTC))
	})

	t.Run("should return a *time.ParseError on a value not matching the layout", func(t *testing.T) {
		_, err := c.ParseInLocation(time.DateTime, "nope", ny)
		var parseErr *time.ParseError
		if !errors.As(err, &parseErr) {
			t.Errorf("ParseInLocation() error = %T, want a *time.ParseError", err)
		}
	})
}

func Test_UnixCases(t *testing.T) {
	c := newClock(t)
	for _, tc := range []struct {
		name      string
		got, want time.Time
	}{
		{"should return the epoch for zero", c.Unix(0, 0), time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"should accept negative seconds", c.Unix(-86400, 0), time.Date(1969, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"should normalise a nsec above 999999999 into sec", c.Unix(-1, 1_500_000_000),
			time.Date(1970, 1, 1, 0, 0, 0, 500_000_000, time.UTC)},
		{"should normalise a negative nsec into sec", c.Unix(1, -1),
			time.Date(1970, 1, 1, 0, 0, 0, 999_999_999, time.UTC)},
		{"should convert microseconds", c.UnixMicro(1_500_000), time.Date(1970, 1, 1, 0, 0, 1, 500_000_000, time.UTC)},
		{"should accept negative microseconds", c.UnixMicro(-1), time.Date(1969, 12, 31, 23, 59, 59, 999_999_000, time.UTC)},
		{"should convert milliseconds", c.UnixMilli(1_500), time.Date(1970, 1, 1, 0, 0, 1, 500_000_000, time.UTC)},
		{"should accept negative milliseconds", c.UnixMilli(-1), time.Date(1969, 12, 31, 23, 59, 59, 999_000_000, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.got.Equal(tc.want) {
				t.Errorf("got %v, want %v", tc.got.UTC(), tc.want)
			}
			testutil.Equal(t, "location", tc.got.Location(), time.Local)
			if hasMonotonic(tc.got) {
				t.Error("the time holds a monotonic reading")
			}
		})
	}
}

func Test_ConcurrentUse(t *testing.T) {
	t.Run("should be safe for concurrent use", func(t *testing.T) {
		c := newClock(t)
		timer := c.NewTimer(time.Hour)
		ticker := c.NewTicker(time.Hour)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 20 {
					_ = c.Now()
					_ = c.Since(c.Now())
					_, _ = c.ParseDuration("1s")
					_ = timer.Reset(time.Hour)
					_ = timer.C()
					ticker.Reset(time.Hour)
					_ = ticker.C()
				}
			})
		}
		wg.Wait()
		timer.Stop()
		ticker.Stop()
	})
}

// ---------------------------------------------------------------------------
// timers and tickers edge cases
// ---------------------------------------------------------------------------

// receivesAt returns the time received from ch within patience, and
// whether one was.
func receivesAt(ch <-chan time.Time) (time.Time, bool) {
	select {
	case at := <-ch:
		return at, true
	case <-time.After(patience):
		return time.Time{}, false
	}
}

// collected reports whether the value is recovered by the garbage collector
// once no longer referenced, within patience.
func collected(newValue func() any) bool {
	done := make(chan struct{})
	func() {
		v := newValue()
		runtime.SetFinalizer(v, func(any) { close(done) })
	}()
	deadline := time.Now().Add(patience)
	for time.Now().Before(deadline) {
		runtime.GC()
		select {
		case <-done:
			return true
		case <-time.After(time.Millisecond):
		}
	}
	return false
}

func Test_AfterCases(t *testing.T) {
	c := newClock(t)

	t.Run("should send the current time once the duration elapsed", func(t *testing.T) {
		start := time.Now()
		at, ok := receivesAt(c.After(10 * time.Millisecond))
		testutil.Equal(t, "received", ok, true)
		if at.Sub(start) < 10*time.Millisecond {
			t.Errorf("sent after %v, want at least 10ms", at.Sub(start))
		}
	})

	t.Run("should send once", func(t *testing.T) {
		ch := c.After(time.Millisecond)
		testutil.Equal(t, "received", receives(ch), true)
		testutil.Equal(t, "sent twice", staysQuiet(ch), true)
	})

	for _, d := range []time.Duration{0, -time.Hour} {
		t.Run("should send at once for a duration of "+d.String(), func(t *testing.T) {
			testutil.Equal(t, "received", receives(c.After(d)), true)
		})
	}
}

func Test_TickCases(t *testing.T) {
	c := newClock(t)

	t.Run("should return a nil channel, blocking forever, for a duration of zero or less", func(t *testing.T) {
		ch := c.Tick(-time.Nanosecond)
		if ch != nil {
			t.Fatal("Tick(-1ns) is not nil")
		}
		testutil.Equal(t, "received", staysQuiet(ch), true)
	})

	t.Run("should tick at the period", func(t *testing.T) {
		start := time.Now()
		ch := c.Tick(5 * time.Millisecond)
		first, ok := receivesAt(ch)
		testutil.Equal(t, "first tick", ok, true)
		if first.Sub(start) < 5*time.Millisecond {
			t.Errorf("first tick after %v, want at least 5ms", first.Sub(start))
		}
	})
}

func Test_SleepCases(t *testing.T) {
	c := newClock(t)
	for _, d := range []time.Duration{0, -time.Hour} {
		t.Run("should return at once for a duration of "+d.String(), func(t *testing.T) {
			start := time.Now()
			c.Sleep(d)
			if elapsed := time.Since(start); elapsed > quiet {
				t.Errorf("Sleep(%v) returned after %v", d, elapsed)
			}
		})
	}
}

func Test_NewTimerCases(t *testing.T) {
	c := newClock(t)

	t.Run("should send the current time once the duration elapsed", func(t *testing.T) {
		start := time.Now()
		at, ok := receivesAt(c.NewTimer(10 * time.Millisecond).C())
		testutil.Equal(t, "received", ok, true)
		if at.Sub(start) < 10*time.Millisecond {
			t.Errorf("sent after %v, want at least 10ms", at.Sub(start))
		}
	})

	t.Run("should fire at once for a duration of zero", func(t *testing.T) {
		testutil.Equal(t, "received", receives(c.NewTimer(0).C()), true)
	})

	t.Run("should return false from Stop and Reset once stopped, and fire again once reset", func(t *testing.T) {
		timer := c.NewTimer(time.Hour)
		testutil.Equal(t, "Stop()", timer.Stop(), true)
		testutil.Equal(t, "Reset() after Stop()", timer.Reset(5*time.Millisecond), false)
		testutil.Equal(t, "fired after Reset()", receives(timer.C()), true)
	})

	t.Run("should drop the unreceived time of the previous settings on Reset", func(t *testing.T) {
		timer := c.NewTimer(time.Millisecond)
		time.Sleep(20 * time.Millisecond) // past its time, not received
		testutil.Equal(t, "Reset() of a timer whose time was not received", timer.Reset(time.Hour), true)
		testutil.Equal(t, "stale time", staysQuiet(timer.C()), true)
		testutil.Equal(t, "Stop()", timer.Stop(), true)
	})

	t.Run("should fire once after several Resets", func(t *testing.T) {
		timer := c.NewTimer(time.Hour)
		for range 5 {
			testutil.Equal(t, "Reset() while running", timer.Reset(5*time.Millisecond), true)
		}
		testutil.Equal(t, "fired", receives(timer.C()), true)
		testutil.Equal(t, "fired twice", staysQuiet(timer.C()), true)
	})

	t.Run("should be recovered by the garbage collector without being stopped", func(t *testing.T) {
		testutil.Equal(t, "collected", collected(func() any { return c.NewTimer(time.Hour) }), true)
	})
}

func Test_AfterFuncCases(t *testing.T) {
	c := newClock(t)

	t.Run("should call f once, in its own goroutine", func(t *testing.T) {
		release := make(chan struct{})
		started := make(chan time.Time, 2)
		timer := c.AfterFunc(time.Millisecond, func() {
			started <- time.Now()
			<-release // blocking f does not block the caller
		})
		testutil.Equal(t, "called", receives(started), true)
		testutil.Equal(t, "called twice", staysQuiet(started), true)

		stopped := make(chan bool, 1)
		go func() { stopped <- timer.Stop() }()
		select {
		case got := <-stopped:
			testutil.Equal(t, "Stop() while f runs", got, false) // without waiting for f
		case <-time.After(patience):
			t.Error("Stop() waited for f to complete")
		}
		close(release)
	})

	t.Run("should call f once the new duration elapsed when reset while running", func(t *testing.T) {
		called := make(chan time.Time, 2)
		timer := c.AfterFunc(time.Hour, func() { called <- time.Now() })
		testutil.Equal(t, "Reset() while running", timer.Reset(5*time.Millisecond), true)
		testutil.Equal(t, "called", receives(called), true)
		testutil.Equal(t, "called twice", staysQuiet(called), true)
	})

	t.Run("should call f again when reset after being stopped", func(t *testing.T) {
		called := make(chan time.Time, 2)
		timer := c.AfterFunc(time.Hour, func() { called <- time.Now() })
		testutil.Equal(t, "Stop()", timer.Stop(), true)
		testutil.Equal(t, "Stop() again", timer.Stop(), false)
		testutil.Equal(t, "Reset() after Stop()", timer.Reset(5*time.Millisecond), false)
		testutil.Equal(t, "called", receives(called), true)
	})

	t.Run("should not refuse a nil f", func(t *testing.T) {
		testutil.NotPanics(t, "AfterFunc(1h, nil)", func() {
			timer := c.AfterFunc(time.Hour, nil)
			if timer.C() != nil {
				t.Error("C() is not nil")
			}
			testutil.Equal(t, "Stop()", timer.Stop(), true)
		})
	})

	t.Run("should crash the program when a nil f fires", func(t *testing.T) {
		if os.Getenv("FLAM_CLOCK_NIL_AFTERFUNC") == "1" {
			c.AfterFunc(0, nil)
			time.Sleep(patience)
			os.Exit(0) // not reached: the program crashes first
		}
		cmd := exec.Command(os.Args[0], "-test.run=^Test_AfterFuncCases$/^should_crash_the_program_when_a_nil_f_fires$")
		cmd.Env = append(os.Environ(), "FLAM_CLOCK_NIL_AFTERFUNC=1")
		out, err := cmd.CombinedOutput()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("the program did not crash: error = %v, output:\n%s", err, out)
		}
		// the documentation tells of a panic, but the runtime crashes with
		// the fatal error "go of nil func value" instead, which no recover
		// can catch either: only the crash is asserted
	})
}

func Test_NewTickerCases(t *testing.T) {
	c := newClock(t)

	t.Run("should tick at the period", func(t *testing.T) {
		start := time.Now()
		ticker := c.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		first, ok := receivesAt(ticker.C())
		testutil.Equal(t, "first tick", ok, true)
		if first.Sub(start) < 10*time.Millisecond {
			t.Errorf("first tick after %v, want at least 10ms", first.Sub(start))
		}
	})

	t.Run("should drop the ticks a slow receiver is not ready for", func(t *testing.T) {
		ticker := c.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		time.Sleep(100 * time.Millisecond) // about 20 periods, none received
		pending := 0
		for done := false; !done; {
			select {
			case <-ticker.C():
				pending++
			default:
				done = true
			}
		}
		if pending > 5 {
			t.Errorf("%d ticks were queued, want them dropped", pending)
		}
	})

	t.Run("should not close its channel on Stop", func(t *testing.T) {
		ticker := c.NewTicker(time.Hour)
		ticker.Stop()
		select {
		case _, ok := <-ticker.C():
			t.Errorf("received from the channel after Stop(), open = %v", ok)
		case <-time.After(quiet):
		}
	})

	t.Run("should accept Stop more than once", func(t *testing.T) {
		ticker := c.NewTicker(time.Hour)
		testutil.NotPanics(t, "Stop() twice", func() { ticker.Stop(); ticker.Stop() })
	})

	t.Run("should send the next tick once the new period elapsed from Reset", func(t *testing.T) {
		ticker := c.NewTicker(time.Hour)
		defer ticker.Stop()
		start := time.Now()
		ticker.Reset(10 * time.Millisecond)
		at, ok := receivesAt(ticker.C())
		testutil.Equal(t, "ticked", ok, true)
		if at.Sub(start) < 10*time.Millisecond {
			t.Errorf("ticked after %v, want at least 10ms", at.Sub(start))
		}
	})

	t.Run("should drop the unreceived tick of the previous period on Reset", func(t *testing.T) {
		ticker := c.NewTicker(time.Millisecond)
		defer ticker.Stop()
		time.Sleep(20 * time.Millisecond) // a tick is pending
		ticker.Reset(time.Hour)
		testutil.Equal(t, "stale tick", staysQuiet(ticker.C()), true)
	})

	for _, d := range []time.Duration{0, -time.Nanosecond, -time.Hour} {
		t.Run("should panic on Reset with a period of "+d.String(), func(t *testing.T) {
			ticker := c.NewTicker(time.Hour)
			defer ticker.Stop()
			testutil.Panics(t, "Reset()", func() { ticker.Reset(d) })
		})
		t.Run("should panic on NewTicker with a period of "+d.String(), func(t *testing.T) {
			testutil.Panics(t, "NewTicker()", func() { c.NewTicker(d) })
		})
	}

	t.Run("should be recovered by the garbage collector without being stopped", func(t *testing.T) {
		testutil.Equal(t, "collected", collected(func() any { return c.NewTicker(time.Hour) }), true)
	})
}
