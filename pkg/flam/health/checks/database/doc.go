// Package database is the health check driver of the database
// connections: each check pings a connection of the
// [github.com/cjdias/flam-in-go/pkg/flam.IDatabaseConnectionFactory],
// taking it from the factory, and so opening it, on its first run, such
// as:
//
//	flam:
//	  health:
//	    checks:
//	      main: {driver: flam.health.checks.driver.database}
//	      replica: {driver: flam.health.checks.driver.database, connection: read, kind: both}
//
// The check counts for the readiness probe unless its kind is set, as a
// database that does not answer makes the application unable to serve,
// rather than broken.
package database
