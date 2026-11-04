package endpoints

import (
	"context"
	"net/http"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
	"github.com/cjdias/flam-in-go/pkg/flam/rest"
)

// newRegister returns the register of the endpoints of the probes of the
// registry, for the group and at the paths of the configuration, or their
// defaults, failing with ErrInvalidField for a value that is not a
// non-empty string. It is the provider registered by the bundle returned
// from [NewBundle].
func newRegister(
	config flam.IConfig,
	registry flam.IHealthRegistry,
) (flam.IRestEndpointRegister, error) {
	r := &register{registry: registry}
	for _, field := range []struct {
		path   string
		def    string
		target *string
	}{
		{GroupConfigPath, defaultGroup, &r.group},
		{LivenessConfigPath, defaultLiveness, &r.liveness},
		{ReadinessConfigPath, defaultReadiness, &r.readiness},
	} {
		s, ok := config.Get(field.path, field.def).(string)
		if !ok || s == "" {
			return nil, flam.NewWrappedError(ErrInvalidField, registerNewMessage).
				With(ErrorFieldField, field.path)
		}
		*field.target = s
	}
	return r, nil
}

// register is the [flam.IRestEndpointRegister] of the endpoints of the
// probes.
type register struct {
	registry  flam.IHealthRegistry
	group     string
	liveness  string
	readiness string
}

// compile time check that *register implements IRestEndpointRegister
var _ flam.IRestEndpointRegister = (*register)(nil)

func (r *register) Group() string {
	return r.group
}

// Register registers the endpoint of the liveness probe, then the one of
// the readiness probe, failing with the error of the router.
func (r *register) Register(
	router flam.IRestRouter,
) error {
	if err := router.Handle(
		flam.RestEndpoint{Id: LivenessEndpointId, Pattern: http.MethodGet + " " + r.liveness},
		serve(r.registry.Liveness),
	); err != nil {
		return err
	}
	return router.Handle(
		flam.RestEndpoint{Id: ReadinessEndpointId, Pattern: http.MethodGet + " " + r.readiness},
		serve(r.registry.Readiness),
	)
}

// registerNewMessage is the message of the errors returned by
// newRegister.
const registerNewMessage = "unable to create health endpoints"

// serve returns the handler answering with the report of the probe, as
// the data of the envelope when the probe succeeds, or else as the
// ContextFieldReport of its error, of the 503 status.
func serve(
	probe func(ctx context.Context) flam.HealthReport,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		report := probe(req.Context())
		env := envelope.NewEnvelope(report)
		if !report.Healthy() {
			env = envelope.NewEnvelope().WithNewError(http.StatusServiceUnavailable, ErrUnhealthy,
				map[string]any{ContextFieldReport: report})
		}
		w.Header().Set("Cache-Control", "no-store")
		_ = rest.WriteEnvelope(w, req, env)
	})
}
