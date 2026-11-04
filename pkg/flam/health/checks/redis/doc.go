// Package redis is the health check driver of the Redis connections: each
// check sends a PING with a connection of the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], taking
// it from the factory, and so creating it, on its first run, such as:
//
//	flam:
//	  health:
//	    checks:
//	      main: {driver: flam.health.checks.driver.redis}
//	      sessions: {driver: flam.health.checks.driver.redis, connection: cache, kind: both}
//
// The check counts for the readiness probe unless its kind is set, as a
// server that does not answer makes the application unable to serve,
// rather than broken.
package redis
