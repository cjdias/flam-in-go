package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/adaptors/memory"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field"
	"github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// loaderNow is the time of the clocks of the loader tests.
var loaderNow = time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

// item is the object of the loader tests, keyed "cache.item:<Id>".
type item struct {
	Id   uint
	Name string
}

// loaderParts returns the field key generator and the json serializer of
// an app.
func loaderParts(t *testing.T) (flam.ICacheKeyGenerator, flam.ICacheSerializer) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), cache.NewBundle(), json.NewBundle(), field.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	a := testutil.NewApp()
	for _, b := range bundles {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() { _ = cache.NewBundle().(flam.IClosableBundle).Close(a) })
	var g flam.ICacheKeyGenerator
	var s flam.ICacheSerializer
	testutil.NoError(t, a.Invoke(func(gf flam.ICacheKeyGeneratorFactory, sf flam.ICacheSerializerFactory) error {
		var err error
		if g, err = gf.Get(field.Id); err != nil {
			return err
		}
		s, err = sf.Get(json.Id)
		return err
	}))
	return g, s
}

// newSpy returns a spy over a memory adaptor "main" of a fake clock, and
// the clock.
func newSpy(t *testing.T) (*spyAdaptor, *testutil.Clock) {
	t.Helper()
	g, s := loaderParts(t)
	clock := testutil.NewClock(loaderNow)
	a, err := memory.NewAdaptor("main", g, s, clock, 100, 0)
	testutil.NoError(t, err)
	return &spyAdaptor{ICacheKeyedAdaptor: a.(flam.ICacheKeyedAdaptor)}, clock
}

// spyAdaptor counts the Get calls of its adaptor, failing those its
// getErr tells, of their 1-based number, and every Set and SetEphemeral
// when setErr is set.
type spyAdaptor struct {
	flam.ICacheKeyedAdaptor

	gets   atomic.Int32
	getErr func(n int32) error
	setErr error
}

func (s *spyAdaptor) Get(ctx context.Context, objects ...any) ([]any, error) {
	n := s.gets.Add(1)
	if s.getErr != nil {
		if err := s.getErr(n); err != nil {
			return nil, err
		}
	}
	return s.ICacheKeyedAdaptor.Get(ctx, objects...)
}

func (s *spyAdaptor) Set(ctx context.Context, objects ...any) error {
	if s.setErr != nil {
		return s.setErr
	}
	return s.ICacheKeyedAdaptor.Set(ctx, objects...)
}

func (s *spyAdaptor) SetEphemeral(ctx context.Context, ttl time.Duration, objects ...any) error {
	if s.setErr != nil {
		return s.setErr
	}
	return s.ICacheKeyedAdaptor.SetEphemeral(ctx, ttl, objects...)
}

// unkeyed hides the keys of its adaptor.
type unkeyed struct{ flam.ICacheAdaptor }

// failingKeys is a key generator failing for the items named "bad".
type failingKeys struct{ flam.ICacheKeyGenerator }

func (g failingKeys) Generate(obj any) (string, error) {
	if i, ok := obj.(*item); ok && i.Name == "bad" {
		return "", errBoom
	}
	return g.ICacheKeyGenerator.Generate(obj)
}

// counter returns the load function naming the items "loaded <Id>", and
// its count of calls.
func counter() (cache.LoadFunc[item], *atomic.Int32) {
	calls := &atomic.Int32{}
	return func(_ context.Context, i *item) error {
		calls.Add(1)
		i.Name = "loaded"
		return nil
	}, calls
}

// has reports whether the adaptor holds the item of the id.
func has(t *testing.T, a flam.ICacheAdaptor, id uint) bool {
	t.Helper()
	found, err := a.Has(context.Background(), &item{Id: id})
	testutil.NoError(t, err)
	return found[0]
}

// newLoader returns the loader of the adaptor, failing the test on error.
func newLoader(t *testing.T, a flam.ICacheAdaptor, load cache.LoadFunc[item], opts ...cache.LoaderOption) *cache.Loader[item] {
	t.Helper()
	l, err := cache.NewLoader(a, load, opts...)
	testutil.NoError(t, err)
	return l
}

