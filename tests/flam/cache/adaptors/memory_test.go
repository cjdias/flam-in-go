package adaptors

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// memoryNow is the time of the clocks of the memory tests.
var memoryNow = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

// newMemoryApp returns a testutil.App with the config, cache and memory
// driver bundles, the clock, unless nil, and the values.
func newMemoryApp(t *testing.T, values map[string]any, clock flam.IClock) *testutil.App {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), cache.NewBundle(), json.NewBundle(), field.NewBundle(), memory.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	app := testutil.NewApp()
	if clock != nil {
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() { _ = cache.NewBundle().(flam.IClosableBundle).Close(app) })
	return app
}

// memoryAdaptor returns the adaptor "main" of the fields, of the memory
// driver unless given, created by the factory of an app with the clock,
// unless nil, and the values.
func memoryAdaptor(t *testing.T, fields container.Bag, values map[string]any, clock flam.IClock) (flam.ICacheAdaptor, error) {
	t.Helper()
	if _, ok := fields[factory.ConfigFieldDriver]; !ok {
		fields[factory.ConfigFieldDriver] = memory.Driver
	}
	all := map[string]any{cache.AdaptorConfigPath + ".main": fields}
	for path, value := range values {
		all[path] = value
	}
	app := newMemoryApp(t, all, clock)
	var a flam.ICacheAdaptor
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.ICacheAdaptorFactory) { a, err = f.Get("main") }))
	return a, err
}

// memoryParts returns the key generator and serializer of an app.
func memoryParts(t *testing.T) (flam.ICacheKeyGenerator, flam.ICacheSerializer) {
	t.Helper()
	app := newMemoryApp(t, nil, nil)
	var g flam.ICacheKeyGenerator
	var s flam.ICacheSerializer
	testutil.NoError(t, app.Invoke(func(gf flam.ICacheKeyGeneratorFactory, sf flam.ICacheSerializerFactory) error {
		var err error
		if g, err = gf.Get(field.Id); err != nil {
			return err
		}
		s, err = sf.Get(json.Id)
		return err
	}))
	return g, s
}

// newMemoryAdaptor returns the adaptor "main" over the parts of an app,
// holding at most maxEntries values, sweeping at the period, and its
// clock.
func newMemoryAdaptor(t *testing.T, maxEntries int, sweepPeriod time.Duration) (flam.ICacheAdaptor, *testutil.Clock) {
	t.Helper()
	g, s := memoryParts(t)
	clock := testutil.NewClock(memoryNow)
	a, err := memory.NewAdaptor("main", g, s, clock, maxEntries, sweepPeriod)
	testutil.NoError(t, err)
	return a, clock
}

// has returns the result of Has for the users of the ids.
func has(t *testing.T, a flam.ICacheAdaptor, ids ...uint) []bool {
	t.Helper()
	objects := make([]any, len(ids))
	for i, id := range ids {
		objects[i] = &user{Id: id}
	}
	found, err := a.Has(context.Background(), objects...)
	testutil.NoError(t, err)
	return found
}

// pickySerializer fails to deserialize the values of the users named
// "bad".
type pickySerializer struct{ flam.ICacheSerializer }

func (s pickySerializer) Deserialize(data string, obj any) error {
	if strings.Contains(data, `"bad"`) {
		return errBoom
	}
	return s.ICacheSerializer.Deserialize(data, obj)
}

