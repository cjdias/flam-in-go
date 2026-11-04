// Package endpoints serves the probes of the
// [github.com/cjdias/flam-in-go/pkg/flam.IHealthRegistry] as REST
// endpoints: the bundle returned from [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IRestEndpointRegister]
// registering, on the servers of its group, the endpoint of the liveness
// probe and the one of the readiness probe, at the paths configured at
// [LivenessConfigPath] and [ReadinessConfigPath], "/health/live" and
// "/health/ready" unless set.
//
// The group is the one at [GroupConfigPath], "health" unless set, so that
// the probes are served by a server of their own, such as on a port not
// exposed to the clients of the application:
//
//	flam:
//	  rest:
//	    servers:
//	      api: {driver: flam.rest.servers.driver.standard, address: ":8080"}
//	      health: {driver: flam.rest.servers.driver.standard, address: ":8081"}
//
// or, to serve them along with the endpoints of the "api" server:
//
//	flam:
//	  health:
//	    endpoints:
//	      group: api
//
// An endpoint answers a GET, or a HEAD, request with the report of its
// probe, a [github.com/cjdias/flam-in-go/pkg/flam.HealthReport], as the
// data of the envelope written by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope], with a 200
// status when the probe succeeds. When it fails, it answers with an
// envelope error of the 503 status wrapping [ErrUnhealthy], written as an
// RFC 9457 problem, the report being its [ContextFieldReport] extension,
// such as:
//
//	{
//	  "type": "https://developer.mozilla.org/en-US/docs/Web/HTTP/Status/503",
//	  "title": "Service Unavailable",
//	  "status": 503,
//	  "detail": "health probe failed",
//	  "id": "e:2",
//	  "report": {
//	    "status": "down",
//	    "checks": [{"id": "main", "status": "down", "error": "...", "duration": "1.2ms"}]
//	  }
//	}
//
// As the errors of the checks are written in the reports, the probes
// should not be exposed to the clients of the application. The responses
// are not to be cached, as their Cache-Control header tells.
package endpoints
