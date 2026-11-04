package middleware

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics/middleware"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// epoch is the time of the fake clocks.
var epoch = time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

// fixture is an app with the config, rest, metrics and metrics middleware
// bundles, a fake clock, and the middlewares configured.
type fixture struct {
	app        *testutil.App
	clock      *testutil.Clock
	registerer prometheus.Registerer
	gatherer   prometheus.Gatherer
}

// newFixture returns the fixture of the middlewares of the configurations,
// by id, the configuration also holding the values, without the metrics
// bundle when bare.
func newFixture(t *testing.T, middlewares map[string]container.Bag, values map[string]any, bare bool) *fixture {
	t.Helper()
	bundles := []flam.IBundle{config.NewBundle(), rest.NewBundle(), middleware.NewBundle()}
	if !bare {
		bundles = append(bundles, metrics.NewBundle())
	}
	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	testutil.NoError(t, bag.Set(metrics.GoCollectorConfigPath, false))
	testutil.NoError(t, bag.Set(metrics.ProcessCollectorConfigPath, false))
	for id, fields := range middlewares {
		if !fields.Has(factory.ConfigFieldDriver) {
			fields[factory.ConfigFieldDriver] = middleware.Driver
		}
		testutil.NoError(t, bag.Set(rest.MiddlewareConfigPath+"."+id, fields))
	}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}

	f := &fixture{app: testutil.NewApp(), clock: testutil.NewClock(epoch)}
	testutil.NoError(t, f.app.Provide(func() flam.IClock { return f.clock }))
	for _, b := range bundles {
		testutil.NoError(t, f.app.RegisterBundle(b))
	}
	testutil.NoError(t, f.app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	if !bare {
		testutil.NoError(t, f.app.Invoke(func(r prometheus.Registerer, g prometheus.Gatherer) {
			f.registerer, f.gatherer = r, g
		}))
	}
	return f
}

// get returns the middleware of the id, or the error of its creation.
func (f *fixture) get(t *testing.T, id string) (flam.IRestMiddleware, error) {
	t.Helper()
	var m flam.IRestMiddleware
	var err error
	testutil.NoError(t, f.app.Invoke(func(factory flam.IRestMiddlewareFactory) { m, err = factory.Get(id) }))
	return m, err
}

// must returns the middleware of the id, failing the test when it cannot
// be created.
func (f *fixture) must(t *testing.T, id string) flam.IRestMiddleware {
	t.Helper()
	m, err := f.get(t, id)
	testutil.NoError(t, err)
	return m
}

// value returns the value of the metric of the name and the labels, the
// sample count of a histogram, or -1 when it is not gathered.
func (f *fixture) value(t *testing.T, name string, labels map[string]string) float64 {
	t.Helper()
	families, err := f.gatherer.Gather()
	testutil.NoError(t, err)
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, m := range family.GetMetric() {
			got := map[string]string{}
			for _, pair := range m.GetLabel() {
				got[pair.GetName()] = pair.GetValue()
			}
			if len(got) != len(labels) {
				continue
			}
			match := true
			for k, v := range labels {
				match = match && got[k] == v
			}
			if !match {
				continue
			}
			switch {
			case m.GetCounter() != nil:
				return m.GetCounter().GetValue()
			case m.GetGauge() != nil:
				return m.GetGauge().GetValue()
			default:
				return float64(m.GetHistogram().GetSampleCount())
			}
		}
	}
	return -1
}

// labels returns the labels of a request served by the endpoint 7 of the
// service 3, of the pattern "GET /items/{id}", with the method and the
// code, or of the in flight gauge when both are empty.
func labels(method, code string) map[string]string {
	l := map[string]string{
		middleware.LabelService:  "3",
		middleware.LabelEndpoint: "7",
		middleware.LabelPattern:  "GET /items/{id}",
	}
	if method != "" || code != "" {
		l[middleware.LabelMethod] = method
		l[middleware.LabelCode] = code
	}
	return l
}