func Test_MemoryBundle(t *testing.T) {
	t.Run("should declare the driver, fields, paths and defaults", func(t *testing.T) {
		testutil.Equal(t, "Driver", memory.Driver, "flam.cache.adaptors.driver.memory")
		testutil.Equal(t, "ConfigFieldMaxEntries", memory.ConfigFieldMaxEntries, "max_entries")
		testutil.Equal(t, "ConfigFieldSweepPeriod", memory.ConfigFieldSweepPeriod, "sweep_period")
		testutil.Equal(t, "DefaultMaxEntriesConfigPath", memory.DefaultMaxEntriesConfigPath, "flam.cache.defaults.adaptors.memory.max_entries")
		testutil.Equal(t, "DefaultSweepPeriodConfigPath", memory.DefaultSweepPeriodConfigPath, "flam.cache.defaults.adaptors.memory.sweep_period")
		testutil.Equal(t, "DefaultMaxEntries", memory.DefaultMaxEntries, 10000)
		testutil.Equal(t, "DefaultSweepPeriod", memory.DefaultSweepPeriod, time.Minute)
		testutil.Equal(t, "ErrorFieldID", memory.ErrorFieldID, "id")
		testutil.Equal(t, "ErrorFieldField", memory.ErrorFieldField, "field")
		testutil.Equal(t, "ErrorFieldSerializer", memory.ErrorFieldSerializer, "serializer")
		testutil.Equal(t, "ErrorFieldKeyGenerator", memory.ErrorFieldKeyGenerator, "key_generator")
	})

	t.Run("should provide the creator", func(t *testing.T) {
		b := memory.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.cache.adaptors.memory")
		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.cache.adaptors.memory", "provider/1"})
	})

	t.Run("should fail to register when the app fails", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(string) error { return errBoom }
		if err := memory.NewBundle().Register(app); !errors.Is(err, errBoom) {
			t.Fatalf("Register() = %v, want boom", err)
		}
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := memoryAdaptor(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, testutil.NewClock(memoryNow))
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}

func Test_MemoryCreator(t *testing.T) {
	t.Run("should create the adaptor with the built-in defaults", func(t *testing.T) {
		a, err := memoryAdaptor(t, container.Bag{}, nil, testutil.NewClock(memoryNow))
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", a.Id(), "main")
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}))
		testutil.Equal(t, "Has()", has(t, a, 1), []bool{true})
	})

	t.Run("should take the fields of the configuration", func(t *testing.T) {
		for _, maxEntries := range []any{1, "1", " 1 ", 1.0, uint8(1), int64(1)} {
			a, err := memoryAdaptor(t, container.Bag{
				cache.ConfigFieldSerializer:   json.Id,
				cache.ConfigFieldKeyGenerator: field.Id,
				memory.ConfigFieldMaxEntries:  maxEntries,
				memory.ConfigFieldSweepPeriod: "0s",
			}, nil, testutil.NewClock(memoryNow))
			testutil.NoError(t, err)
			testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}))
			testutil.Equal(t, "Has()", has(t, a, 1, 2), []bool{false, true})
		}
	})

	t.Run("should take the defaults of the configuration", func(t *testing.T) {
		a, err := memoryAdaptor(t, container.Bag{cache.ConfigFieldSerializer: nil}, map[string]any{
			memory.DefaultMaxEntriesConfigPath:  2,
			memory.DefaultSweepPeriodConfigPath: time.Second,
		}, testutil.NewClock(memoryNow))
		testutil.NoError(t, err)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}, &user{Id: 3}))
		testutil.Equal(t, "Has()", has(t, a, 1, 2, 3), []bool{false, true, true})
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for _, c := range []struct {
			fields container.Bag
			field  string
		}{
			{container.Bag{cache.ConfigFieldSerializer: ""}, cache.ConfigFieldSerializer},
			{container.Bag{cache.ConfigFieldKeyGenerator: 3}, cache.ConfigFieldKeyGenerator},
			{container.Bag{memory.ConfigFieldMaxEntries: 0}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: -1}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: 1.5}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: 1e12}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: "x"}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: "0"}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldMaxEntries: true}, memory.ConfigFieldMaxEntries},
			{container.Bag{memory.ConfigFieldSweepPeriod: "-1s"}, memory.ConfigFieldSweepPeriod},
			{container.Bag{memory.ConfigFieldSweepPeriod: 5}, memory.ConfigFieldSweepPeriod},
		} {
			_, err := memoryAdaptor(t, c.fields, nil, testutil.NewClock(memoryNow))
			testutil.Error(t, errors.Unwrap(err), memory.ErrInvalidField,
				"unable to create memory cache adaptor: invalid memory cache adaptor field",
				map[string]any{memory.ErrorFieldID: "main", memory.ErrorFieldField: c.field})
		}
	})

	t.Run("should fail with an unknown serializer or key generator", func(t *testing.T) {
		for _, c := range []struct {
			fields   container.Bag
			sentinel error
			field    string
		}{
			{container.Bag{cache.ConfigFieldSerializer: "nope"}, memory.ErrSerializer, memory.ErrorFieldSerializer},
			{container.Bag{cache.ConfigFieldKeyGenerator: "nope"}, memory.ErrKeyGenerator, memory.ErrorFieldKeyGenerator},
		} {
			_, err := memoryAdaptor(t, c.fields, nil, testutil.NewClock(memoryNow))
			if !errors.Is(err, c.sentinel) || !errors.Is(err, factory.ErrUnknownID) {
				t.Fatalf("Get() = %v, want %v and factory.ErrUnknownID", err, c.sentinel)
			}
			var e *flam.Error
			errors.As(errors.Unwrap(err), &e)
			testutil.Context(t, e, map[string]any{memory.ErrorFieldID: "main", c.field: "nope"})
		}
	})

	t.Run("should fail without the clock", func(t *testing.T) {
		_, err := memoryAdaptor(t, container.Bag{}, nil, nil)
		testutil.Error(t, errors.Unwrap(err), memory.ErrClock,
			"unable to create memory cache adaptor: unable to get memory cache adaptor clock",
			map[string]any{memory.ErrorFieldID: "main"})
	})
}

