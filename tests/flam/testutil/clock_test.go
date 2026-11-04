package testutil_test

import (
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var start = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// pending returns the value ch holds, without waiting, and whether it held
// one: the Clock sends without blocking, so a fired timer holds its value.
func pending(ch <-chan time.Time) (time.Time, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		return time.Time{}, false
	}
}

func Test_Clock_Time(t *testing.T) {
	t.Run("should only move with Advance", func(t *testing.T) {
		c := testutil.NewClock(start)
		testutil.Equal(t, "Now()", c.Now(), start)
		c.Advance(time.Hour)
		testutil.Equal(t, "Now()", c.Now(), start.Add(time.Hour))
		testutil.Equal(t, "Since()", c.Since(start), time.Hour)
		testutil.Equal(t, "Until()", c.Until(start.Add(3*time.Hour)), 2*time.Hour)
	})

	t.Run("should delegate the other methods to the time package", func(t *testing.T) {
		c := testutil.NewClock(start)
		d, err := c.ParseDuration("1h")
		testutil.NoError(t, err)
		testutil.Equal(t, "ParseDuration()", d, time.Hour)
		testutil.Equal(t, "Date()", c.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), start)
		testutil.Equal(t, "Unix()", c.Unix(5, 0), time.Unix(5, 0))
		testutil.Equal(t, "UnixMilli()", c.UnixMilli(5), time.UnixMilli(5))
		testutil.Equal(t, "UnixMicro()", c.UnixMicro(5), time.UnixMicro(5))
		testutil.Equal(t, "FixedZone()", c.FixedZone("X", 60).String(), "X")
		loc, err := c.LoadLocation("UTC")
		testutil.NoError(t, err)
		testutil.Equal(t, "LoadLocation()", loc, time.UTC)
		p, err := c.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
		testutil.NoError(t, err)
		testutil.Equal(t, "Parse()", p, start)
		p, err = c.ParseInLocation(time.DateTime, "2026-01-01 00:00:00", time.UTC)
		testutil.NoError(t, err)
		testutil.Equal(t, "ParseInLocation()", p, start)
	})
}

