package interceptors

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	flamgrpc "github.com/cjdias/flam-in-go/pkg/flam/grpc"
	"github.com/cjdias/flam-in-go/pkg/flam/grpc/interceptors/metrics"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// Help texts of the metrics, as registered by the interceptors.
const (
	handledHelp  = "Number of gRPC calls served, by service, method and code."
	durationHelp = "Time taken to serve the gRPC calls, in seconds, by service, method and code."
)

// measuring returns the metrics interceptor "main" of the fields, with the
// clock, and the registry when not nil.
func measuring(t *testing.T, driver flam.IBundle, fields container.Bag, values map[string]any, c *testutil.Clock, registry *prometheus.Registry) (flam.IGrpcInterceptor, error) {
	t.Helper()
	if !fields.Has(factory.ConfigFieldDriver) {
		fields[factory.ConfigFieldDriver] = metrics.Driver
	}
	providers := []any{clock(c)}
	if registry != nil {
		providers = append(providers, func() prometheus.Registerer { return registry })
	}
	return create(t, driver, fields, values, providers...)
}

// family returns the family of the metric of the name gathered from the
// registry, or nil.
func family(t *testing.T, registry *prometheus.Registry, name string) *dto.MetricFamily {
	t.Helper()
	families, err := registry.Gather()
	testutil.NoError(t, err)
	for _, f := range families {
		if f.GetName() == name {
			return f
		}
	}
	return nil
}

// series returns the series of the family, by their labels, such as
// "pkg.Users/Get/OK".
func series(f *dto.MetricFamily) map[string]*dto.Metric {
	all := map[string]*dto.Metric{}
	for _, m := range f.GetMetric() {
		labels := map[string]string{}
		for _, pair := range m.GetLabel() {
			labels[pair.GetName()] = pair.GetValue()
		}
		all[labels[metrics.LabelService]+"/"+labels[metrics.LabelMethod]+"/"+labels[metrics.LabelCode]] = m
	}
	return all
}

func Test_Metrics_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                         {metrics.Id, "metrics"},
		"Driver":                     {metrics.Driver, "flam.grpc.interceptors.driver.metrics"},
		"ConfigFieldNamespace":       {metrics.ConfigFieldNamespace, "namespace"},
		"ConfigFieldBuckets":         {metrics.ConfigFieldBuckets, "buckets"},
		"DefaultNamespaceConfigPath": {metrics.DefaultNamespaceConfigPath, "flam.grpc.defaults.interceptors.metrics.namespace"},
		"DefaultBucketsConfigPath":   {metrics.DefaultBucketsConfigPath, "flam.grpc.defaults.interceptors.metrics.buckets"},
		"MetricHandled":              {metrics.MetricHandled, "server_handled_total"},
		"MetricDuration":             {metrics.MetricDuration, "server_handling_seconds"},
		"Subsystem":                  {metrics.Subsystem, "grpc"},
		"LabelService":               {metrics.LabelService, "service"},
		"LabelMethod":                {metrics.LabelMethod, "method"},
		"LabelCode":                  {metrics.LabelCode, "code"},
		"CodePanic":                  {metrics.CodePanic, "panic"},
		"ErrorFieldID":               {metrics.ErrorFieldID, "id"},
		"ErrorFieldField":            {metrics.ErrorFieldField, "field"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	testutil.Equal(t, "ErrInvalidField", metrics.ErrInvalidField.Error(), "invalid metrics grpc interceptor field")
	testutil.Equal(t, "ErrMissingRegisterer", metrics.ErrMissingRegisterer.Error(), "metrics registerer not provided by the container")
	testutil.Equal(t, "ErrRegister", metrics.ErrRegister.Error(), "unable to register metrics grpc interceptor metrics")
}

func Test_Metrics_Bundle(t *testing.T) {
	b := metrics.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.grpc.interceptors.metrics")

	bag := container.Bag{}
	testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
	for path, want := range map[string]any{
		flamgrpc.InterceptorConfigPath + "." + metrics.Id: container.Bag{factory.ConfigFieldDriver: metrics.Driver},
		metrics.DefaultNamespaceConfigPath:                "flam",
		metrics.DefaultBucketsConfigPath:                  []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	} {
		testutil.Equal(t, path, bag.Get(path), want)
	}
	if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
		t.Fatal("Config() did not fail")
	}
}

