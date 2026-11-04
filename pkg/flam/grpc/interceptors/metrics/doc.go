// Package metrics is the gRPC interceptor driver measuring the calls
// served by a server, unary or streaming, on the Prometheus
// [github.com/prometheus/client_golang/prometheus.Registerer] of the
// container, provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics].
//
// An interceptor registers two metrics, named after its namespace, "flam"
// unless set, such as "flam_grpc_server_handled_total":
//   - [MetricHandled], the counter of the calls served;
//   - [MetricDuration], the histogram of the time taken to serve them, in
//     seconds, measured with the
//     [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, of
//     the buckets of the interceptor.
//
// Each is labelled by the service of the call at [LabelService], such as
// "pkg.Users", its method at [LabelMethod], such as "Get", and the name of
// the code of its status at [LabelCode], such as "OK" or "NotFound", or
// [CodePanic] for a handler that panicked, whose panic then goes on. As
// the server only serves the methods of its services, the number of series
// stays bounded.
//
// The interceptors of the same namespace share their metrics, of the
// buckets of the first one created, so that their calls are measured
// together: configure another namespace for the interceptors whose metrics
// must be told apart.
package metrics
