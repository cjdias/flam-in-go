package breakers

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/client"
	"github.com/cjdias/flam-in-go/pkg/flam/client/breakers"
	"github.com/cjdias/flam-in-go/pkg/flam/client/breakers/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// create returns the breaker "main" of the fields, created by the factory
// of an app with the client bundle, the standard driver bundle and its
// defaults, the values and the clock.
func create(t *testing.T, fields container.Bag, values map[string]any, clock flam.IClock) (flam.IClientCircuitBreaker, error) {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), client.NewBundle(), standard.NewBundle()}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = standard.Driver
	}
	testutil.NoError(t, bag.Set(client.BreakerConfigPath+".main", fields))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	app := testutil.NewApp()
	testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var b flam.IClientCircuitBreaker
	var err error
	testutil.NoError(t, app.Invoke(func(f flam.IClientCircuitBreakerFactory) { b, err = f.Get("main") }))
	return b, err
}

// newBreaker returns the breaker of the fields, failing the test when it
// cannot be created, and its clock.
func newBreaker(t *testing.T, fields container.Bag) (flam.IClientCircuitBreaker, *testutil.Clock) {
	t.Helper()
	clock := testutil.NewClock(time.Unix(0, 0))
	b, err := create(t, fields, nil, clock)
	testutil.NoError(t, err)
	return b, clock
}

// allow asks the breaker to let a request through, failing the test when
// the answer is not ok.
func allow(t *testing.T, b flam.IClientCircuitBreaker, ok bool) uint64 {
	t.Helper()
	generation, got := b.Allow()
	if got != ok {
		t.Fatalf("Allow() = %v, want %v", got, ok)
	}
	return generation
}

