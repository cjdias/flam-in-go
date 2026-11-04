// Package lock is the distributed lock feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.ILockerFactory] of an
// application, which creates the lockers described by the configuration
// at [LockerConfigPath] with the creators provided in the
// [LockerCreatorGroup] container group, such as:
//
//	flam:
//	  lock:
//	    lockers:
//	      main:  {driver: flam.lock.lockers.driver.redis, connection: main, prefix: "orders:lock:"}
//	      local: {driver: flam.lock.lockers.driver.memory, auto_refresh: false}
//
// A locker acquires locks on string keys, each expiring once its ttl
// elapsed unless refreshed, so that a crashed holder does not keep it
// forever, and each held under a random token, unique to the acquisition,
// so that only its holder refreshes or releases it, never one that
// acquired it after it was lost. Each acquisition of a key also takes a
// fencing token, a number increasing with each acquisition, to hand to
// the resources the lock guards, so that they refuse the writes of a
// holder that lost the lock without knowing it.
//
// The locker driver may refresh the locks on their own, as configured by
// [ConfigFieldAutoRefresh], and each lock tells its holder when it is lost
// by closing the channel of its Lost method, as when its backend could
// not be reached for its whole ttl.
//
// The package [github.com/cjdias/flam-in-go/pkg/flam/lock/leader] builds
// a leader election on a locker, as a process of the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog].
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/lock/lockers], reading the fields
// shared by the drivers, such as [ConfigFieldRetryInterval], along with
// their own. They import this package for its paths, groups and errors,
// so this package never imports them.
package lock