func Test_Loader_Declarations(t *testing.T) {
	testutil.Equal(t, "ErrorFieldOption", cache.ErrorFieldOption, "option")
	testutil.Equal(t, "ErrorFieldPanic", cache.ErrorFieldPanic, "panic")
	var _ flam.ICacheLoader[item] = (*cache.Loader[item])(nil)
}

func Test_NewLoader(t *testing.T) {
	spy, _ := newSpy(t)
	load, _ := counter()

	t.Run("should fail with a nil adaptor or load function", func(t *testing.T) {
		for _, a := range []flam.ICacheAdaptor{nil, (*spyAdaptor)(nil)} {
			l, err := cache.NewLoader(a, load)
			testutil.Error(t, err, cache.ErrNilAdaptor, "unable to create cache loader: nil cache loader adaptor", map[string]any{})
			testutil.Equal(t, "loader", l, (*cache.Loader[item])(nil))
		}
		_, err := cache.NewLoader[item](spy, nil)
		testutil.Error(t, err, cache.ErrNilLoad, "unable to create cache loader: nil cache loader load function",
			map[string]any{cache.ErrorFieldID: "main"})
	})

	t.Run("should fail with an adaptor telling no keys, without a key generator", func(t *testing.T) {
		_, err := cache.NewLoader(unkeyed{spy}, load)
		testutil.Error(t, err, cache.ErrNotKeyed, "unable to create cache loader: cache loader adaptor tells no keys",
			map[string]any{cache.ErrorFieldID: "main"})
	})

	t.Run("should fail with the first invalid option, skipping the nil ones", func(t *testing.T) {
		g, _ := loaderParts(t)
		for option, opts := range map[string][]cache.LoaderOption{
			"ttl":           {cache.WithTTL(0)},
			"jitter":        {nil, cache.WithJitter(-time.Second), cache.WithTTL(-time.Second)},
			"negative_ttl":  {cache.WithNegativeTTL(-time.Second)},
			"clock":         {cache.WithClock(nil), cache.WithKeyGenerator(nil)},
			"key_generator": {cache.WithKeyGenerator(nil), cache.WithClock((*testutil.Clock)(nil))},
		} {
			_, err := cache.NewLoader(spy, load, opts...)
			testutil.Error(t, err, cache.ErrInvalidOption, "unable to create cache loader: invalid cache loader option",
				map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldOption: option})
		}
		_, err := cache.NewLoader(spy, load, nil, cache.WithTTL(time.Second), cache.WithJitter(0),
			cache.WithNegativeTTL(time.Second), cache.WithClock(testutil.NewClock(loaderNow)),
			cache.WithKeyGenerator(g), cache.WithFailClosed(), cache.WithErrorHandler(nil))
		testutil.NoError(t, err)
	})
}