// serve serves a request of the method and the target with a router of
// the service 3 holding the endpoint 7 of the pattern, wrapped in the
// middlewares, returning the recorded response and the value its serving
// panicked with, if any.
func serve(t *testing.T, method, target, pattern string, handler http.HandlerFunc, mws ...flam.IRestMiddleware) (rec *httptest.ResponseRecorder, panicked any) {
	t.Helper()
	router, err := rest.NewRouter(3, mws...)
	testutil.NoError(t, err)
	testutil.NoError(t, router.Handle(flam.RestEndpoint{Id: 7, Pattern: pattern}, handler))
	rec = httptest.NewRecorder()
	defer func() { panicked = recover() }()
	router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec, nil
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"Id":                         {middleware.Id, "metrics"},
		"Driver":                     {middleware.Driver, "flam.rest.middlewares.driver.metrics"},
		"ConfigFieldNamespace":       {middleware.ConfigFieldNamespace, "namespace"},
		"ConfigFieldBuckets":         {middleware.ConfigFieldBuckets, "buckets"},
		"ConfigFieldCodeClass":       {middleware.ConfigFieldCodeClass, "code_class"},
		"DefaultNamespaceConfigPath": {middleware.DefaultNamespaceConfigPath, "flam.rest.defaults.middlewares.metrics.namespace"},
		"DefaultBucketsConfigPath":   {middleware.DefaultBucketsConfigPath, "flam.rest.defaults.middlewares.metrics.buckets"},
		"DefaultCodeClassConfigPath": {middleware.DefaultCodeClassConfigPath, "flam.rest.defaults.middlewares.metrics.code_class"},
		"MetricRequests":             {middleware.MetricRequests, "requests_total"},
		"MetricDuration":             {middleware.MetricDuration, "request_duration_seconds"},
		"MetricInFlight":             {middleware.MetricInFlight, "requests_in_flight"},
		"Subsystem":                  {middleware.Subsystem, "rest"},
		"LabelService":               {middleware.LabelService, "service"},
		"LabelEndpoint":              {middleware.LabelEndpoint, "endpoint"},
		"LabelPattern":               {middleware.LabelPattern, "pattern"},
		"LabelMethod":                {middleware.LabelMethod, "method"},
		"LabelCode":                  {middleware.LabelCode, "code"},
		"MethodOther":                {middleware.MethodOther, "OTHER"},
		"CodePanic":                  {middleware.CodePanic, "panic"},
		"ErrorFieldID":               {middleware.ErrorFieldID, "id"},
		"ErrorFieldField":            {middleware.ErrorFieldField, "field"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"ErrInvalidField":      middleware.ErrInvalidField,
		"ErrMissingRegisterer": middleware.ErrMissingRegisterer,
		"ErrRegister":          middleware.ErrRegister,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the creator", func(t *testing.T) {
		b := middleware.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.metrics.middleware")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.metrics.middleware", "provider/1"})
	})

	t.Run("should configure the middleware and the defaults of its fields", func(t *testing.T) {
		b := middleware.NewBundle()
		bag := container.Bag{}
		testutil.NoError(t, b.(flam.IConfigurableBundle).Config(bag))
		for path, want := range map[string]any{
			rest.MiddlewareConfigPath + "." + middleware.Id: container.Bag{factory.ConfigFieldDriver: middleware.Driver},
			middleware.DefaultNamespaceConfigPath:           "flam",
			middleware.DefaultBucketsConfigPath:             []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
			middleware.DefaultCodeClassConfigPath:           false,
		} {
			testutil.Equal(t, path, bag.Get(path), want)
		}
		if err := b.(flam.IConfigurableBundle).Config(container.Bag{"flam": "scalar"}); err == nil {
			t.Fatal("Config() did not fail")
		}
	})
}