func Test_MemoryNewAdaptor(t *testing.T) {
	g, s := memoryParts(t)
	clock := testutil.NewClock(memoryNow)
	for _, tc := range []struct {
		g          flam.ICacheKeyGenerator
		s          flam.ICacheSerializer
		c          flam.IClock
		maxEntries int
		sweep      time.Duration
		sentinel   error
		text       string
		ctx        map[string]any
	}{
		{nil, s, clock, 1, 0, memory.ErrNilKeyGenerator, "nil memory cache adaptor key generator", map[string]any{memory.ErrorFieldID: "main"}},
		{g, nil, clock, 1, 0, memory.ErrNilSerializer, "nil memory cache adaptor serializer", map[string]any{memory.ErrorFieldID: "main"}},
		{g, s, nil, 1, 0, memory.ErrNilClock, "nil memory cache adaptor clock", map[string]any{memory.ErrorFieldID: "main"}},
		{g, s, (*testutil.Clock)(nil), 1, 0, memory.ErrNilClock, "nil memory cache adaptor clock", map[string]any{memory.ErrorFieldID: "main"}},
		{g, s, clock, 0, 0, memory.ErrInvalidField, "invalid memory cache adaptor field",
			map[string]any{memory.ErrorFieldID: "main", memory.ErrorFieldField: memory.ConfigFieldMaxEntries}},
		{g, s, clock, 1, -time.Second, memory.ErrInvalidField, "invalid memory cache adaptor field",
			map[string]any{memory.ErrorFieldID: "main", memory.ErrorFieldField: memory.ConfigFieldSweepPeriod}},
	} {
		a, err := memory.NewAdaptor("main", tc.g, tc.s, tc.c, tc.maxEntries, tc.sweep)
		testutil.Error(t, err, tc.sentinel, "unable to create memory cache adaptor: "+tc.text, tc.ctx)
		testutil.Equal(t, "adaptor", a, nil)
	}
}

