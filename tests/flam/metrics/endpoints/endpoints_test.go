package endpoints

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics/endpoints"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// registers are the endpoint registers of the container.
type registers struct {
	dig.In

	Registers []flam.IRestEndpointRegister `group:"flam.rest.endpoints"`
}

// router is a flam.IRestRouter recording the endpoints it handles.
type router struct {
	endpoints []flam.RestEndpoint
}

func (r *router) Handle(endpoint flam.RestEndpoint, _ http.Handler) error {
	r.endpoints = append(r.endpoints, endpoint)
	return nil
}

// newRegister returns the metrics endpoint register of an app with the
// config and the endpoints bundles, the metrics bundle unless bare, and
// the configuration holding the values, along with the registerer of the
// app, nil when bare.
func newRegister(t *testing.T, values map[string]any, bare bool) (flam.IRestEndpointRegister, prometheus.Registerer) {
	t.Helper()
	app := testutil.NewApp()
	bundles := []flam.IBundle{config.NewBundle(), endpoints.NewBundle()}
	if !bare {
		bundles = append(bundles, metrics.NewBundle())
	}
	for _, b := range bundles {
		testutil.NoError(t, app.RegisterBundle(b))
	}
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(metrics.GoCollectorConfigPath, false))
	testutil.NoError(t, bag.Set(metrics.ProcessCollectorConfigPath, false))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))

	var register flam.IRestEndpointRegister
	testutil.NoError(t, app.Invoke(func(r registers) {
		testutil.Equal(t, "registers", len(r.Registers), 1)
		register = r.Registers[0]
	}))
	var registerer prometheus.Registerer
	if !bare {
		testutil.NoError(t, app.Invoke(func(r prometheus.Registerer) { registerer = r }))
	}
	return register, registerer
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"GroupConfigPath": {endpoints.GroupConfigPath, "flam.metrics.endpoint.group"},
		"PathConfigPath":  {endpoints.PathConfigPath, "flam.metrics.endpoint.path"},
		"IdConfigPath":    {endpoints.IdConfigPath, "flam.metrics.endpoint.id"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	if endpoints.ErrMissingGatherer == nil {
		t.Error("ErrMissingGatherer = nil")
	}
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the endpoint register provider", func(t *testing.T) {
		b := endpoints.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.metrics.endpoints")

		app := testutil.NewApp()
		testutil.NoError(t, app.RegisterBundle(b))
		testutil.Equal(t, "calls", app.Calls(), []string{"bundle:flam.metrics.endpoints", "provider/1"})
	})
}

func Test_Register(t *testing.T) {
	t.Run("should serve the metrics of the gatherer at /metrics in the metrics group", func(t *testing.T) {
		register, registerer := newRegister(t, nil, false)
		testutil.Equal(t, "Group()", register.Group(), "metrics")
		counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "app_hits_total", Help: "Hits."})
		counter.Add(2)
		testutil.NoError(t, registerer.Register(counter))

		recorded := &router{}
		testutil.NoError(t, register.Register(recorded))
		testutil.Equal(t, "endpoints", recorded.endpoints, []flam.RestEndpoint{{Id: 0, Pattern: "GET /metrics"}})

		r, err := rest.NewRouter(4)
		testutil.NoError(t, err)
		testutil.NoError(t, register.Register(r))
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
		testutil.Equal(t, "status", rec.Code, http.StatusOK)
		testutil.Equal(t, "content type", strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain"), true)
		body, err := io.ReadAll(rec.Body)
		testutil.NoError(t, err)
		testutil.Equal(t, "body", string(body), "# HELP app_hits_total Hits.\n# TYPE app_hits_total counter\napp_hits_total 2\n")

		rec = httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/metrics", nil))
		testutil.Equal(t, "post status", rec.Code, http.StatusMethodNotAllowed)
	})

	t.Run("should take the group, the path and the id of the configuration", func(t *testing.T) {
		register, _ := newRegister(t, map[string]any{
			endpoints.GroupConfigPath: "api",
			endpoints.PathConfigPath:  "/internal/metrics",
			endpoints.IdConfigPath:    9,
		}, false)
		testutil.Equal(t, "Group()", register.Group(), "api")
		recorded := &router{}
		testutil.NoError(t, register.Register(recorded))
		testutil.Equal(t, "endpoints", recorded.endpoints, []flam.RestEndpoint{{Id: 9, Pattern: "GET /internal/metrics"}})
	})

	t.Run("should fail on an invalid path", func(t *testing.T) {
		register, _ := newRegister(t, map[string]any{endpoints.PathConfigPath: "metrics"}, false)
		r, err := rest.NewRouter(4)
		testutil.NoError(t, err)
		err = register.Register(r)
		if !errors.Is(err, rest.ErrInvalidEndpoint) {
			t.Fatalf("error = %v, want it to wrap %v", err, rest.ErrInvalidEndpoint)
		}
	})

	t.Run("should fail without gatherer", func(t *testing.T) {
		register, _ := newRegister(t, nil, true)
		recorded := &router{}
		testutil.Error(t, register.Register(recorded), endpoints.ErrMissingGatherer,
			"unable to register metrics endpoint: metrics gatherer not provided by the container", map[string]any{})
		testutil.Equal(t, "endpoints", len(recorded.endpoints), 0)
	})
}
