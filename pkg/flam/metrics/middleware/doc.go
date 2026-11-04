// Package middleware is the REST middleware driver measuring the requests
// served by the endpoints, on the Prometheus
// [github.com/prometheus/client_golang/prometheus.Registerer] of the
// container, provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics].
//
// A middleware registers three metrics, named after its namespace, "flam"
// unless set, such as "flam_rest_requests_total":
//   - [MetricRequests], the counter of the requests served;
//   - [MetricDuration], the histogram of the time taken to serve them, in
//     seconds, measured with the
//     [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, of
//     the buckets of the middleware;
//   - [MetricInFlight], the gauge of the requests being served.
//
// Each is labelled by the endpoint serving the request: the id of its
// service at [LabelService], its id at [LabelEndpoint] and its pattern at
// [LabelPattern], such as "GET /users/{id}", never the path of the
// request, so that the number of series stays bounded. The requests
// matching no pattern are measured under the endpoint of id 0 and the
// empty pattern. The counter and the histogram are also labelled by the
// method of the request at [LabelMethod], [MethodOther] for a method
// other than the standard ones, and by the status of the response at
// [LabelCode], such as "404", or its class, such as "4xx", with
// [ConfigFieldCodeClass], or [CodePanic] for a handler that panicked
// before writing a response, whose panic then goes on.
//
// The middlewares of the same namespace share their metrics, of the
// buckets of the first one created, so that their requests are measured
// together: configure another namespace for the middlewares whose metrics
// must be told apart.
// List the middleware first among the middlewares of a server to measure
// the time they take too, after the envelope middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope] to
// measure the panics it answers as 500 responses:
//
//	flam:
//	  rest:
//	    defaults:
//	      servers:
//	        middlewares: [envelope, metrics, log]
package middleware
