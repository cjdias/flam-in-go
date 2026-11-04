package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// start is the time of the fake clocks.
var start = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// newApp returns a testutil.App with the config bundle, the fake clock, the
// bundles given, and a configuration holding the values, at paths.
func newApp(t *testing.T, clock flam.IClock, values map[string]any, bundles ...flam.IBundle) *testutil.App {
	t.Helper()
	bag := container.Bag{}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	a := testutil.NewApp()
	testutil.NoError(t, a.Provide(func() flam.IClock { return clock }))
	for _, b := range append([]flam.IBundle{config.NewBundle()}, bundles...) {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// registryOf returns the registry of the app.
func registryOf(t *testing.T, a flam.IApp) flam.IHealthRegistry {
	t.Helper()
	var r flam.IHealthRegistry
	testutil.NoError(t, a.Invoke(func(provided flam.IHealthRegistry) { r = provided }))
	return r
}

// newRegistry returns the registry of a new app with the health bundle and
// the configuration values.
func newRegistry(t *testing.T, clock flam.IClock, values map[string]any) flam.IHealthRegistry {
	t.Helper()
	return registryOf(t, newApp(t, clock, values, health.NewBundle()))
}

// mustCheck returns the check of NewCheck, failing the test on an error.
func mustCheck(t *testing.T, id string, kind flam.HealthKind, fn func(ctx context.Context) error) flam.IHealthCheck {
	t.Helper()
	check, err := health.NewCheck(id, kind, fn)
	testutil.NoError(t, err)
	return check
}

// ok is a check function that succeeds.
func ok(context.Context) error { return nil }

// fakeCheck is a flam.IHealthCheck of any kind, failing to close with
// its error.
type fakeCheck struct {
	id     string
	kind   flam.HealthKind
	closed error
}

func (c *fakeCheck) Id() string                  { return c.id }
func (c *fakeCheck) Kind() flam.HealthKind       { return c.kind }
func (c *fakeCheck) Check(context.Context) error { return nil }
func (c *fakeCheck) Close() error                { return c.closed }

// fakeCreator is a flam.IHealthCheckCreator of the "fake" driver,
// creating checks of the both kind failing to close with its error.
type fakeCreator struct {
	closed error
}

func (fakeCreator) Accept(bag flam.IBag) bool {
	return bag.String(factory.ConfigFieldDriver) == "fake"
}

func (c fakeCreator) Create(bag flam.IBag) (flam.IHealthCheck, error) {
	return &fakeCheck{id: bag.String(factory.ConfigFieldId), kind: flam.HealthBoth, closed: c.closed}, nil
}

// fakeBundle provides the fakeCreator, and the checks in the check group.
type fakeBundle struct {
	closed error
	checks []flam.IHealthCheck
}

func (fakeBundle) Id() string { return "fake" }

func (b fakeBundle) Register(a flam.IApp) error {
	if err := a.Provide(func() flam.IHealthCheckCreator { return fakeCreator{closed: b.closed} }, dig.Group(health.CheckCreatorGroup)); err != nil {
		return err
	}
	return a.Provide(func() []flam.IHealthCheck { return b.checks }, dig.Group(health.CheckGroup+",flatten"))
}

// waitTimers waits for the clock to hold n timers.
func waitTimers(clock *testutil.Clock, n int) {
	for clock.Timers() != n {
		runtime.Gosched()
	}
}

func Test_Declarations(t *testing.T) {
	for _, tc := range [][2]string{
		{health.CheckConfigPath, "flam.health.checks"},
		{health.CheckCreatorGroup, "flam.health.checks.creator"},
		{health.CheckGroup, "flam.health.checks"},
		{health.TimeoutConfigPath, "flam.health.timeout"},
		{health.CacheConfigPath, "flam.health.cache"},
		{health.ConfigFieldKind, "kind"},
		{health.DefaultKindConfigPath, "flam.health.defaults.checks.kind"},
		{health.KindLiveness, "liveness"},
		{health.KindReadiness, "readiness"},
		{health.KindBoth, "both"},
		{health.ErrorFieldID, "id"},
		{health.ErrorFieldField, "field"},
		{health.ErrorFieldKind, "kind"},
		{health.ErrorFieldPanic, "panic"},
	} {
		testutil.Equal(t, "const", tc[0], tc[1])
	}
	testutil.Equal(t, "HealthLiveness", flam.HealthLiveness, flam.HealthKind(1))
	testutil.Equal(t, "HealthReadiness", flam.HealthReadiness, flam.HealthKind(2))
	testutil.Equal(t, "HealthBoth", flam.HealthBoth, flam.HealthKind(3))
	testutil.Equal(t, "HealthStatusUp", flam.HealthStatusUp, flam.HealthStatus("up"))
	testutil.Equal(t, "HealthStatusDown", flam.HealthStatusDown, flam.HealthStatus("down"))
}

func Test_HealthReport(t *testing.T) {
	t.Run("should tell whether it is healthy", func(t *testing.T) {
		testutil.Equal(t, "up", flam.HealthReport{Status: flam.HealthStatusUp}.Healthy(), true)
		testutil.Equal(t, "down", flam.HealthReport{Status: flam.HealthStatusDown}.Healthy(), false)
		testutil.Equal(t, "zero", flam.HealthReport{}.Healthy(), false)
	})

	t.Run("should write the durations as text, and leave out the empty errors", func(t *testing.T) {
		data, err := json.Marshal(flam.HealthReport{
			Status: flam.HealthStatusDown,
			Checks: []flam.HealthCheckReport{
				{Id: "a", Status: flam.HealthStatusUp, Duration: 1500 * time.Microsecond},
				{Id: "b", Status: flam.HealthStatusDown, Error: "boom", Duration: 2 * time.Second},
			},
		})
		testutil.NoError(t, err)
		testutil.Equal(t, "json", string(data), `{"status":"down","checks":[`+
			`{"id":"a","status":"up","duration":"1.5ms"},`+
			`{"id":"b","status":"down","error":"boom","duration":"2s"}]}`)
	})
}

func Test_NewCheck(t *testing.T) {
	t.Run("should refuse an empty id", func(t *testing.T) {
		check, err := health.NewCheck("", flam.HealthBoth, ok)
		testutil.Error(t, err, health.ErrInvalidID, "unable to create health check: invalid health check id",
			map[string]any{health.ErrorFieldID: ""})
		testutil.Equal(t, "check", check, nil)
	})

	t.Run("should refuse an invalid kind", func(t *testing.T) {
		for _, kind := range []flam.HealthKind{0, 4, -1} {
			_, err := health.NewCheck("a", kind, ok)
			testutil.Error(t, err, health.ErrInvalidKind, "unable to create health check: invalid health check kind",
				map[string]any{health.ErrorFieldID: "a", health.ErrorFieldKind: int(kind)})
		}
	})

	t.Run("should refuse a nil function", func(t *testing.T) {
		_, err := health.NewCheck("a", flam.HealthBoth, nil)
		testutil.Error(t, err, health.ErrNilCheck, "unable to create health check: nil health check",
			map[string]any{health.ErrorFieldID: "a"})
	})

	t.Run("should check with the function", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), errBoom, "value")
		check := mustCheck(t, "a", flam.HealthReadiness, func(got context.Context) error {
			testutil.Equal(t, "ctx", got, ctx)
			return errBoom
		})
		testutil.Equal(t, "Id()", check.Id(), "a")
		testutil.Equal(t, "Kind()", check.Kind(), flam.HealthReadiness)
		testutil.Equal(t, "Check()", check.Check(ctx), errBoom)
	})
}

