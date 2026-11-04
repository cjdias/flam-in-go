// Package http is the health check driver of the HTTP services: each
// check requests a URL with GET, healthy when the status of the response
// is in its range, 2xx unless set, such as:
//
//	flam:
//	  health:
//	    checks:
//	      search: {driver: flam.health.checks.driver.http, url: "http://search:9200/_cluster/health"}
//	      payments: {driver: flam.health.checks.driver.http, client: payments, url: /health, timeout: 1s}
//	      legacy: {driver: flam.health.checks.driver.http, url: "https://legacy.example.com", max_status: 399}
//
// The check requests the URL, absolute, with a plain HTTP client, or,
// when its client field is set, with that client of the
// [github.com/cjdias/flam-in-go/pkg/flam.IClientFactory], taken from the
// factory, and so created, on its first run, the URL, such as a path,
// being then resolved against the base URL of the client, with its
// headers, retries and circuit breaker. It waits for the response up to
// its timeout, if any, within the timeout of the probe, and reads up to
// 64KiB of its body before closing it.
//
// The check counts for the readiness probe unless its kind is set, as a
// service that does not answer makes the application unable to serve,
// rather than broken.
package http