// state fails the test when the state of the breaker is not want.
func state(t *testing.T, b flam.IClientCircuitBreaker, want flam.ClientCircuitState) {
	t.Helper()
	testutil.Equal(t, "State()", b.State(), want)
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                                {standard.Id, "standard"},
		"Driver":                            {standard.Driver, "flam.client.breakers.driver.standard"},
		"ConfigFieldFailureThreshold":       {standard.ConfigFieldFailureThreshold, "failure_threshold"},
		"ConfigFieldOpenTimeout":            {standard.ConfigFieldOpenTimeout, "open_timeout"},
		"ConfigFieldHalfOpenRequests":       {standard.ConfigFieldHalfOpenRequests, "half_open_requests"},
		"DefaultFailureThresholdConfigPath": {standard.DefaultFailureThresholdConfigPath, "flam.client.defaults.breakers.standard.failure_threshold"},
		"DefaultOpenTimeoutConfigPath":      {standard.DefaultOpenTimeoutConfigPath, "flam.client.defaults.breakers.standard.open_timeout"},
		"DefaultHalfOpenRequestsConfigPath": {standard.DefaultHalfOpenRequestsConfigPath, "flam.client.defaults.breakers.standard.half_open_requests"},
		"ErrorFieldID":                      {standard.ErrorFieldID, "id"},
		"ErrorFieldField":                   {standard.ErrorFieldField, "field"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	if standard.ErrInvalidField == nil {
		t.Error("ErrInvalidField = nil")
	}
}

func Test_Bundles(t *testing.T) {
	t.Run("should register every driver", func(t *testing.T) {
		b := breakers.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.client.breakers")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{
			"bundle:flam.client.breakers", "bundle:flam.client.breakers.standard", "provider/1",
		})
	})

	t.Run("should configure the standard breaker and the defaults", func(t *testing.T) {
		b := standard.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.client.breakers.standard")

		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			client.BreakerConfigPath + "." + standard.Id: container.Bag{factory.ConfigFieldDriver: standard.Driver},
			standard.DefaultFailureThresholdConfigPath:   5,
			standard.DefaultOpenTimeoutConfigPath:        "30s",
			standard.DefaultHalfOpenRequestsConfigPath:   10,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should create a closed breaker with the defaults", func(t *testing.T) {
		b, clock := newBreaker(t, container.Bag{})
		testutil.Equal(t, "Id()", b.Id(), "main")
		state(t, b, flam.ClientCircuitClosed)

		// 5 failures open it, for 30s, then 10 trials close it
		for range 5 {
			b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		}
		state(t, b, flam.ClientCircuitOpen)
		clock.Advance(30*time.Second - 1)
		state(t, b, flam.ClientCircuitOpen)
		clock.Advance(1)
		state(t, b, flam.ClientCircuitHalfOpen)
		generations := make([]uint64, 10)
		for i := range generations {
			generations[i] = allow(t, b, true)
		}
		allow(t, b, false)
		for _, generation := range generations {
			b.Done(generation, flam.ClientOutcomeSuccess)
		}
		state(t, b, flam.ClientCircuitClosed)
	})

	t.Run("should create the default breaker", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
		bag := container.Bag{}
		for _, bundle := range []flam.IBundle{config.NewBundle(), client.NewBundle(), breakers.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(bundle))
		}
		testutil.NoError(t, standard.NewBundle().(flam.IConfigurableBundle).Config(bag))
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		testutil.NoError(t, app.Invoke(func(f flam.IClientCircuitBreakerFactory) {
			b, err := f.Get(standard.Id)
			testutil.NoError(t, err)
			testutil.Equal(t, "Id()", b.Id(), standard.Id)
		}))
	})

	t.Run("should read the fields, then the defaults", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		b, err := create(t, container.Bag{standard.ConfigFieldFailureThreshold: 2}, map[string]any{
			standard.DefaultFailureThresholdConfigPath: 7,
			standard.DefaultOpenTimeoutConfigPath:      time.Second,
			standard.DefaultHalfOpenRequestsConfigPath: 1,
		}, clock)
		testutil.NoError(t, err)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitClosed)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitOpen)
		clock.Advance(time.Second)
		b.Done(allow(t, b, true), flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitClosed)
	})

	t.Run("should use the built-in defaults without the default paths", func(t *testing.T) {
		clock := testutil.NewClock(time.Unix(0, 0))
		app := testutil.NewApp()
		testutil.NoError(t, app.Provide(func() flam.IClock { return clock }))
		for _, bundle := range []flam.IBundle{config.NewBundle(), client.NewBundle(), standard.NewBundle()} {
			testutil.NoError(t, app.RegisterBundle(bundle))
		}
		bag := container.Bag{}
		testutil.NoError(t, bag.Set(client.BreakerConfigPath+".main", container.Bag{factory.ConfigFieldDriver: standard.Driver}))
		testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
		testutil.NoError(t, app.Invoke(func(f flam.IClientCircuitBreakerFactory) {
			b, err := f.Get("main")
			testutil.NoError(t, err)
			for range 4 {
				b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
			}
			state(t, b, flam.ClientCircuitClosed)
			b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
			state(t, b, flam.ClientCircuitOpen)
			clock.Advance(30 * time.Second)
			for range 10 {
				allow(t, b, true)
			}
			allow(t, b, false)
		}))
	})

	t.Run("should not create a breaker of another driver", func(t *testing.T) {
		_, err := create(t, container.Bag{factory.ConfigFieldDriver: "other"}, nil, testutil.NewClock(time.Unix(0, 0)))
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want ErrNoCreator", err)
		}
	})

	for name, tc := range map[string]struct {
		fields container.Bag
		values map[string]any
		field  string
	}{
		"a failure threshold of another type":   {container.Bag{standard.ConfigFieldFailureThreshold: "5"}, nil, standard.ConfigFieldFailureThreshold},
		"a zero failure threshold":              {container.Bag{standard.ConfigFieldFailureThreshold: 0}, nil, standard.ConfigFieldFailureThreshold},
		"an invalid default failure threshold":  {container.Bag{}, map[string]any{standard.DefaultFailureThresholdConfigPath: -1}, standard.ConfigFieldFailureThreshold},
		"an invalid open timeout":               {container.Bag{standard.ConfigFieldOpenTimeout: "soon"}, nil, standard.ConfigFieldOpenTimeout},
		"a zero open timeout":                   {container.Bag{standard.ConfigFieldOpenTimeout: "0s"}, nil, standard.ConfigFieldOpenTimeout},
		"a negative open timeout":               {container.Bag{standard.ConfigFieldOpenTimeout: -time.Second}, nil, standard.ConfigFieldOpenTimeout},
		"a half open requests of another type":  {container.Bag{standard.ConfigFieldHalfOpenRequests: 1.5}, nil, standard.ConfigFieldHalfOpenRequests},
		"a zero half open requests":             {container.Bag{standard.ConfigFieldHalfOpenRequests: 0}, nil, standard.ConfigFieldHalfOpenRequests},
		"an invalid default half open requests": {container.Bag{}, map[string]any{standard.DefaultHalfOpenRequestsConfigPath: "1"}, standard.ConfigFieldHalfOpenRequests},
	} {
		t.Run("should fail to create a breaker with "+name, func(t *testing.T) {
			_, err := create(t, tc.fields, tc.values, testutil.NewClock(time.Unix(0, 0)))
			var e *flam.Error
			if !errors.Is(err, standard.ErrInvalidField) || !errors.As(err, &e) {
				t.Fatalf("Get() = %v, want ErrInvalidField", err)
			}
			for errors.As(e.Unwrap(), &e) {
			}
			testutil.Error(t, e, standard.ErrInvalidField, "unable to create standard client circuit breaker: invalid standard client circuit breaker field",
				map[string]any{standard.ErrorFieldID: "main", standard.ErrorFieldField: tc.field})
		})
	}
}