func Test_Clock_Timer(t *testing.T) {
	t.Run("should fire at its deadline, with the deadline", func(t *testing.T) {
		c := testutil.NewClock(start)
		timer := c.NewTimer(time.Minute)
		testutil.Equal(t, "Timers()", c.Timers(), 1)

		c.Advance(time.Minute - time.Nanosecond)
		if _, ok := pending(timer.C()); ok {
			t.Fatal("the timer fired before its deadline")
		}
		c.Advance(time.Hour)
		v, ok := pending(timer.C())
		testutil.Equal(t, "fired", ok, true)
		testutil.Equal(t, "value", v, start.Add(time.Minute))
		testutil.Equal(t, "Timers()", c.Timers(), 0)
	})

	t.Run("should fire at once for a non-positive duration", func(t *testing.T) {
		c := testutil.NewClock(start)
		for _, d := range []time.Duration{0, -time.Second} {
			v, ok := pending(c.NewTimer(d).C())
			testutil.Equal(t, "fired", ok, true)
			testutil.Equal(t, "value", v, start.Add(d))
		}
	})

	t.Run("should report whether Stop stopped it, dropping a value not received", func(t *testing.T) {
		c := testutil.NewClock(start)
		timer := c.NewTimer(time.Minute)
		testutil.Equal(t, "Stop() before firing", timer.Stop(), true)
		testutil.Equal(t, "Stop() again", timer.Stop(), false)
		c.Advance(time.Hour)
		if _, ok := pending(timer.C()); ok {
			t.Fatal("a stopped timer fired")
		}

		timer = c.NewTimer(time.Minute)
		c.Advance(time.Minute)
		testutil.Equal(t, "Stop() of a value not received", timer.Stop(), true)
		if _, ok := pending(timer.C()); ok {
			t.Fatal("Stop() kept the stale value")
		}
	})

	t.Run("should reschedule with Reset", func(t *testing.T) {
		c := testutil.NewClock(start)
		timer := c.NewTimer(time.Minute)
		testutil.Equal(t, "Reset() of an active timer", timer.Reset(time.Hour), true)
		c.Advance(time.Minute)
		if _, ok := pending(timer.C()); ok {
			t.Fatal("the timer fired at its old deadline")
		}
		c.Advance(time.Hour)
		_, ok := pending(timer.C())
		testutil.Equal(t, "fired", ok, true)
		testutil.Equal(t, "Reset() of a fired timer", timer.Reset(time.Minute), false)
		c.Advance(time.Minute)
		v, _ := pending(timer.C())
		testutil.Equal(t, "value after Reset()", v, start.Add(time.Hour+2*time.Minute))
	})

	t.Run("should fire each timer at its own deadline in one Advance", func(t *testing.T) {
		c := testutil.NewClock(start)
		durations := []time.Duration{3 * time.Second, time.Second, 2 * time.Second}
		var channels []<-chan time.Time
		for _, d := range durations {
			channels = append(channels, c.NewTimer(d).C())
		}
		c.Advance(time.Minute)
		for i, d := range durations {
			v, _ := pending(channels[i])
			testutil.Equal(t, d.String(), v, start.Add(d))
		}
		testutil.Equal(t, "Now()", c.Now(), start.Add(time.Minute))
	})

	t.Run("should run the function of AfterFunc and have no channel", func(t *testing.T) {
		c := testutil.NewClock(start)
		ran := make(chan struct{})
		timer := c.AfterFunc(time.Minute, func() { close(ran) })
		if timer.C() != nil {
			t.Fatal("C() is not nil")
		}
		c.Advance(time.Minute)
		<-ran
		testutil.Equal(t, "Stop() after running", timer.Stop(), false)
	})

	t.Run("should send once with After and block Sleep until Advance", func(t *testing.T) {
		c := testutil.NewClock(start)
		ch := c.After(time.Minute)
		c.Advance(time.Minute)
		v, _ := pending(ch)
		testutil.Equal(t, "After()", v, start.Add(time.Minute))

		slept := make(chan struct{})
		go func() { c.Sleep(time.Minute); close(slept) }()
		for c.Timers() == 0 {
			time.Sleep(time.Millisecond)
		}
		select {
		case <-slept:
			t.Fatal("Sleep() returned before Advance()")
		default:
		}
		c.Advance(time.Minute)
		<-slept
		c.Sleep(0) // returns at once
	})
}

func Test_Clock_Ticker(t *testing.T) {
	t.Run("should tick each period, dropping the ticks not received", func(t *testing.T) {
		c := testutil.NewClock(start)
		ticker := c.NewTicker(time.Minute)
		c.Advance(time.Minute)
		v, _ := pending(ticker.C())
		testutil.Equal(t, "first tick", v, start.Add(time.Minute))

		c.Advance(3 * time.Minute)
		v, _ = pending(ticker.C())
		testutil.Equal(t, "kept tick", v, start.Add(2*time.Minute))
		if _, ok := pending(ticker.C()); ok {
			t.Fatal("a tick not received was queued")
		}
		testutil.Equal(t, "Timers()", c.Timers(), 1)
	})

	t.Run("should stop and reset", func(t *testing.T) {
		c := testutil.NewClock(start)
		ticker := c.NewTicker(time.Minute)
		c.Advance(time.Minute)
		ticker.Stop()
		if _, ok := pending(ticker.C()); ok {
			t.Fatal("Stop() kept the stale tick")
		}
		c.Advance(time.Hour)
		if _, ok := pending(ticker.C()); ok {
			t.Fatal("a stopped ticker ticked")
		}

		ticker.Reset(time.Second)
		c.Advance(time.Second)
		v, _ := pending(ticker.C())
		testutil.Equal(t, "tick after Reset()", v, start.Add(time.Hour+time.Minute+time.Second))
	})

	t.Run("should panic for a non-positive period", func(t *testing.T) {
		c := testutil.NewClock(start)
		testutil.Panics(t, "NewTicker(0)", func() { c.NewTicker(0) })
		ticker := c.NewTicker(time.Minute)
		testutil.Panics(t, "Reset(-1)", func() { ticker.Reset(-1) })
		if c.Tick(0) != nil {
			t.Fatal("Tick(0) is not nil")
		}
		ch := c.Tick(time.Minute)
		c.Advance(time.Minute)
		_, ok := pending(ch)
		testutil.Equal(t, "Tick() ticked", ok, true)
	})
}