func Test_Loader(t *testing.T) {
	t.Run("should get the cached objects without loading them", func(t *testing.T) {
		spy, _ := newSpy(t)
		load, calls := counter()
		testutil.NoError(t, spy.Set(context.Background(), &item{Id: 1, Name: "cached"}))
		got := &item{Id: 1}
		testutil.NoError(t, newLoader(t, spy, load).Get(context.Background(), got))
		testutil.Equal(t, "item", got, &item{Id: 1, Name: "cached"})
		testutil.Equal(t, "loads", calls.Load(), int32(0))
	})

	t.Run("should load and store the missing objects, with no expiration", func(t *testing.T) {
		spy, clock := newSpy(t)
		load, calls := counter()
		l := newLoader(t, spy, load)
		var ctx context.Context // replaced with context.Background
		got := &item{Id: 1}
		testutil.NoError(t, l.Get(ctx, got))
		testutil.Equal(t, "item", got, &item{Id: 1, Name: "loaded"})
		clock.Advance(1000 * time.Hour)
		again := &item{Id: 1}
		testutil.NoError(t, l.Get(context.Background(), again))
		testutil.Equal(t, "again", again, &item{Id: 1, Name: "loaded"})
		testutil.Equal(t, "loads", calls.Load(), int32(1))
		testutil.Equal(t, "Get calls", spy.gets.Load(), int32(3)) // a miss gets twice
	})

	t.Run("should store the objects loaded with the ttl", func(t *testing.T) {
		spy, clock := newSpy(t)
		load, _ := counter()
		testutil.NoError(t, newLoader(t, spy, load, cache.WithTTL(time.Minute)).Get(context.Background(), &item{Id: 1}))
		clock.Advance(time.Minute - time.Nanosecond)
		testutil.Equal(t, "before the ttl", has(t, spy, 1), true)
		clock.Advance(time.Nanosecond)
		testutil.Equal(t, "after the ttl", has(t, spy, 1), false)
	})

	t.Run("should add the jitter to the ttl", func(t *testing.T) {
		spy, clock := newSpy(t)
		load, _ := counter()
		l := newLoader(t, spy, load, cache.WithTTL(time.Minute), cache.WithJitter(time.Second))
		for id := range uint(20) {
			testutil.NoError(t, l.Get(context.Background(), &item{Id: id}))
		}
		clock.Advance(time.Minute - time.Nanosecond)
		for id := range uint(20) {
			testutil.Equal(t, "before the ttl", has(t, spy, id), true)
		}
		clock.Advance(time.Second + time.Nanosecond)
		for id := range uint(20) {
			testutil.Equal(t, "after the ttl and jitter", has(t, spy, id), false)
		}
	})

	t.Run("should ignore the jitter without a ttl", func(t *testing.T) {
		spy, clock := newSpy(t)
		load, _ := counter()
		testutil.NoError(t, newLoader(t, spy, load, cache.WithJitter(time.Second)).Get(context.Background(), &item{Id: 1}))
		clock.Advance(time.Hour)
		testutil.Equal(t, "Has()", has(t, spy, 1), true)
	})

	t.Run("should get again the objects missed before loading them", func(t *testing.T) {
		spy, _ := newSpy(t)
		spy.getErr = func(n int32) error {
			if n == 2 { // stored by another caller after the first get missed
				return spy.ICacheKeyedAdaptor.Set(context.Background(), &item{Id: 1, Name: "raced"})
			}
			return nil
		}
		load, calls := counter()
		got := &item{Id: 1}
		testutil.NoError(t, newLoader(t, spy, load).Get(context.Background(), got))
		testutil.Equal(t, "item", got, &item{Id: 1, Name: "raced"})
		testutil.Equal(t, "loads", calls.Load(), int32(0))
	})

	t.Run("should return the error of the load function, storing nothing", func(t *testing.T) {
		spy, _ := newSpy(t)
		calls := 0
		l := newLoader(t, spy, func(_ context.Context, i *item) error {
			calls++
			i.Name = "partial"
			return errBoom
		})
		got := &item{Id: 1}
		for range 2 {
			if err := l.Get(context.Background(), got); err != errBoom {
				t.Fatalf("Get() = %v, want errBoom as is", err)
			}
		}
		testutil.Equal(t, "item", got, &item{Id: 1})
		testutil.Equal(t, "Has()", has(t, spy, 1), false)
		testutil.Equal(t, "loads", calls, 2)
	})

	t.Run("should load again the objects not found, without a negative ttl", func(t *testing.T) {
		spy, _ := newSpy(t)
		calls := 0
		l := newLoader(t, spy, func(context.Context, *item) error {
			calls++
			return cache.ErrNotFound
		})
		for range 2 {
			if err := l.Get(context.Background(), &item{Id: 1}); err != cache.ErrNotFound {
				t.Fatalf("Get() = %v, want ErrNotFound as is", err)
			}
		}
		testutil.Equal(t, "loads", calls, 2)
	})

	t.Run("should remember the objects not found for the negative ttl", func(t *testing.T) {
		spy, _ := newSpy(t)
		clock := testutil.NewClock(loaderNow)
		loads := map[uint]int{}
		l := newLoader(t, spy, func(_ context.Context, i *item) error {
			loads[i.Id]++
			return errors.Join(errBoom, cache.ErrNotFound)
		}, cache.WithNegativeTTL(time.Minute), cache.WithClock(clock))

		get := func(id uint) {
			t.Helper()
			if err := l.Get(context.Background(), &item{Id: id}); !errors.Is(err, cache.ErrNotFound) {
				t.Fatalf("Get(%d) = %v, want ErrNotFound", id, err)
			}
		}
		get(1)
		testutil.Error(t, l.Get(context.Background(), &item{Id: 1}), cache.ErrNotFound, "unable to get from cache loader: cache object not found",
			map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldKey: "cache.item:1"})
		get(2)
		clock.Advance(time.Minute - time.Nanosecond)
		get(1)
		get(2)
		testutil.Equal(t, "loads before the ttl", loads, map[uint]int{1: 1, 2: 1})

		clock.Advance(time.Nanosecond)
		get(1) // forgotten when found expired
		get(3) // sweeps 2
		get(1)
		get(3)
		testutil.Equal(t, "loads after the ttl", loads, map[uint]int{1: 2, 2: 1, 3: 1})
		get(2)
		testutil.Equal(t, "loads after the sweep", loads, map[uint]int{1: 2, 2: 2, 3: 1})
	})

	t.Run("should fail for a nil object", func(t *testing.T) {
		spy, _ := newSpy(t)
		load, calls := counter()
		testutil.Error(t, newLoader(t, spy, load).Get(context.Background(), nil), cache.ErrNilObject,
			"unable to get from cache loader: nil cache loader object", map[string]any{cache.ErrorFieldID: "main"})
		testutil.Equal(t, "loads", calls.Load(), int32(0))
	})

	t.Run("should fail when the key cannot be generated", func(t *testing.T) {
		g, s := loaderParts(t)
		a, err := memory.NewAdaptor("main", failingKeys{g}, s, testutil.NewClock(loaderNow), 10, 0)
		testutil.NoError(t, err)
		load, calls := counter()
		err = newLoader(t, a, load).Get(context.Background(), &item{Name: "bad"})
		testutil.Error(t, err, errBoom, "unable to get from cache loader: unable to get memory cache keys: boom",
			map[string]any{cache.ErrorFieldID: "main"})

		err = newLoader(t, unkeyed{a}, load, cache.WithKeyGenerator(failingKeys{g})).Get(context.Background(), &item{Name: "bad"})
		testutil.Error(t, err, errBoom, "unable to get from cache loader: boom", map[string]any{cache.ErrorFieldID: "main"})
		testutil.Equal(t, "loads", calls.Load(), int32(0))
	})

	t.Run("should take the keys from the key generator", func(t *testing.T) {
		spy, _ := newSpy(t)
		g, _ := loaderParts(t)
		load, calls := counter()
		l := newLoader(t, unkeyed{spy}, load, cache.WithKeyGenerator(g))
		testutil.NoError(t, l.Get(context.Background(), &item{Id: 1}))
		testutil.NoError(t, l.Get(context.Background(), &item{Id: 1}))
		testutil.Equal(t, "loads", calls.Load(), int32(1))
		testutil.Equal(t, "Has()", has(t, spy, 1), true)
	})

	t.Run("should fail open, giving the errors of the adaptor to the handler", func(t *testing.T) {
		spy, _ := newSpy(t)
		spy.getErr = func(int32) error { return errBoom }
		spy.setErr = errors.New("set")
		load, calls := counter()
		var handled []error
		l := newLoader(t, spy, load, cache.WithErrorHandler(func(ctx context.Context, err error) {
			if ctx == nil {
				t.Error("nil context")
			}
			handled = append(handled, err)
		}))
		got := &item{Id: 1}
		testutil.NoError(t, l.Get(context.Background(), got))
		testutil.Equal(t, "item", got, &item{Id: 1, Name: "loaded"})
		testutil.Equal(t, "handled", handled, []error{errBoom, errBoom, spy.setErr})

		spy.setErr = nil
		testutil.NoError(t, newLoader(t, spy, load, cache.WithTTL(time.Second)).Get(context.Background(), &item{Id: 2}))
		testutil.Equal(t, "loads", calls.Load(), int32(2))
		testutil.Equal(t, "Has()", has(t, spy, 2), true)
	})

	t.Run("should fail closed with the errors of the adaptor", func(t *testing.T) {
		spy, _ := newSpy(t)
		load, calls := counter()
		l := newLoader(t, spy, load, cache.WithFailClosed(), cache.WithTTL(time.Second))

		spy.getErr = func(int32) error { return errBoom }
		got := &item{Id: 1}
		if err := l.Get(context.Background(), got); err != errBoom {
			t.Fatalf("Get() = %v, want errBoom", err)
		}
		spy.gets.Store(0)
		spy.getErr = func(n int32) error {
			if n == 2 { // the second get of the miss
				return errBoom
			}
			return nil
		}
		if err := l.Get(context.Background(), got); err != errBoom {
			t.Fatalf("Get() = %v, want errBoom", err)
		}
		testutil.Equal(t, "loads", calls.Load(), int32(0))

		spy.getErr = nil
		spy.setErr = errors.New("set")
		if err := l.Get(context.Background(), got); err != spy.setErr {
			t.Fatalf("Get() = %v, want the set error", err)
		}
		testutil.Equal(t, "item", got, &item{Id: 1})
		testutil.Equal(t, "loads", calls.Load(), int32(1))
	})

	t.Run("should fail when the load function panics", func(t *testing.T) {
		spy, _ := newSpy(t)
		l := newLoader(t, spy, func(context.Context, *item) error { panic("oops") })
		got := &item{Id: 1}
		testutil.Error(t, l.Get(context.Background(), got), cache.ErrLoadPanic,
			"unable to load cache object: cache loader load function panicked",
			map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldKey: "cache.item:1", cache.ErrorFieldPanic: "oops"})
		testutil.Equal(t, "item", got, &item{Id: 1})
	})

	t.Run("should stop waiting once the context is done, the load going on", func(t *testing.T) {
		spy, _ := newSpy(t)
		started, release := make(chan struct{}), make(chan struct{})
		var loadCtx context.Context
		l := newLoader(t, spy, func(ctx context.Context, i *item) error {
			loadCtx = ctx
			close(started)
			<-release
			i.Name = "loaded"
			return nil
		})
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-started
			cancel()
		}()
		got := &item{Id: 1}
		err := l.Get(ctx, got)
		testutil.Error(t, err, context.Canceled, "unable to get from cache loader: context canceled",
			map[string]any{cache.ErrorFieldID: "main", cache.ErrorFieldKey: "cache.item:1"})
		testutil.Equal(t, "item", got, &item{Id: 1})

		testutil.NoError(t, loadCtx.Err())
		close(release)
		deadline := time.Now().Add(5 * time.Second)
		for !has(t, spy, 1) {
			if time.Now().After(deadline) {
				t.Fatal("the load did not store the item")
			}
			time.Sleep(time.Millisecond)
		}
	})

	t.Run("should load once for the concurrent callers of a key", func(t *testing.T) {
		const callers = 32
		spy, _ := newSpy(t)
		var calls atomic.Int32
		l := newLoader(t, spy, func(_ context.Context, i *item) error {
			calls.Add(1)
			// every caller got the item once, and the first one again
			for spy.gets.Load() < callers+1 {
				time.Sleep(time.Millisecond)
			}
			i.Name = "loaded"
			return nil
		})

		items := make([]*item, callers)
		errs := make([]error, callers)
		var wg sync.WaitGroup
		for i := range callers {
			items[i] = &item{Id: 1}
			wg.Go(func() { errs[i] = l.Get(context.Background(), items[i]) })
		}
		wg.Wait()
		for i := range callers {
			testutil.NoError(t, errs[i])
			testutil.Equal(t, "item", items[i], &item{Id: 1, Name: "loaded"})
		}
		testutil.Equal(t, "loads", calls.Load(), int32(1))
	})
}