func Test_Middleware(t *testing.T) {
	t.Run("should measure the requests of the endpoint", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		testutil.Equal(t, "Id()", m.Id(), "metrics")

		inFlight := -1.0
		rec, panicked := serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
			inFlight = f.value(t, "flam_rest_requests_in_flight", labels("", ""))
			f.clock.Advance(1500 * time.Millisecond)
			w.WriteHeader(http.StatusCreated)
		}, m)
		testutil.Equal(t, "panicked", panicked, nil)
		testutil.Equal(t, "status", rec.Code, http.StatusCreated)
		testutil.Equal(t, "in flight while served", inFlight, 1.0)
		testutil.Equal(t, "in flight", f.value(t, "flam_rest_requests_in_flight", labels("", "")), 0.0)
		testutil.Equal(t, "requests", f.value(t, "flam_rest_requests_total", labels("GET", "201")), 1.0)
		testutil.Equal(t, "durations", f.value(t, "flam_rest_request_duration_seconds", labels("GET", "201")), 1.0)

		err := promtestutil.GatherAndCompare(f.gatherer, strings.NewReader(`
# HELP flam_rest_request_duration_seconds Time taken to serve the REST requests, in seconds, by endpoint, method and status.
# TYPE flam_rest_request_duration_seconds histogram
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.005"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.01"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.025"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.05"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.1"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.25"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="0.5"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="1"} 0
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="2.5"} 1
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="5"} 1
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="10"} 1
flam_rest_request_duration_seconds_bucket{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3",le="+Inf"} 1
flam_rest_request_duration_seconds_sum{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3"} 1.5
flam_rest_request_duration_seconds_count{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3"} 1
# HELP flam_rest_requests_total Number of REST requests served, by endpoint, method and status.
# TYPE flam_rest_requests_total counter
flam_rest_requests_total{code="201",endpoint="7",method="GET",pattern="GET /items/{id}",service="3"} 1
`), "flam_rest_request_duration_seconds", "flam_rest_requests_total")
		testutil.NoError(t, err)
	})

	t.Run("should expose the in flight gauge of every endpoint before its first request", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		_, err := rest.NewRouter(3, m)
		testutil.NoError(t, err)
		testutil.Equal(t, "unmatched in flight", f.value(t, "flam_rest_requests_in_flight", map[string]string{
			middleware.LabelService: "3", middleware.LabelEndpoint: "0", middleware.LabelPattern: "",
		}), 0.0)
		testutil.Equal(t, "requests", f.value(t, "flam_rest_requests_total", labels("GET", "200")), -1.0)
	})

	t.Run("should label the status 200 of a handler writing none", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {}, m)
		testutil.Equal(t, "requests", f.value(t, "flam_rest_requests_total", labels("GET", "200")), 1.0)
	})

	t.Run("should measure the requests matching no pattern under the empty pattern", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		rec, _ := serve(t, http.MethodGet, "/unknown/path", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {}, m)
		testutil.Equal(t, "status", rec.Code, http.StatusNotFound)
		rec, _ = serve(t, http.MethodDelete, "/items/1", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {}, m)
		testutil.Equal(t, "status", rec.Code, http.StatusMethodNotAllowed)
		unmatched := func(method, code string) map[string]string {
			return map[string]string{
				middleware.LabelService: "3", middleware.LabelEndpoint: "0", middleware.LabelPattern: "",
				middleware.LabelMethod: method, middleware.LabelCode: code,
			}
		}
		testutil.Equal(t, "not found", f.value(t, "flam_rest_requests_total", unmatched("GET", "404")), 1.0)
		testutil.Equal(t, "not allowed", f.value(t, "flam_rest_requests_total", unmatched("DELETE", "405")), 1.0)
	})

	t.Run("should label the methods other than the standard ones as other", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		for _, method := range []string{"PURGE", "get", http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace} {
			serve(t, method, "/items/1", "/items/{id}", func(http.ResponseWriter, *http.Request) {}, m)
		}
		any := func(method string) map[string]string {
			return map[string]string{
				middleware.LabelService: "3", middleware.LabelEndpoint: "7", middleware.LabelPattern: "/items/{id}",
				middleware.LabelMethod: method, middleware.LabelCode: "200",
			}
		}
		testutil.Equal(t, "other", f.value(t, "flam_rest_requests_total", any("OTHER")), 2.0)
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
			http.MethodPatch, http.MethodDelete, http.MethodOptions, http.MethodTrace} {
			testutil.Equal(t, method, f.value(t, "flam_rest_requests_total", any(method)), 1.0)
		}

		h := m.Wrap(flam.RestEndpoint{ServiceId: 3, Id: 7, Pattern: "/items/{id}"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		req := httptest.NewRequest(http.MethodGet, "/items/1", nil)
		req.Method = http.MethodConnect
		h.ServeHTTP(httptest.NewRecorder(), req)
		testutil.Equal(t, "connect", f.value(t, "flam_rest_requests_total", any(http.MethodConnect)), 1.0)
	})

	t.Run("should label the classes of the statuses", func(t *testing.T) {
		f := newFixture(t, map[string]container.Bag{"main": {middleware.ConfigFieldCodeClass: true}}, nil, false)
		m := f.must(t, "main")
		for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusNotFound, http.StatusServiceUnavailable} {
			serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }, m)
		}
		testutil.Equal(t, "2xx", f.value(t, "flam_rest_requests_total", labels("GET", "2xx")), 2.0)
		testutil.Equal(t, "4xx", f.value(t, "flam_rest_requests_total", labels("GET", "4xx")), 1.0)
		testutil.Equal(t, "5xx", f.value(t, "flam_rest_requests_total", labels("GET", "5xx")), 1.0)
	})

	t.Run("should label the panics before a response and let them go on", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		m := f.must(t, middleware.Id)
		_, panicked := serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {
			f.clock.Advance(time.Second)
			panic(errBoom)
		}, m)
		testutil.Equal(t, "panicked", panicked, any(errBoom))
		testutil.Equal(t, "requests", f.value(t, "flam_rest_requests_total", labels("GET", "panic")), 1.0)
		testutil.Equal(t, "durations", f.value(t, "flam_rest_request_duration_seconds", labels("GET", "panic")), 1.0)
		testutil.Equal(t, "in flight", f.value(t, "flam_rest_requests_in_flight", labels("", "")), 0.0)

		_, panicked = serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
			panic(errBoom)
		}, m)
		testutil.Equal(t, "panicked", panicked, any(errBoom))
		testutil.Equal(t, "written", f.value(t, "flam_rest_requests_total", labels("GET", "202")), 1.0)
	})

	t.Run("should share the metrics of the middlewares of the same namespace", func(t *testing.T) {
		f := newFixture(t, map[string]container.Bag{
			"a":     {},
			"b":     {middleware.ConfigFieldBuckets: []any{1, 2}},
			"other": {middleware.ConfigFieldNamespace: "other"},
		}, nil, false)
		a, b, other := f.must(t, "a"), f.must(t, "b"), f.must(t, "other")
		for _, m := range []flam.IRestMiddleware{a, b, other} {
			serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {}, m)
		}
		testutil.Equal(t, "shared", f.value(t, "flam_rest_requests_total", labels("GET", "200")), 2.0)
		testutil.Equal(t, "other", f.value(t, "other_rest_requests_total", labels("GET", "200")), 1.0)
		count, err := promtestutil.GatherAndCount(f.gatherer, "flam_rest_request_duration_seconds")
		testutil.NoError(t, err)
		testutil.Equal(t, "histograms", count, 1)
	})

	t.Run("should take the buckets of the configuration", func(t *testing.T) {
		for name, buckets := range map[string]any{
			"floats":  []float64{0.5, 2},
			"numbers": []any{int8(0), uint(1), float32(1.5), 2},
		} {
			f := newFixture(t, nil, map[string]any{
				middleware.DefaultBucketsConfigPath:   buckets,
				middleware.DefaultNamespaceConfigPath: "",
			}, false)
			m := f.must(t, middleware.Id)
			serve(t, http.MethodGet, "/items/1", "GET /items/{id}", func(http.ResponseWriter, *http.Request) {
				f.clock.Advance(time.Second)
			}, m)
			families, err := f.gatherer.Gather()
			testutil.NoError(t, err)
			for _, family := range families {
				if family.GetName() != "rest_request_duration_seconds" {
					continue
				}
				var bounds []float64
				for _, bucket := range family.GetMetric()[0].GetHistogram().GetBucket() {
					bounds = append(bounds, bucket.GetUpperBound())
				}
				switch name {
				case "floats":
					testutil.Equal(t, name, bounds, []float64{0.5, 2})
				default:
					testutil.Equal(t, name, bounds, []float64{0, 1, 1.5, 2})
				}
			}
			testutil.Equal(t, name+" requests", f.value(t, "rest_requests_total", labels("GET", "200")), 1.0)
		}
	})
}

