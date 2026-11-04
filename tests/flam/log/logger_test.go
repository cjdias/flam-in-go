package log

import (
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/log"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Logger(t *testing.T) {
	t.Run("should provide a single logger", func(t *testing.T) {
		a := newLogApp(t, container.Bag{}, nil)
		if a.logger(t) != a.logger(t) {
			t.Fatal("the container did not provide a single ILogger")
		}
	})

	t.Run("should keep the entries until flushed", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		l.SignalInfo("ch", "m")
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{})
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "written again", a.streams.get("a").messages(), []string{"m"})
	})

	t.Run("should signal and broadcast at each level", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		l.Signal(flam.LogNone, "ch", "signal")
		l.SignalFatal("ch", "signal fatal")
		l.SignalError("ch", "signal error")
		l.SignalWarning("ch", "signal warning")
		l.SignalNotice("ch", "signal notice")
		l.SignalInfo("ch", "signal info")
		l.SignalDebug("ch", "signal debug")
		l.Broadcast(flam.LogNone, "broadcast")
		l.BroadcastFatal("broadcast fatal")
		l.BroadcastError("broadcast error")
		l.BroadcastWarning("broadcast warning")
		l.BroadcastNotice("broadcast notice")
		l.BroadcastInfo("broadcast info")
		l.BroadcastDebug("broadcast debug")
		testutil.NoError(t, l.Flush())

		levels := []flam.LogLevel{
			flam.LogNone, flam.LogFatal, flam.LogError, flam.LogWarning, flam.LogNotice, flam.LogInfo, flam.LogDebug,
		}
		want := []written{}
		for _, broadcast := range []bool{false, true} {
			for _, level := range levels {
				w := written{timestamp: now, level: level, broadcast: broadcast, channel: "ch", message: "signal"}
				if broadcast {
					w.channel, w.message = "", "broadcast"
				}
				if level != flam.LogNone {
					w.message += " " + level.String()
				}
				want = append(want, w)
			}
		}
		testutil.Equal(t, "written", a.streams.get("a").written(), want)
	})

	t.Run("should stamp the entries with the time of the clock", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		l.BroadcastInfo("first")
		a.clock.Advance(time.Minute)
		l.BroadcastInfo("second")
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "first", got[0].timestamp, now)
		testutil.Equal(t, "second", got[1].timestamp, now.Add(time.Minute))
	})

	t.Run("should keep a copy of the contexts merged, skipping the nil ones", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		first, second := container.Bag{"a": 1}, container.Bag{"b": 2}
		var nilBag container.Bag
		l.SignalInfo("ch", "m", first, nil, nilBag, second)
		l.BroadcastInfo("m")
		first["a"] = 10
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "ctx", got[0].ctx, []flam.IBag{container.Bag{"a": 1, "b": 2}})
		testutil.Equal(t, "no ctx", len(got[1].ctx), 0)
	})

	t.Run("should merge the contexts, the later ones winning, deep copied", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		nested := container.Bag{"n": 1}
		l.SignalInfo("ch", "m", container.Bag{"a": 1, "b": 1, "x": nested}, container.Bag{"b": 2})
		nested["n"] = 2
		testutil.NoError(t, l.Flush())
		got := a.streams.get("a").written()
		testutil.Equal(t, "ctx", got[0].ctx, []flam.IBag{container.Bag{"a": 1, "b": 2, "x": container.Bag{"n": 1}}})
	})

	t.Run("should write to every stored stream, in id order", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"b": nil, "a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		l.BroadcastInfo("m1")
		l.BroadcastInfo("m2")
		testutil.NoError(t, l.Flush())
		for _, id := range []string{"a", "b"} {
			testutil.Equal(t, id, a.streams.get(id).messages(), []string{"m1", "m2"})
		}
	})

	t.Run("should write to the streams stored after the entries were given", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		l := a.logger(t)
		l.BroadcastInfo("m")
		testutil.NoError(t, l.Flush())
		_, err := a.factory(t).Get("a")
		testutil.NoError(t, err)
		testutil.Equal(t, "dropped", a.streams.get("a").messages(), []string{})

		l.BroadcastInfo("m")
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{"m"})
	})

	t.Run("should write to every stream, joining an error per failed stream, and drop the entries", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": {"write"}, "b": nil, "c": {"write"}}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		l.BroadcastInfo("m1")
		l.SignalInfo("ch", "m2")
		errs := joined(t, l.Flush())
		testutil.Equal(t, "errors", len(errs), 2)
		for i, id := range []string{"a", "c"} {
			testutil.Error(t, errs[i], errBoom, "unable to flush log entries to stream: boom",
				map[string]any{log.ErrorFieldID: id})
		}
		// a failing stream is not given the next entries
		testutil.Equal(t, "a", a.streams.get("a").messages(), []string{"m1"})
		testutil.Equal(t, "b", a.streams.get("b").messages(), []string{"m1", "m2"})
		testutil.Equal(t, "c", a.streams.get("c").messages(), []string{"m1"})
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "a", a.streams.get("a").messages(), []string{"m1"})
	})

	t.Run("should not write to a stream removed since listed, nor create it again", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil, "b": nil}), nil, func(app *testutil.App) {
			testutil.NoError(t, app.Decorate(func(f flam.ILogStreamFactory) flam.ILogStreamFactory {
				return &lookupFailing{ILogStreamFactory: f, missing: map[string]bool{}}
			}))
		})
		testutil.NoError(t, a.boot())
		a.factory(t).(*lookupFailing).missing["a"] = true
		l := a.logger(t)
		l.BroadcastInfo("m")
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "a", a.streams.get("a").messages(), []string{})
		testutil.Equal(t, "b", a.streams.get("b").messages(), []string{"m"})
	})

	t.Run("should not create again a stream removed", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		removed := a.streams.get("a")
		testutil.NoError(t, a.factory(t).Remove("a"))
		l := a.logger(t)
		l.BroadcastInfo("m")
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "stored", a.factory(t).Stored(), []string{})
		testutil.Equal(t, "a", removed.messages(), []string{})
		testutil.Equal(t, "closed", removed.isClosed(), true)
	})

	t.Run("should keep the order of the entries of concurrent calls", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 50 {
					l.BroadcastInfo("m")
					if err := l.Flush(); err != nil {
						t.Error(err)
					}
				}
			})
		}
		wg.Wait()
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "written", len(a.streams.get("a").written()), 400)
	})

	t.Run("should drop the entries once the stream factory is closed", func(t *testing.T) {
		a := newLogApp(t, defaults(t, nil, map[string][]string{"a": nil}), nil)
		testutil.NoError(t, a.boot())
		l := a.logger(t)
		testutil.NoError(t, a.factory(t).Close())
		l.BroadcastInfo("m")
		testutil.NoError(t, l.Flush())
		testutil.Equal(t, "written", a.streams.get("a").messages(), []string{})
	})
}