func Test_MemoryAdaptor(t *testing.T) {
	t.Run("should set, check, get and delete the objects", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		testutil.Equal(t, "Id()", a.Id(), "main")
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}, &user{Id: 2, Name: "bob"}))
		testutil.Equal(t, "Has()", has(t, a, 1, 3, 1), []bool{true, false, true})

		first, missing := &user{Id: 1}, &user{Id: 3, Name: "kept"}
		found, err := a.Get(context.Background(), first, missing)
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", found, []any{first, nil})
		testutil.Equal(t, "filled", first, &user{Id: 1, Name: "ann"})
		testutil.Equal(t, "unchanged", missing, &user{Id: 3, Name: "kept"})

		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}, &user{Id: 3}))
		testutil.Equal(t, "Has() after Del", has(t, a, 1, 2), []bool{false, true})
	})

	t.Run("should replace a value set again", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}))
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "bob"}))
		got := &user{Id: 1}
		_, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "Name", got.Name, "bob")
	})

	t.Run("should hold copies of the objects", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		set := &user{Id: 1, Tags: map[string]bool{"a": true}}
		testutil.NoError(t, a.Set(context.Background(), set))
		set.Tags["b"] = true

		got := &user{Id: 1}
		_, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "got", got, &user{Id: 1, Tags: map[string]bool{"a": true}})
		got.Tags["c"] = true

		again := &user{Id: 1}
		_, err = a.Get(context.Background(), again)
		testutil.NoError(t, err)
		testutil.Equal(t, "again", again, &user{Id: 1, Tags: map[string]bool{"a": true}})
	})

	t.Run("should replace the objects, instead of merging in them", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Tags: map[string]bool{"new": true}}))
		got := &user{Id: 1, Name: "stale", Tags: map[string]bool{"stale": true}}
		_, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "object", got, &user{Id: 1, Tags: map[string]bool{"new": true}})
	})

	t.Run("should change no object when one fails to decode", func(t *testing.T) {
		g, s := memoryParts(t)
		a, err := memory.NewAdaptor("main", g, pickySerializer{s}, testutil.NewClock(memoryNow), 10, 0)
		testutil.NoError(t, err)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}, &user{Id: 2, Name: "bad"}))

		first, second := &user{Id: 1}, &user{Id: 2, Name: "kept"}
		_, err = a.Get(context.Background(), first, second)
		testutil.Error(t, err, errBoom, "unable to get from memory cache: boom",
			map[string]any{memory.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:2"})
		testutil.Equal(t, "first", first, &user{Id: 1})
		testutil.Equal(t, "second", second, &user{Id: 2, Name: "kept"})
	})

	t.Run("should fail for an object that is not a pointer", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.Set(context.Background(), user{Id: 1}))
		_, err := a.Get(context.Background(), user{Id: 1})
		if !errors.Is(err, cache.ErrDeserialize) {
			t.Fatalf("Get() = %v, want cache.ErrDeserialize", err)
		}
	})

	t.Run("should expire the ephemeral objects", func(t *testing.T) {
		a, clock := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 1, Name: "ann"}, &user{Id: 2}))
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 3}))
		clock.Advance(time.Minute - time.Nanosecond)
		testutil.Equal(t, "Has() before expiry", has(t, a, 1, 2, 3), []bool{true, true, true})

		clock.Advance(time.Nanosecond)
		got := &user{Id: 1}
		found, err := a.Get(context.Background(), got)
		testutil.NoError(t, err)
		testutil.Equal(t, "Get() after expiry", found, []any{nil})
		testutil.Equal(t, "Has() after expiry", has(t, a, 1, 2, 3), []bool{false, false, true})
	})

	t.Run("should make an ephemeral object permanent when set again", func(t *testing.T) {
		a, clock := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Minute, &user{Id: 1}))
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		clock.Advance(time.Hour)
		testutil.Equal(t, "Has()", has(t, a, 1), []bool{true})
	})

	t.Run("should evict the least recently set or got values", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 2, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}))
		_, err := a.Get(context.Background(), &user{Id: 1})
		testutil.NoError(t, err)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 3}))
		testutil.Equal(t, "Has() after Get", has(t, a, 1, 2, 3), []bool{true, false, true})

		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Hour, &user{Id: 4}))
		testutil.Equal(t, "Has() after Set", has(t, a, 1, 3, 4), []bool{true, false, true})
	})

	t.Run("should not make the checked values more recently used", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 2, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}))
		testutil.Equal(t, "Has()", has(t, a, 1), []bool{true})
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 3}))
		testutil.Equal(t, "Has() after Set", has(t, a, 1, 2, 3), []bool{false, true, true})
	})

	t.Run("should keep the last values of a set beyond the maximum", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 2, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}, &user{Id: 2}, &user{Id: 3}))
		testutil.Equal(t, "Has()", has(t, a, 1, 2, 3), []bool{false, true, true})
	})

	t.Run("should sweep the expired values on the writes once per period", func(t *testing.T) {
		// 1 expired and most recently used, 2 least recently used: a sweep
		// removing 1 leaves room for 3, else 2 is evicted
		scenario := func(sweepPeriod time.Duration, advance time.Duration) []bool {
			a, clock := newMemoryAdaptor(t, 2, sweepPeriod)
			testutil.NoError(t, a.Set(context.Background(), &user{Id: 2}))
			testutil.NoError(t, a.SetEphemeral(context.Background(), time.Second, &user{Id: 1}))
			clock.Advance(advance)
			testutil.NoError(t, a.Set(context.Background(), &user{Id: 3}))
			return has(t, a, 2, 3)
		}
		testutil.Equal(t, "swept", scenario(time.Minute, time.Minute), []bool{true, true})
		testutil.Equal(t, "not due", scenario(time.Minute, time.Minute-time.Nanosecond), []bool{false, true})
		testutil.Equal(t, "disabled", scenario(0, time.Minute), []bool{false, true})
	})

	t.Run("should sweep again a period after the last sweep", func(t *testing.T) {
		a, clock := newMemoryAdaptor(t, 2, time.Minute)
		clock.Advance(time.Minute)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 9})) // sweeps
		testutil.NoError(t, a.Del(context.Background(), &user{Id: 9}))

		testutil.NoError(t, a.Set(context.Background(), &user{Id: 2}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Second, &user{Id: 1}))
		clock.Advance(time.Minute - time.Nanosecond)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 3})) // not due: evicts 2
		testutil.Equal(t, "Has() before the period", has(t, a, 2, 3), []bool{false, true})

		testutil.NoError(t, a.Del(context.Background(), &user{Id: 1}, &user{Id: 3}))
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 2}))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Nanosecond, &user{Id: 3}))
		clock.Advance(time.Nanosecond)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 4})) // due: sweeps 3
		testutil.Equal(t, "Has() after the period", has(t, a, 2, 3, 4), []bool{true, false, true})
	})

	t.Run("should do nothing for no objects", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		found, err := a.Has(context.Background())
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", found, []bool{})
		got, err := a.Get(context.Background())
		testutil.NoError(t, err)
		testutil.Equal(t, "Get()", got, []any{})
		testutil.NoError(t, a.Set(context.Background()))
		testutil.NoError(t, a.SetEphemeral(context.Background(), time.Second))
		testutil.NoError(t, a.Del(context.Background()))
	})

	t.Run("should replace a nil context with context.Background", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		var ctx context.Context
		testutil.NoError(t, a.Set(ctx, &user{Id: 1, Name: "ann"}))
		testutil.NoError(t, a.SetEphemeral(ctx, time.Minute, &user{Id: 2}))
		found, err := a.Has(ctx, &user{Id: 1}, &user{Id: 2})
		testutil.NoError(t, err)
		testutil.Equal(t, "Has()", found, []bool{true, true})
		got := &user{Id: 1}
		_, err = a.Get(ctx, got)
		testutil.NoError(t, err)
		testutil.Equal(t, "Name", got.Name, "ann")
		testutil.NoError(t, a.Del(ctx, &user{Id: 1}))
	})

	t.Run("should fail with a cancelled context, changing nothing", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1, Name: "ann"}))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got := &user{Id: 1}
		_, hasErr := a.Has(ctx, &user{Id: 1})
		_, getErr := a.Get(ctx, got)
		for message, err := range map[string]error{
			"unable to check memory cache":            hasErr,
			"unable to get from memory cache":         getErr,
			"unable to set in memory cache":           a.Set(ctx, &user{Id: 2}),
			"unable to set ephemeral in memory cache": a.SetEphemeral(ctx, time.Second, &user{Id: 2}),
			"unable to delete from memory cache":      a.Del(ctx, &user{Id: 1}),
		} {
			testutil.Error(t, err, cache.ErrStorage, message+": cache storage failure: context canceled",
				map[string]any{memory.ErrorFieldID: "main"})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("%s = %v, want context.Canceled", message, err)
			}
		}
		testutil.Equal(t, "object", got, &user{Id: 1})
		testutil.Equal(t, "Has()", has(t, a, 1, 2), []bool{true, false})
	})

	t.Run("should refuse a ttl that is not positive", func(t *testing.T) {
		a, _ := newMemoryAdaptor(t, 10, 0)
		for _, ttl := range []time.Duration{0, -time.Second} {
			testutil.Error(t, a.SetEphemeral(context.Background(), ttl, &user{Id: 1}), memory.ErrInvalidTTL,
				"unable to set ephemeral in memory cache: invalid memory cache adaptor ttl",
				map[string]any{memory.ErrorFieldID: "main"})
		}
	})

	t.Run("should fail with the errors of the key generator", func(t *testing.T) {
		g, s := memoryParts(t)
		a, err := memory.NewAdaptor("main", failingGenerator{g}, s, testutil.NewClock(memoryNow), 10, 0)
		testutil.NoError(t, err)
		bad := &user{Name: "bad"}
		_, hasErr := a.Has(context.Background(), bad)
		_, getErr := a.Get(context.Background(), bad)
		for message, err := range map[string]error{
			"unable to check memory cache":            hasErr,
			"unable to get from memory cache":         getErr,
			"unable to set in memory cache":           a.Set(context.Background(), &user{Id: 1}, bad),
			"unable to set ephemeral in memory cache": a.SetEphemeral(context.Background(), time.Second, bad),
			"unable to delete from memory cache":      a.Del(context.Background(), bad),
		} {
			testutil.Error(t, err, errBoom, message+": boom", map[string]any{memory.ErrorFieldID: "main"})
		}
		testutil.Equal(t, "Has()", has(t, a, 1), []bool{false})
	})

	t.Run("should fail with the errors of the serializer, with the key", func(t *testing.T) {
		g, s := memoryParts(t)
		a, err := memory.NewAdaptor("main", g, failingSerializer{s}, testutil.NewClock(memoryNow), 10, 0)
		testutil.NoError(t, err)
		bad := &user{Id: 9, Name: "bad"}
		testutil.Error(t, a.Set(context.Background(), bad), errBoom, "unable to set in memory cache: boom",
			map[string]any{memory.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:9"})
		testutil.Error(t, a.SetEphemeral(context.Background(), time.Second, bad), errBoom, "unable to set ephemeral in memory cache: boom",
			map[string]any{memory.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:9"})

		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		_, err = a.Get(context.Background(), &user{Id: 1})
		testutil.Error(t, err, errBoom, "unable to get from memory cache: boom",
			map[string]any{memory.ErrorFieldID: "main", cache.ErrorFieldKey: "adaptors.user:1"})
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		a, clock := newMemoryAdaptor(t, 8, time.Millisecond)
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range 50 {
					id := uint(i*50 + j%10)
					_ = a.Set(context.Background(), &user{Id: id, Name: "n"})
					_ = a.SetEphemeral(context.Background(), time.Millisecond, &user{Id: id + 1})
					_, _ = a.Get(context.Background(), &user{Id: id}, &user{Id: id + 1})
					_, _ = a.Has(context.Background(), &user{Id: id})
					_ = a.Del(context.Background(), &user{Id: id + 1})
					clock.Advance(time.Millisecond)
				}
			}()
		}
		wg.Wait()
		testutil.NoError(t, a.Set(context.Background(), &user{Id: 1}))
		testutil.Equal(t, "Has()", has(t, a, 1), []bool{true})
	})
}
