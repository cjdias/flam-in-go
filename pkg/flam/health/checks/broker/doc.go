// Package broker is the health check driver of the brokers: each check
// takes a broker of the
// [github.com/cjdias/flam-in-go/pkg/flam.IBrokerFactory] from the
// factory, and so creates it, connecting it to its backend, on its first
// run, such as:
//
//	flam:
//	  health:
//	    checks:
//	      events: {driver: flam.health.checks.driver.broker}
//	      jobs: {driver: flam.health.checks.driver.broker, broker: nats, topic: health}
//
// As the [github.com/cjdias/flam-in-go/pkg/flam.IBroker] contract has no
// ping, the check pings the broker when its driver implements the
// optional [github.com/cjdias/flam-in-go/pkg/flam.IHealthPinger], as the
// redis and nats drivers of
// [github.com/cjdias/flam-in-go/pkg/flam/broker/brokers] do. Otherwise,
// such as for the kafka driver, it publishes an empty message on the topic of
// its topic field, when set, which its subscribers, if any, must accept,
// and only takes the broker from the factory when unset.
//
// The check counts for the readiness probe unless its kind is set, as a
// backend that does not answer makes the application unable to serve,
// rather than broken.
package broker