func Test_Metrics_Interceptor(t *testing.T) {
	t.Run("should count and time the calls, by service, method and code", func(t *testing.T) {
		registry, c := prometheus.NewRegistry(), newClock()
		i, err := measuring(t, metrics.NewBundle(), container.Bag{}, nil, c, registry)
		testutil.NoError(t, err)
		testutil.Equal(t, "Id()", i.Id(), "main")

		resp, err := unary(i, context.Background(), c, "resp", nil, false)
		testutil.NoError(t, err)
		testutil.Equal(t, "resp", resp, "resp")
		notFound := status.Error(codes.NotFound, "")
		_, err = unary(i, context.Background(), c, nil, notFound, false)
		testutil.Equal(t, "unary error", err, notFound)
		testutil.Equal(t, "stream error", streaming(i, context.Background(), c, notFound, false), notFound)
		testutil.Panics(t, "unary panic", func() { _, _ = unary(i, context.Background(), c, nil, nil, true) })
		testutil.Panics(t, "stream panic", func() { _ = streaming(i, context.Background(), c, nil, true) })

		handled := series(family(t, registry, "flam_grpc_server_handled_total"))
		duration := series(family(t, registry, "flam_grpc_server_handling_seconds"))
		testutil.Equal(t, "series", len(handled), 3)
		for labels, count := range map[string]uint64{
			"pkg.Users/Get/OK":       1,
			"pkg.Users/Get/NotFound": 2,
			"pkg.Users/Get/panic":    2,
		} {
			testutil.Equal(t, labels+" handled", handled[labels].GetCounter().GetValue(), float64(count))
			testutil.Equal(t, labels+" count", duration[labels].GetHistogram().GetSampleCount(), count)
			testutil.Equal(t, labels+" sum", duration[labels].GetHistogram().GetSampleSum(), float64(count))
		}
		testutil.Equal(t, "help", family(t, registry, "flam_grpc_server_handled_total").GetHelp(), handledHelp)
		testutil.Equal(t, "buckets", len(duration["pkg.Users/Get/OK"].GetHistogram().GetBucket()), 11)
	})

	t.Run("should share the metrics of a namespace", func(t *testing.T) {
		registry, c := prometheus.NewRegistry(), newClock()
		app := []map[string]any{
			{flamgrpc.InterceptorConfigPath + ".other": container.Bag{factory.ConfigFieldDriver: metrics.Driver}},
		}
		first, err := measuring(t, metrics.NewBundle(), container.Bag{}, app[0], c, registry)
		testutil.NoError(t, err)
		second, err := measuring(t, metrics.NewBundle(), container.Bag{}, nil, c, registry)
		testutil.NoError(t, err)
		_, _ = unary(first, context.Background(), nil, nil, nil, false)
		_, _ = unary(second, context.Background(), nil, nil, nil, false)
		handled := series(family(t, registry, "flam_grpc_server_handled_total"))
		testutil.Equal(t, "handled", handled["pkg.Users/Get/OK"].GetCounter().GetValue(), float64(2))
	})

	t.Run("should take the fields, then their defaults", func(t *testing.T) {
		registry, c := prometheus.NewRegistry(), newClock()
		i, err := measuring(t, metrics.NewBundle(), container.Bag{
			metrics.ConfigFieldNamespace: "app",
		}, map[string]any{metrics.DefaultBucketsConfigPath: []any{1, uint8(2), float32(3.5)}}, c, registry)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, false)
		duration := series(family(t, registry, "app_grpc_server_handling_seconds"))
		var bounds []float64
		for _, bucket := range duration["pkg.Users/Get/OK"].GetHistogram().GetBucket() {
			bounds = append(bounds, bucket.GetUpperBound())
		}
		testutil.Equal(t, "buckets", bounds, []float64{1, 2, 3.5})
	})

	t.Run("should use the built-in defaults when the defaults are missing", func(t *testing.T) {
		registry, c := prometheus.NewRegistry(), newClock()
		i, err := measuring(t, bare{metrics.NewBundle()}, container.Bag{}, nil, c, registry)
		testutil.NoError(t, err)
		_, _ = unary(i, context.Background(), nil, nil, nil, false)
		duration := series(family(t, registry, "flam_grpc_server_handling_seconds"))
		testutil.Equal(t, "buckets", len(duration["pkg.Users/Get/OK"].GetHistogram().GetBucket()), 11)
	})

	t.Run("should fail with an invalid field", func(t *testing.T) {
		for name, tc := range map[string]struct {
			field string
			value any
		}{
			"namespace":            {metrics.ConfigFieldNamespace, 1},
			"buckets type":         {metrics.ConfigFieldBuckets, "1"},
			"empty buckets":        {metrics.ConfigFieldBuckets, []float64{}},
			"empty any buckets":    {metrics.ConfigFieldBuckets, []any{}},
			"bucket type":          {metrics.ConfigFieldBuckets, []any{1, "2"}},
			"decreasing buckets":   {metrics.ConfigFieldBuckets, []float64{2, 1}},
			"repeated buckets":     {metrics.ConfigFieldBuckets, []any{1, 1.0}},
			"not a number buckets": {metrics.ConfigFieldBuckets, []float64{math.NaN()}},
		} {
			_, err := measuring(t, metrics.NewBundle(), container.Bag{tc.field: tc.value}, nil, newClock(), prometheus.NewRegistry())
			if !errors.Is(err, metrics.ErrInvalidField) {
				t.Fatalf("%s: Get() = %v, want ErrInvalidField", name, err)
			}
			testutil.Error(t, errors.Unwrap(err), metrics.ErrInvalidField,
				"unable to create metrics grpc interceptor: invalid metrics grpc interceptor field",
				map[string]any{metrics.ErrorFieldID: "main", metrics.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail without registerer in the container", func(t *testing.T) {
		_, err := measuring(t, metrics.NewBundle(), container.Bag{}, nil, newClock(), nil)
		testutil.Error(t, errors.Unwrap(err), metrics.ErrMissingRegisterer,
			"unable to create metrics grpc interceptor: metrics registerer not provided by the container",
			map[string]any{metrics.ErrorFieldID: "main"})
	})

	t.Run("should fail with an invalid namespace", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		_, err := measuring(t, metrics.NewBundle(), container.Bag{metrics.ConfigFieldNamespace: "\xff"}, nil, newClock(), registry)
		if !errors.Is(err, metrics.ErrRegister) || !strings.Contains(err.Error(), "is not a valid metric name") {
			t.Fatalf("Get() = %v, want ErrRegister of the invalid name", err)
		}
		families, _ := registry.Gather()
		testutil.Equal(t, "families", len(families), 0)
	})

	t.Run("should unregister its metrics when one conflicts", func(t *testing.T) {
		registry := prometheus.NewRegistry()
		testutil.NoError(t, registry.Register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flam_grpc_server_handling_seconds",
			Help: durationHelp,
		}, []string{metrics.LabelService, metrics.LabelMethod, metrics.LabelCode})))
		_, err := measuring(t, metrics.NewBundle(), container.Bag{}, nil, newClock(), registry)
		var already prometheus.AlreadyRegisteredError
		if !errors.Is(err, metrics.ErrRegister) || !errors.As(err, &already) {
			t.Fatalf("Get() = %v, want ErrRegister of the conflict", err)
		}
		var fe *flam.Error
		errors.As(errors.Unwrap(err), &fe)
		testutil.Context(t, fe, map[string]any{metrics.ErrorFieldID: "main"})
		testutil.NoError(t, registry.Register(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flam_grpc_server_handled_total",
			Help: handledHelp,
		}, []string{metrics.LabelService, metrics.LabelMethod, metrics.LabelCode})))
	})

	t.Run("should leave the configurations of other drivers", func(t *testing.T) {
		_, err := measuring(t, metrics.NewBundle(), container.Bag{factory.ConfigFieldDriver: "other"}, nil, newClock(), prometheus.NewRegistry())
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want factory.ErrNoCreator", err)
		}
	})
}
