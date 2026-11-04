package endpoints

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/health/endpoints"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// newApp returns a testutil.App with the config, health and endpoints
// bundles, a fake clock, and a configuration holding the values, at paths.
func newApp(t *testing.T, values map[string]any) *testutil.App {
	t.Helper()
	bag := container.Bag{}
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	a := testutil.NewApp()
	clock := testutil.NewClock(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	testutil.NoError(t, a.Provide(func() flam.IClock { return clock }))
	for _, b := range []flam.IBundle{config.NewBundle(), health.NewBundle(), endpoints.NewBundle()} {
		testutil.NoError(t, a.RegisterBundle(b))
	}
	testutil.NoError(t, a.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	return a
}

// registersArgs are the registers of the endpoint group.
type registersArgs struct {
	dig.In

	Registers []flam.IRestEndpointRegister `group:"flam.rest.endpoints"`
}

// registerOf returns the single register of the endpoint group of the app,
// or the error of the container.
func registerOf(a flam.IApp) (flam.IRestEndpointRegister, error) {
	var register flam.IRestEndpointRegister
	err := a.Invoke(func(args registersArgs) { register = args.Registers[0] })
	return register, err
}

// routerOf returns the router of the service 7 holding the endpoints of
// the register of the app, whose registry holds the checks.
func routerOf(t *testing.T, a flam.IApp, checks ...flam.IHealthCheck) *rest.Router {
	t.Helper()
	register, err := registerOf(a)
	testutil.NoError(t, err)
	testutil.NoError(t, a.Invoke(func(r flam.IHealthRegistry) error {
		for _, check := range checks {
			if err := r.Add(check); err != nil {
				return err
			}
		}
		return nil
	}))
	router, err := rest.NewRouter(7)
	testutil.NoError(t, err)
	testutil.NoError(t, register.Register(router))
	return router
}

// mustCheck returns the check of NewCheck, failing with err.
func mustCheck(t *testing.T, id string, kind flam.HealthKind, err error) flam.IHealthCheck {
	t.Helper()
	check, e := health.NewCheck(id, kind, func(context.Context) error { return err })
	testutil.NoError(t, e)
	return check
}

// serve serves the request of the method and target with the handler.
func serve(h http.Handler, method, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

// body returns the JSON body of the response, decoded.
func body(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var decoded map[string]any
	testutil.NoError(t, json.Unmarshal(rec.Body.Bytes(), &decoded))
	return decoded
}

func Test_Declarations(t *testing.T) {
	testutil.Equal(t, "GroupConfigPath", endpoints.GroupConfigPath, "flam.health.endpoints.group")
	testutil.Equal(t, "LivenessConfigPath", endpoints.LivenessConfigPath, "flam.health.endpoints.liveness")
	testutil.Equal(t, "ReadinessConfigPath", endpoints.ReadinessConfigPath, "flam.health.endpoints.readiness")
	testutil.Equal(t, "LivenessEndpointId", endpoints.LivenessEndpointId, 1)
	testutil.Equal(t, "ReadinessEndpointId", endpoints.ReadinessEndpointId, 2)
	testutil.Equal(t, "ContextFieldReport", endpoints.ContextFieldReport, "report")
	testutil.Equal(t, "ErrorFieldField", endpoints.ErrorFieldField, "field")
}

func Test_Bundle(t *testing.T) {
	b := endpoints.NewBundle()
	testutil.Equal(t, "Id()", b.Id(), "flam.health.endpoints")

	a := newApp(t, nil)
	testutil.Equal(t, "calls", a.Calls()[len(a.Calls())-3:], []string{
		"bundle:flam.health.endpoints", "provider/1", "invoker/0",
	})
	register, err := registerOf(a)
	testutil.NoError(t, err)
	testutil.Equal(t, "Group()", register.Group(), "health")
}

func Test_Register_Config(t *testing.T) {
	t.Run("should take the group and the paths of the configuration", func(t *testing.T) {
		a := newApp(t, map[string]any{
			endpoints.GroupConfigPath:     "api",
			endpoints.LivenessConfigPath:  "/livez",
			endpoints.ReadinessConfigPath: "/readyz",
		})
		router := routerOf(t, a)
		register, _ := registerOf(a)
		testutil.Equal(t, "Group()", register.Group(), "api")

		testutil.Equal(t, "live", serve(router, http.MethodGet, "/livez").Code, http.StatusOK)
		testutil.Equal(t, "ready", serve(router, http.MethodGet, "/readyz").Code, http.StatusOK)
		testutil.Equal(t, "default", serve(router, http.MethodGet, "/health/live").Code, http.StatusNotFound)
	})

	t.Run("should refuse a value that is not a non-empty string", func(t *testing.T) {
		for _, path := range []string{endpoints.GroupConfigPath, endpoints.LivenessConfigPath, endpoints.ReadinessConfigPath} {
			for _, value := range []any{"", 3} {
				_, err := registerOf(newApp(t, map[string]any{path: value}))
				if !errors.Is(err, endpoints.ErrInvalidField) {
					t.Fatalf("Invoke() = %v, want endpoints.ErrInvalidField", err)
				}
				var e *flam.Error
				errors.As(err, &e)
				testutil.Equal(t, "Error()", e.Error(), "unable to create health endpoints: invalid health endpoints field")
				testutil.Context(t, e, map[string]any{endpoints.ErrorFieldField: path})
			}
		}
	})

	t.Run("should fail to register an invalid or conflicting path", func(t *testing.T) {
		for _, values := range []map[string]any{
			{endpoints.LivenessConfigPath: "live"},
			{endpoints.ReadinessConfigPath: "/health/live"},
		} {
			register, err := registerOf(newApp(t, values))
			testutil.NoError(t, err)
			router, err := rest.NewRouter(0)
			testutil.NoError(t, err)
			if err := register.Register(router); !errors.Is(err, rest.ErrInvalidEndpoint) {
				t.Fatalf("Register() = %v, want rest.ErrInvalidEndpoint", err)
			}
		}
	})
}

func Test_Register_Serve(t *testing.T) {
	t.Run("should answer with the report of a probe that succeeds", func(t *testing.T) {
		router := routerOf(t, newApp(t, nil),
			mustCheck(t, "db", flam.HealthReadiness, nil),
			mustCheck(t, "loop", flam.HealthLiveness, errBoom))

		rec := serve(router, http.MethodGet, "/health/ready")
		testutil.Equal(t, "Code", rec.Code, http.StatusOK)
		testutil.Equal(t, "Content-Type", rec.Header().Get("Content-Type"), "application/json")
		testutil.Equal(t, "Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
		testutil.Equal(t, "body", body(t, rec), map[string]any{
			"data": map[string]any{
				"status": "up",
				"checks": []any{map[string]any{"id": "db", "status": "up", "duration": "0s"}},
			},
		})
	})

	t.Run("should answer with a problem holding the report of a probe that fails", func(t *testing.T) {
		router := routerOf(t, newApp(t, nil),
			mustCheck(t, "db", flam.HealthReadiness, nil),
			mustCheck(t, "loop", flam.HealthLiveness, errBoom))

		rec := serve(router, http.MethodGet, "/health/live")
		testutil.Equal(t, "Code", rec.Code, http.StatusServiceUnavailable)
		testutil.Equal(t, "Content-Type", rec.Header().Get("Content-Type"), envelope.ContentTypeProblem)
		testutil.Equal(t, "Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
		testutil.Equal(t, "body", body(t, rec), map[string]any{
			"type":   envelope.ProblemTypeBase + "503",
			"title":  "Service Unavailable",
			"status": float64(503),
			"detail": "health probe failed",
			"id":     "s:7.e:1",
			"report": map[string]any{
				"status": "down",
				"checks": []any{map[string]any{"id": "loop", "status": "down", "error": "boom", "duration": "0s"}},
			},
		})
	})

	t.Run("should answer a HEAD request, and refuse other methods", func(t *testing.T) {
		router := routerOf(t, newApp(t, nil), mustCheck(t, "db", flam.HealthReadiness, errBoom))

		rec := serve(router, http.MethodHead, "/health/ready")
		testutil.Equal(t, "Code", rec.Code, http.StatusServiceUnavailable)
		testutil.Equal(t, "POST", serve(router, http.MethodPost, "/health/ready").Code, http.StatusMethodNotAllowed)
	})
}