func Test_ParseKind(t *testing.T) {
	for _, tc := range []struct {
		value any
		kind  flam.HealthKind
		ok    bool
	}{
		{"liveness", flam.HealthLiveness, true},
		{"readiness", flam.HealthReadiness, true},
		{"both", flam.HealthBoth, true},
		{flam.HealthReadiness, flam.HealthReadiness, true},
		{"Both", 0, false},
		{flam.HealthKind(7), 7, false},
		{1, 0, false},
		{nil, 0, false},
	} {
		kind, valid := health.ParseKind(tc.value)
		testutil.Equal(t, "kind", kind, tc.kind)
		testutil.Equal(t, "ok", valid, tc.ok)
	}
}

func Test_Registry_Add(t *testing.T) {
	t.Run("should hold the checks added", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		testutil.Equal(t, "Available()", r.Available(), []string{})
		testutil.NoError(t, r.Add(mustCheck(t, "b", flam.HealthBoth, ok)))
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, ok)))

		testutil.Equal(t, "Available()", r.Available(), []string{"a", "b"})
		testutil.Equal(t, "Has(a)", r.Has("a"), true)
		testutil.Equal(t, "Has(c)", r.Has("c"), false)
	})

	t.Run("should refuse a nil check", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		for _, check := range []flam.IHealthCheck{nil, (*fakeCheck)(nil)} {
			testutil.Error(t, r.Add(check), health.ErrNilCheck, "unable to add health check: nil health check",
				map[string]any{health.ErrorFieldID: ""})
		}
	})

	t.Run("should refuse an empty id or an invalid kind", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		testutil.Error(t, r.Add(&fakeCheck{kind: flam.HealthBoth}), health.ErrInvalidID,
			"unable to add health check: invalid health check id", map[string]any{health.ErrorFieldID: ""})
		testutil.Error(t, r.Add(&fakeCheck{id: "a"}), health.ErrInvalidKind,
			"unable to add health check: invalid health check kind",
			map[string]any{health.ErrorFieldID: "a", health.ErrorFieldKind: 0})
		testutil.Equal(t, "Available()", r.Available(), []string{})
	})

	t.Run("should refuse an id in use", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, ok)))
		testutil.Error(t, r.Add(mustCheck(t, "a", flam.HealthLiveness, ok)), health.ErrDuplicateCheck,
			"unable to add health check: health check id already in use", map[string]any{health.ErrorFieldID: "a"})
	})
}

