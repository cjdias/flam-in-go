// Package watchdog is the watchdog feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IKennel] of an application,
// holding its long running processes, such as its servers, and the
// [github.com/cjdias/flam-in-go/pkg/flam.IWatchdogLoggerAdaptorFactory]
// creating the logger adaptors described by the configuration at
// [LoggerConfigPath] with the creators provided in the
// [LoggerCreatorGroup] container group.
//
// The processes are provided in the [ProcessGroup] container group, and
// configured by their id at [ProcessConfigPath], or added to the kennel
// with Add, such as:
//
//	flam:
//	  watchdog:
//	    processes:
//	      api: {logger: standard}
//	      jobs: {active: false}
//
// When the application runs, the kennel runs each active process under a
// watchdog, which runs it again when it panics, and reports its start,
// its failures and its end to its logger adaptor, until they all end:
// the run of the application then returns the errors they returned
// joined, the panics being reported to the logger adaptors only.
//
// An interrupt or a SIGTERM signal shuts the kennel down, as Kubernetes
// expects: the kennel first drains for the delay at
// [DrainDelayConfigPath], 0 unless set, its IsDraining method reporting
// true, so that the readiness probe of the application fails, while the
// processes, such as the servers, keep serving the requests still sent
// to it, such as:
//
//	flam:
//	  watchdog:
//	    drain_delay: 5s
//
// The kennel then closes, cancelling the context given to the Run method
// of the processes, then closing every process, so that they end and the
// run returns, the application being closed after it. A second signal
// during the drain cuts it short, closing the kennel at once, while a
// signal following the drain ends the program as usual, should a process
// fail to end. A signal the program started ignoring stays ignored. As the application cannot be closed from another goroutine
// while it runs, closing a process, or sending these signals, is the way
// to end the run.
//
// A process panicking is run again after a delay, of 100ms unless set at
// [RestartDelayConfigPath], doubled at each following panic, up to 30s
// unless set at [RestartMaxDelayConfigPath], and reset after a run lasting
// that maximum at least. The panics are reported with the value and the
// stack of the panic, at [ErrorFieldPanic] and [ErrorFieldStack] in the
// context of their error. The panics of the logger adaptors are dropped,
// so that they do not end the program.
//
// The logger adaptor drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers]. They import
// this package for its paths and groups, so this package never imports
// them.
package watchdog
