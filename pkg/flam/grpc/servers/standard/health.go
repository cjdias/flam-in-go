package standard

import (
	"context"
	"time"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// health is the standard gRPC health service of a server, whose services,
// the whole server included, under the empty name, are SERVING while the
// readiness probe of the registry succeeds, polled each interval while the
// server runs, or as long as the server runs without registry. Once the
// server stops, they are NOT_SERVING, and the Watch calls end, so that
// they do not hold the graceful stop of the server.
type health struct {
	*grpchealth.Server

	registry flam.IHealthRegistry
	clock    flam.IClock
	interval time.Duration
	services []string
	ctx      context.Context
	cancel   context.CancelFunc
}

// newHealth returns the health service of the registry, nil for none,
// probing its readiness each interval of the clock.
func newHealth(
	registry flam.IHealthRegistry,
	clock flam.IClock,
	interval time.Duration,
) *health {
	ctx, cancel := context.WithCancel(context.Background())
	return &health{
		Server:   grpchealth.NewServer(),
		registry: registry,
		clock:    clock,
		interval: interval,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Watch streams the status of the service, as the standard health service
// does, until the call ends or the server stops.
func (h *health) Watch(
	req *grpc_health_v1.HealthCheckRequest,
	stream grpc_health_v1.Health_WatchServer,
) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()
	stop := context.AfterFunc(h.ctx, cancel)
	defer stop()
	return h.Server.Watch(req, &healthStream{Health_WatchServer: stream, ctx: ctx})
}

// healthStream is the stream of a Watch call, of the context ending with
// the server.
type healthStream struct {
	grpc_health_v1.Health_WatchServer

	ctx context.Context
}

func (s *healthStream) Context() context.Context {
	return s.ctx
}

// init sets the services of the health service, the whole server and the
// services registered on it, SERVING without registry, and NOT_SERVING
// until the first probe of the registry otherwise.
func (h *health) init(
	services map[string]grpc.ServiceInfo,
) {
	h.services = []string{""}
	for service := range services {
		h.services = append(h.services, service)
	}
	status := grpc_health_v1.HealthCheckResponse_SERVING
	if h.registry != nil {
		status = grpc_health_v1.HealthCheckResponse_NOT_SERVING
	}
	h.set(status)
}

// start probes the readiness of the registry at once, then each interval,
// until the function it returns is called, which waits for the end of the
// probe under way. It probes nothing without registry.
func (h *health) start() func() {
	if h.registry == nil {
		return func() {}
	}
	done := make(chan struct{})
	ended := make(chan struct{})
	ticker := h.clock.NewTicker(h.interval)
	go func() {
		defer close(ended)
		defer ticker.Stop()
		for {
			h.probe()
			select {
			case <-done:
				return
			case <-ticker.C():
			}
		}
	}()
	return func() {
		close(done)
		<-ended
	}
}

// stop sets the services NOT_SERVING for good, and ends the Watch calls
// and the probe under way.
func (h *health) stop() {
	h.Shutdown()
	h.cancel()
}

// probe sets the status of the services from the readiness of the
// registry.
func (h *health) probe() {
	status := grpc_health_v1.HealthCheckResponse_NOT_SERVING
	if h.registry.Readiness(h.ctx).Healthy() {
		status = grpc_health_v1.HealthCheckResponse_SERVING
	}
	h.set(status)
}

// set sets the status of the services.
func (h *health) set(
	status grpc_health_v1.HealthCheckResponse_ServingStatus,
) {
	for _, service := range h.services {
		h.SetServingStatus(service, status)
	}
}