func Test_Registry_Probes(t *testing.T) {
	t.Run("should succeed without check", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		want := flam.HealthReport{Status: flam.HealthStatusUp, Checks: []flam.HealthCheckReport{}}
		testutil.Equal(t, "Liveness()", r.Liveness(context.Background()), want)
		testutil.Equal(t, "Readiness()", r.Readiness(context.Background()), want)
	})

	t.Run("should run the checks of the probe, in id order", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		testutil.NoError(t, r.Add(mustCheck(t, "live", flam.HealthLiveness, ok)))
		testutil.NoError(t, r.Add(mustCheck(t, "ready", flam.HealthReadiness, func(context.Context) error { return errBoom })))
		testutil.NoError(t, r.Add(mustCheck(t, "both", flam.HealthBoth, ok)))

		testutil.Equal(t, "Liveness()", r.Liveness(context.Background()), flam.HealthReport{
			Status: flam.HealthStatusUp,
			Checks: []flam.HealthCheckReport{
				{Id: "both", Status: flam.HealthStatusUp},
				{Id: "live", Status: flam.HealthStatusUp},
			},
		})
		testutil.Equal(t, "Readiness()", r.Readiness(context.Background()), flam.HealthReport{
			Status: flam.HealthStatusDown,
			Checks: []flam.HealthCheckReport{
				{Id: "both", Status: flam.HealthStatusUp},
				{Id: "ready", Status: flam.HealthStatusDown, Error: "boom"},
			},
		})
	})

	t.Run("should run the checks concurrently", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), map[string]any{health.TimeoutConfigPath: "0s"})
		var barrier sync.WaitGroup
		barrier.Add(2)
		for _, id := range []string{"a", "b"} {
			testutil.NoError(t, r.Add(mustCheck(t, id, flam.HealthBoth, func(context.Context) error {
				barrier.Done()
				barrier.Wait() // ends once both run
				return nil
			})))
		}
		testutil.Equal(t, "Healthy()", r.Liveness(context.Background()).Healthy(), true)
	})

	t.Run("should time the checks, and fail those not ending within the timeout", func(t *testing.T) {
		clock := testutil.NewClock(start)
		r := newRegistry(t, clock, map[string]any{health.TimeoutConfigPath: "3s"})
		release := make(chan struct{})
		defer close(release)
		testutil.NoError(t, r.Add(mustCheck(t, "slow", flam.HealthBoth, func(context.Context) error {
			<-release // ignores its context
			return nil
		})))

		reports := make(chan flam.HealthReport)
		go func() { reports <- r.Readiness(context.Background()) }()
		waitTimers(clock, 1)
		clock.Advance(3 * time.Second)

		testutil.Equal(t, "Readiness()", <-reports, flam.HealthReport{
			Status: flam.HealthStatusDown,
			Checks: []flam.HealthCheckReport{{
				Id: "slow", Status: flam.HealthStatusDown, Duration: 3 * time.Second,
				Error: "unable to run health check: health check timed out",
			}},
		})
		testutil.Equal(t, "Timers()", clock.Timers(), 0)
	})

	t.Run("should time out after 2s unless configured", func(t *testing.T) {
		clock := testutil.NewClock(start)
		r := newRegistry(t, clock, nil)
		testutil.NoError(t, r.Add(mustCheck(t, "slow", flam.HealthBoth, func(ctx context.Context) error {
			<-ctx.Done()
			return context.Cause(ctx)
		})))

		reports := make(chan flam.HealthReport)
		go func() { reports <- r.Liveness(context.Background()) }()
		waitTimers(clock, 1)
		clock.Advance(2*time.Second - 1)
		clock.Advance(1)

		report := <-reports
		testutil.Equal(t, "Error", report.Checks[0].Error, "unable to run health check: health check timed out")
		testutil.Equal(t, "Duration", report.Checks[0].Duration, 2*time.Second)
	})

	t.Run("should end the checks with the context of the probe", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		ctx, cancel := context.WithCancel(context.Background())
		release := make(chan struct{})
		defer close(release)
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, func(context.Context) error {
			cancel()
			<-release // ignores its context
			return nil
		})))

		testutil.Equal(t, "Liveness()", r.Liveness(ctx).Checks, []flam.HealthCheckReport{
			{Id: "a", Status: flam.HealthStatusDown, Error: "context canceled"},
		})
	})

	t.Run("should fail the checks that panic", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		testutil.NoError(t, r.Add(mustCheck(t, "error", flam.HealthBoth, func(context.Context) error { panic(errBoom) })))
		testutil.NoError(t, r.Add(mustCheck(t, "value", flam.HealthBoth, func(context.Context) error { panic(42) })))

		testutil.Equal(t, "Liveness()", r.Liveness(context.Background()).Checks, []flam.HealthCheckReport{
			{Id: "error", Status: flam.HealthStatusDown, Error: "unable to run health check: health check panicked: boom"},
			{Id: "value", Status: flam.HealthStatusDown, Error: "unable to run health check: health check panicked: 42"},
		})
	})

	t.Run("should reuse the results of the checks for the cache time", func(t *testing.T) {
		clock := testutil.NewClock(start)
		r := newRegistry(t, clock, map[string]any{health.CacheConfigPath: "1s"})
		var calls atomic.Int32
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, func(context.Context) error {
			calls.Add(1)
			return nil
		})))

		r.Liveness(context.Background())
		r.Readiness(context.Background())
		clock.Advance(time.Second - 1)
		r.Liveness(context.Background())
		testutil.Equal(t, "calls", calls.Load(), int32(1))

		clock.Advance(1)
		r.Liveness(context.Background())
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should not cache the results of a probe whose context is done", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), map[string]any{health.CacheConfigPath: "1s"})
		var calls atomic.Int32
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, func(context.Context) error {
			calls.Add(1)
			return nil
		})))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		r.Liveness(ctx)
		r.Liveness(context.Background())
		r.Liveness(context.Background())
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})

	t.Run("should run the checks for every probe without cache", func(t *testing.T) {
		r := newRegistry(t, testutil.NewClock(start), nil)
		var calls atomic.Int32
		testutil.NoError(t, r.Add(mustCheck(t, "a", flam.HealthBoth, func(context.Context) error {
			calls.Add(1)
			return nil
		})))

		r.Liveness(context.Background())
		r.Liveness(context.Background())
		testutil.Equal(t, "calls", calls.Load(), int32(2))
	})
}

