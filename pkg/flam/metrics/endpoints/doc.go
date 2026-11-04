// Package endpoints serves the metrics of the Prometheus
// [github.com/prometheus/client_golang/prometheus.Gatherer] of the
// container, provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics], to the Prometheus
// scrapes, as an endpoint of the REST servers of its group, "metrics"
// unless set at [GroupConfigPath], at the path "/metrics" unless set at
// [PathConfigPath].
//
// As the servers serve the endpoints of their group, the server group
// defaulting to the server id, a server of id "metrics" serves it, such
// as on a port of its own:
//
//	flam:
//	  rest:
//	    servers:
//	      metrics: {driver: flam.rest.servers.driver.standard, address: ":9090"}
//
// while setting the group of the endpoint to the group of another server
// serves it along with the endpoints of the application:
//
//	flam:
//	  metrics:
//	    endpoint:
//	      group: api
package endpoints
