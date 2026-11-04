// Package kennel is the health check driver of the processes of the
// [github.com/cjdias/flam-in-go/pkg/flam.IKennel]: each check fails when
// one of the processes it watches, active in the kennel, is not running,
// such as a server that ended on a failure, such as:
//
//	flam:
//	  health:
//	    checks:
//	      servers: {driver: flam.health.checks.driver.kennel}
//	      api: {driver: flam.health.checks.driver.kennel, processes: [api]}
//
// The check watches the processes of the kennel, all of them unless its
// processes field lists some, and passes while the kennel does not run
// them, such as before the application runs, as nothing is expected to
// run then. It counts for the readiness probe unless its kind is set.
//
// The check also fails once the kennel drains on a signal, as set at
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog.DrainDelayConfigPath],
// so that the readiness probe fails while the servers still serve, and
// the load balancers stop sending requests before the servers stop,
// unless its draining field holds false, as for a check of the liveness
// probe, which should not fail then.
package kennel