func Test_Registry_Config(t *testing.T) {
	for _, path := range []string{health.TimeoutConfigPath, health.CacheConfigPath} {
		for _, value := range []any{"abc", "-1s", 5} {
			a := newApp(t, testutil.NewClock(start), map[string]any{path: value}, health.NewBundle())
			err := a.Invoke(func(flam.IHealthRegistry) {})
			if !errors.Is(err, health.ErrInvalidField) {
				t.Fatalf("Invoke() = %v, want health.ErrInvalidField", err)
			}
			var e *flam.Error
			errors.As(err, &e)
			testutil.Equal(t, "Error()", e.Error(), "unable to create health registry: invalid health field")
			testutil.Context(t, e, map[string]any{health.ErrorFieldField: path})
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should provide the registry and the factory", func(t *testing.T) {
		b := health.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.health")

		a := newApp(t, testutil.NewClock(start), nil, b)
		testutil.Equal(t, "calls", a.Calls()[len(a.Calls())-4:], []string{
			"bundle:flam.health", "provider/0", "provider/0", "invoker/0",
		})
		testutil.NoError(t, a.Invoke(func(f flam.IHealthCheckFactory) {
			testutil.Equal(t, "Available()", f.Available(), []string{})
		}))
	})

	t.Run("should add the checks of the group, then the configured ones, at boot", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, testutil.NewClock(start), map[string]any{
			health.CheckConfigPath + ".conf": container.Bag{factory.ConfigFieldDriver: "fake"},
		}, b, fakeBundle{checks: []flam.IHealthCheck{
			mustCheck(t, "b", flam.HealthLiveness, ok),
			mustCheck(t, "a", flam.HealthReadiness, ok),
		}})

		testutil.NoError(t, b.(flam.IBootableBundle).Boot(a))
		r := registryOf(t, a)
		testutil.Equal(t, "Available()", r.Available(), []string{"a", "b", "conf"})
		testutil.Equal(t, "Liveness()", r.Liveness(context.Background()), flam.HealthReport{
			Status: flam.HealthStatusUp,
			Checks: []flam.HealthCheckReport{
				{Id: "b", Status: flam.HealthStatusUp},
				{Id: "conf", Status: flam.HealthStatusUp},
			},
		})
	})

	t.Run("should fail to boot with a check of the group that cannot be added", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, testutil.NewClock(start), nil, b, fakeBundle{checks: []flam.IHealthCheck{
			mustCheck(t, "a", flam.HealthLiveness, ok),
			nil,
		}})

		err := b.(flam.IBootableBundle).Boot(a)
		testutil.Error(t, err, health.ErrCheck,
			"unable to boot health bundle: unable to add health check: unable to add health check: nil health check",
			map[string]any{health.ErrorFieldID: ""})
		if !errors.Is(err, health.ErrNilCheck) {
			t.Fatalf("Boot() = %v, want health.ErrNilCheck", err)
		}
	})

	t.Run("should fail to boot with a configured check that cannot be added", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, testutil.NewClock(start), map[string]any{
			health.CheckConfigPath + ".a": container.Bag{factory.ConfigFieldDriver: "fake"},
		}, b, fakeBundle{checks: []flam.IHealthCheck{mustCheck(t, "a", flam.HealthLiveness, ok)}})

		err := b.(flam.IBootableBundle).Boot(a)
		if !errors.Is(err, health.ErrCheck) || !errors.Is(err, health.ErrDuplicateCheck) {
			t.Fatalf("Boot() = %v, want health.ErrCheck and health.ErrDuplicateCheck", err)
		}
	})

	t.Run("should fail to boot with a configured check that cannot be created", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, testutil.NewClock(start), map[string]any{
			health.CheckConfigPath + ".a": container.Bag{factory.ConfigFieldDriver: "unknown"},
		}, b)

		err := b.(flam.IBootableBundle).Boot(a)
		if !errors.Is(err, health.ErrCheck) || !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Boot() = %v, want health.ErrCheck and factory.ErrNoCreator", err)
		}
		var e *flam.Error
		errors.As(err, &e)
		testutil.Context(t, e, map[string]any{health.ErrorFieldID: "a"})
	})

	t.Run("should close the factory, and so its checks", func(t *testing.T) {
		b := health.NewBundle()
		a := newApp(t, testutil.NewClock(start), map[string]any{
			health.CheckConfigPath + ".a": container.Bag{factory.ConfigFieldDriver: "fake"},
		}, b, fakeBundle{closed: errBoom})
		testutil.NoError(t, b.(flam.IBootableBundle).Boot(a))

		if err := b.(flam.IClosableBundle).Close(a); !errors.Is(err, errBoom) {
			t.Fatalf("Close() = %v, want errBoom", err)
		}
		testutil.NoError(t, b.(flam.IClosableBundle).Close(a))
	})

	t.Run("should fail to boot or close without the dependencies", func(t *testing.T) {
		b := health.NewBundle()
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))

		if err := b.(flam.IBootableBundle).Boot(a); err == nil {
			t.Fatal("Boot() = nil, want an error")
		}
		if err := b.(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() = nil, want an error")
		}
	})
}

// compile time check that *fakeCheck implements io.Closer
var _ io.Closer = (*fakeCheck)(nil)