func Test_Breaker(t *testing.T) {
	fields := func() container.Bag {
		return container.Bag{
			standard.ConfigFieldFailureThreshold: 3,
			standard.ConfigFieldOpenTimeout:      "10s",
			standard.ConfigFieldHalfOpenRequests: 2,
		}
	}

	// open returns the breaker of the fields, opened by its failures.
	open := func(t *testing.T) (flam.IClientCircuitBreaker, *testutil.Clock) {
		t.Helper()
		b, clock := newBreaker(t, fields())
		for range 3 {
			b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		}
		state(t, b, flam.ClientCircuitOpen)
		return b, clock
	}

	t.Run("should reset the failures of a closed circuit on a success", func(t *testing.T) {
		b, _ := newBreaker(t, fields())
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeSuccess)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitClosed)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitOpen)
	})

	t.Run("should not count the ignored outcomes of a closed circuit", func(t *testing.T) {
		b, _ := newBreaker(t, fields())
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeIgnored)
		state(t, b, flam.ClientCircuitClosed)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitOpen)
	})

	t.Run("should refuse the requests while open", func(t *testing.T) {
		b, clock := open(t)
		generation := allow(t, b, false)
		testutil.Equal(t, "refused generation", generation, uint64(0))
		clock.Advance(9 * time.Second)
		allow(t, b, false)
		state(t, b, flam.ClientCircuitOpen)
	})

	t.Run("should let the trial requests through once half open", func(t *testing.T) {
		b, clock := open(t)
		clock.Advance(10 * time.Second)
		first := allow(t, b, true)
		second := allow(t, b, true)
		testutil.Equal(t, "same generation", first, second)
		allow(t, b, false)
		state(t, b, flam.ClientCircuitHalfOpen)

		b.Done(first, flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitHalfOpen)
		allow(t, b, false)
		b.Done(second, flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitClosed)
		allow(t, b, true)
	})

	t.Run("should open again on a failed trial", func(t *testing.T) {
		b, clock := open(t)
		clock.Advance(10 * time.Second)
		first := allow(t, b, true)
		second := allow(t, b, true)
		b.Done(first, flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitOpen)
		allow(t, b, false)

		// the trial of the former half open generation does not count
		b.Done(second, flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitOpen)

		clock.Advance(10*time.Second - 1)
		state(t, b, flam.ClientCircuitOpen)
		clock.Advance(1)
		state(t, b, flam.ClientCircuitHalfOpen)
	})

	t.Run("should free the slot of an ignored trial", func(t *testing.T) {
		b, clock := open(t)
		clock.Advance(10 * time.Second)
		first := allow(t, b, true)
		second := allow(t, b, true)
		allow(t, b, false)
		b.Done(first, flam.ClientOutcomeIgnored)
		state(t, b, flam.ClientCircuitHalfOpen)
		third := allow(t, b, true)
		allow(t, b, false)
		b.Done(second, flam.ClientOutcomeSuccess)
		b.Done(third, flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitClosed)
	})

	t.Run("should drop the outcomes of a former generation", func(t *testing.T) {
		b, clock := newBreaker(t, fields())
		slow := allow(t, b, true)
		for range 3 {
			b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		}
		clock.Advance(10 * time.Second)
		trial := allow(t, b, true)

		// a slow failure from before the circuit opened
		b.Done(slow, flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitHalfOpen)
		b.Done(trial, flam.ClientOutcomeSuccess)
		b.Done(allow(t, b, true), flam.ClientOutcomeSuccess)
		state(t, b, flam.ClientCircuitClosed)

		// nor once closed again
		b.Done(slow, flam.ClientOutcomeFailure)
		b.Done(trial, flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		b.Done(allow(t, b, true), flam.ClientOutcomeFailure)
		state(t, b, flam.ClientCircuitClosed)
	})

	t.Run("should ignore an outcome reported while open", func(t *testing.T) {
		b, _ := open(t)
		allow(t, b, false)
		// the generation of the open circuit, which no request goes through
		for _, outcome := range []flam.ClientOutcome{flam.ClientOutcomeSuccess, flam.ClientOutcomeFailure, flam.ClientOutcomeIgnored} {
			b.Done(1, outcome)
			state(t, b, flam.ClientCircuitOpen)
		}
	})

	t.Run("should be safe for concurrent use", func(t *testing.T) {
		b, clock := newBreaker(t, container.Bag{standard.ConfigFieldFailureThreshold: 50, standard.ConfigFieldHalfOpenRequests: 5})
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				for j := range 20 {
					if generation, ok := b.Allow(); ok {
						b.Done(generation, flam.ClientOutcome((i+j)%3))
					}
					_ = b.State()
				}
			})
		}
		wg.Go(func() {
			for range 20 {
				clock.Advance(time.Second)
			}
		})
		wg.Wait()
	})
}
