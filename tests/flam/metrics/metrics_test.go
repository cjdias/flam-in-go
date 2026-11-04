package metrics

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// newApp returns an app with the config and the metrics bundles, the
// configuration holding the values, and the collectors provided in the
// collector group.
func newApp(t *testing.T, values map[string]any, provided ...prometheus.Collector) *testutil.App {
	t.Helper()
	app := testutil.NewApp()
	for _, c := range provided {
		testutil.NoError(t, app.Provide(func() prometheus.Collector { return c }, dig.Group(metrics.CollectorGroup)))
	}
	testutil.NoError(t, app.RegisterBundle(config.NewBundle()))
	testutil.NoError(t, app.RegisterBundle(metrics.NewBundle()))
	bag := container.Bag{}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return app
}

// resolve returns the registerer and the gatherer of the container of the
// app, failing the test when they cannot be resolved.
func resolve(t *testing.T, app *testutil.App) (prometheus.Registerer, prometheus.Gatherer) {
	t.Helper()
	var registerer prometheus.Registerer
	var gatherer prometheus.Gatherer
	testutil.NoError(t, app.Invoke(func(r prometheus.Registerer, g prometheus.Gatherer) {
		registerer, gatherer = r, g
	}))
	return registerer, gatherer
}

// registered reports whether a collector of the same metrics as the one
// given is registered on the registerer, failing the test on any other
// error.
func registered(t *testing.T, registerer prometheus.Registerer, c prometheus.Collector) bool {
	t.Helper()
	err := registerer.Register(c)
	if err == nil {
		return false
	}
	var already prometheus.AlreadyRegisteredError
	if !errors.As(err, &already) {
		t.Fatalf("Register() = %v, want nil or an AlreadyRegisteredError", err)
	}
	return true
}

// duplicate is a collector of the metrics of the collector it embeds.
type duplicate struct{ prometheus.Collector }

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"CollectorGroup":             {metrics.CollectorGroup, "flam.metrics.collectors"},
		"GoCollectorConfigPath":      {metrics.GoCollectorConfigPath, "flam.metrics.collectors.go"},
		"ProcessCollectorConfigPath": {metrics.ProcessCollectorConfigPath, "flam.metrics.collectors.process"},
		"ErrorFieldCollector":        {metrics.ErrorFieldCollector, "collector"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrNilCollector": metrics.ErrNilCollector,
		"ErrRegister":     metrics.ErrRegister,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the registry provider", func(t *testing.T) {
		b := metrics.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.metrics")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.metrics", "provider/1"})
	})

	t.Run("should fail when the provider cannot be registered", func(t *testing.T) {
		app := testutil.NewApp()
		app.Fail = func(kind string) error {
			if kind == "provider" {
				return errors.New("boom")
			}
			return nil
		}
		if err := app.RegisterBundle(metrics.NewBundle()); err == nil {
			t.Fatal("RegisterBundle() did not fail")
		}
	})
}

func Test_Registry(t *testing.T) {
	t.Run("should provide a single registry holding the go and the process collectors", func(t *testing.T) {
		app := newApp(t, nil)
		registerer, gatherer := resolve(t, app)
		if _, ok := registerer.(*prometheus.Registry); !ok {
			t.Fatalf("registerer = %T, want a *prometheus.Registry", registerer)
		}
		testutil.Equal(t, "same registry", any(registerer) == any(gatherer), true)
		testutil.Equal(t, "default registry", registerer == prometheus.DefaultRegisterer, false)
		testutil.Equal(t, "go collector", registered(t, registerer, collectors.NewGoCollector()), true)
		testutil.Equal(t, "process collector", registered(t, registerer, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})), true)

		again, _ := resolve(t, app)
		testutil.Equal(t, "same registerer", again == registerer, true)
	})

	t.Run("should leave out the disabled go and process collectors", func(t *testing.T) {
		registerer, gatherer := resolve(t, newApp(t, map[string]any{
			metrics.GoCollectorConfigPath:      false,
			metrics.ProcessCollectorConfigPath: false,
		}))
		families, err := gatherer.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 0)
		testutil.Equal(t, "go collector", registered(t, registerer, collectors.NewGoCollector()), false)
		testutil.Equal(t, "process collector", registered(t, registerer, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})), false)
	})

	t.Run("should register the collectors of the group", func(t *testing.T) {
		counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "app_events_total", Help: "Events."})
		gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "app_items", Help: "Items."})
		counter.Add(3)
		gauge.Set(5)
		_, gatherer := resolve(t, newApp(t, map[string]any{metrics.ProcessCollectorConfigPath: false}, counter, gauge))
		testutil.Equal(t, "app metrics", promtestutil.CollectAndCount(counter)+promtestutil.CollectAndCount(gauge), 2)
		count, err := promtestutil.GatherAndCount(gatherer, "app_events_total", "app_items")
		testutil.NoError(t, err)
		testutil.Equal(t, "gathered", count, 2)
		testutil.Equal(t, "counter", promtestutil.ToFloat64(counter), 3.0)
		count, err = promtestutil.GatherAndCount(gatherer, "go_goroutines")
		testutil.NoError(t, err)
		testutil.Equal(t, "go metrics", count, 1)
	})

	t.Run("should fail on a nil collector of the group", func(t *testing.T) {
		app := newApp(t, nil, (*prometheus.CounterVec)(nil))
		err := app.Invoke(func(prometheus.Registerer) {})
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("error = %v, want a *flam.Error", err)
		}
		testutil.Error(t, e, metrics.ErrNilCollector, "unable to create metrics registry: nil metrics collector",
			map[string]any{metrics.ErrorFieldCollector: "*prometheus.CounterVec"})
	})

	t.Run("should fail on a collector of the group that cannot be registered", func(t *testing.T) {
		app := newApp(t, nil, duplicate{collectors.NewGoCollector()})
		err := app.Invoke(func(prometheus.Gatherer) {})
		var e *flam.Error
		if !errors.As(err, &e) {
			t.Fatalf("error = %v, want a *flam.Error", err)
		}
		if !errors.Is(e, metrics.ErrRegister) {
			t.Fatalf("error = %v, want it to wrap %v", e, metrics.ErrRegister)
		}
		var already prometheus.AlreadyRegisteredError
		testutil.Equal(t, "already registered", errors.As(e, &already), true)
		testutil.Context(t, e, map[string]any{metrics.ErrorFieldCollector: "metrics.duplicate"})
	})
}