func Test_Creator(t *testing.T) {
	t.Run("should fail on an invalid field", func(t *testing.T) {
		for _, tc := range []struct {
			field string
			value any
		}{
			{middleware.ConfigFieldNamespace, 1},
			{middleware.ConfigFieldBuckets, "1,2"},
			{middleware.ConfigFieldBuckets, []any{}},
			{middleware.ConfigFieldBuckets, []float64{}},
			{middleware.ConfigFieldBuckets, []any{1, "2"}},
			{middleware.ConfigFieldBuckets, []any{2, 1}},
			{middleware.ConfigFieldBuckets, []any{1, 1}},
			{middleware.ConfigFieldBuckets, []float64{math.NaN()}},
			{middleware.ConfigFieldCodeClass, "yes"},
		} {
			f := newFixture(t, map[string]container.Bag{"main": {tc.field: tc.value}}, nil, false)
			m, err := f.get(t, "main")
			testutil.Equal(t, tc.field+" middleware", m, nil)
			testutil.Error(t, errors.Unwrap(err), middleware.ErrInvalidField,
				"unable to create metrics rest middleware: invalid metrics rest middleware field",
				map[string]any{middleware.ErrorFieldID: "main", middleware.ErrorFieldField: tc.field})
		}
	})

	t.Run("should fail without registerer", func(t *testing.T) {
		f := newFixture(t, nil, nil, true)
		_, err := f.get(t, middleware.Id)
		testutil.Error(t, errors.Unwrap(err), middleware.ErrMissingRegisterer,
			"unable to create metrics rest middleware: metrics registerer not provided by the container",
			map[string]any{middleware.ErrorFieldID: "metrics"})
	})

	t.Run("should fail on an invalid namespace", func(t *testing.T) {
		f := newFixture(t, map[string]container.Bag{"main": {middleware.ConfigFieldNamespace: "\xff"}}, nil, false)
		_, err := f.get(t, "main")
		var e *flam.Error
		if !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("error = %v, want a *flam.Error", err)
		}
		if !errors.Is(e, middleware.ErrRegister) {
			t.Fatalf("error = %v, want it to wrap %v", e, middleware.ErrRegister)
		}
		testutil.Context(t, e, map[string]any{middleware.ErrorFieldID: "main"})
	})

	t.Run("should fail on registered metrics of another type", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		testutil.NoError(t, f.registerer.Register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flam_rest_requests_total",
			Help: "Number of REST requests served, by endpoint, method and status.",
		}, []string{"service", "endpoint", "pattern", "method", "code"})))
		_, err := f.get(t, middleware.Id)
		var e *flam.Error
		if !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("error = %v, want a *flam.Error", err)
		}
		if !errors.Is(e, middleware.ErrRegister) {
			t.Fatalf("error = %v, want it to wrap %v", e, middleware.ErrRegister)
		}
		var already prometheus.AlreadyRegisteredError
		testutil.Equal(t, "already registered", errors.As(e, &already), true)
		testutil.Context(t, e, map[string]any{middleware.ErrorFieldID: "metrics"})
	})

	t.Run("should unregister its metrics when one cannot be registered", func(t *testing.T) {
		f := newFixture(t, nil, nil, false)
		testutil.NoError(t, f.registerer.Register(prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "flam_rest_requests_in_flight",
			Help: "Conflicting.",
		}, []string{"other"})))
		_, err := f.get(t, middleware.Id)
		var e *flam.Error
		if !errors.As(errors.Unwrap(err), &e) {
			t.Fatalf("error = %v, want a *flam.Error", err)
		}
		if !errors.Is(e, middleware.ErrRegister) {
			t.Fatalf("error = %v, want it to wrap %v", e, middleware.ErrRegister)
		}
		labels := []string{"service", "endpoint", "pattern", "method", "code"}
		testutil.NoError(t, f.registerer.Register(prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "flam_rest_requests_total",
			Help: "Number of REST requests served, by endpoint, method and status.",
		}, labels)))
		testutil.NoError(t, f.registerer.Register(prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "flam_rest_request_duration_seconds",
			Help: "Time taken to serve the REST requests, in seconds, by endpoint, method and status.",
		}, labels)))
	})
}
