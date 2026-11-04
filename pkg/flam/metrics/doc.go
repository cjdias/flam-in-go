// Package metrics is the metrics feature: the bundle returned from
// [NewBundle] provides the Prometheus registry of an application, as a
// [prometheus.Registerer] to register the collectors of the application
// on, and as a [prometheus.Gatherer] to expose them.
//
// The registry is a new one, not the default registry of the Prometheus
// client, so that the metrics of an application are only those it
// registers. It holds, unless disabled at [GoCollectorConfigPath] and
// [ProcessCollectorConfigPath], the collectors of the Go runtime and of
// the process, along with the collectors provided in the
// [CollectorGroup] container group, such as those of the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/database] and
// [github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/redis]:
//
//	flam:
//	  metrics:
//	    collectors:
//	      process: false
//
// The REST features of the metrics live in the sub packages, each with
// its own bundle, to be registered along with this one and the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/rest]:
// [github.com/cjdias/flam-in-go/pkg/flam/metrics/middleware] measures the
// requests served by the endpoints, and
// [github.com/cjdias/flam-in-go/pkg/flam/metrics/endpoints] serves the
// metrics to the Prometheus scrapes. They import this package for its
// paths and groups, so this package never imports them.
//
// [prometheus.Registerer]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Registerer
// [prometheus.Gatherer]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Gatherer
package metrics
