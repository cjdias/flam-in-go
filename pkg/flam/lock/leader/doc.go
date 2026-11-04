// Package leader is the leader election over a
// [github.com/cjdias/flam-in-go/pkg/flam.ILocker]: the process returned
// from [NewProcess], run by the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], campaigns for the lock
// of a key, and runs its function while it holds it, so that among the
// processes campaigning for a key with the lockers of a backend, such as
// those of the instances of an application sharing a Redis server, a
// single one, the leader, runs its function at a time:
//
//	app.Provide(func(lockers flam.ILockerFactory) (flam.IWatchdogProcess, error) {
//		l, err := lockers.Get("main")
//		if err != nil {
//			return nil, err
//		}
//		return leader.NewProcess("scheduler", l, "scheduler", 15*time.Second, schedule)
//	}, dig.Group(watchdog.ProcessGroup))
//
// The leadership lasts as long as the lock: the locker should refresh its
// locks on its own, as the lockers do unless configured otherwise, so that
// the leader keeps the lock while it lives, the ttl bounding the time
// another process waits for the lock of a leader that crashed. When the
// lock is lost, as when the backend could not be reached for its whole
// ttl, the context of the function is cancelled: the function must then
// stop acting as the leader, as another process may already lead.
//
// As a leader stalled past its ttl may still act while another leads, the
// writes of the function should carry the fencing token of its lock, read
// from its context with [FenceOf], the resources refusing a fence lower
// than the last one they saw.
package leader
