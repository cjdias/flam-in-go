package flam

import (
	"context"
	"io"
)

// IWatchdogLoggerAdaptor reports the lifecycle of the processes run by
// the watchdogs of an [IKennel], each by its id: its start, each failure,
// a panic restarting it or the error it returned, and its end. It is held
// by an [IWatchdogLoggerAdaptorFactory].
type IWatchdogLoggerAdaptor interface {
	IResource

	Start(id string)
	Error(id string, e error)
	Done(id string)
}

// IWatchdogLoggerAdaptorCreator creates the watchdog logger adaptors of a
// driver, for the [IWatchdogLoggerAdaptorFactory].
type IWatchdogLoggerAdaptorCreator = IResourceCreator[IWatchdogLoggerAdaptor]

// IWatchdogLoggerAdaptorFactory creates and holds the watchdog logger
// adaptors, one per id.
type IWatchdogLoggerAdaptorFactory = IFactory[IWatchdogLoggerAdaptor]

// IWatchdogProcess is a long running process of an application, such as a
// server, run by a watchdog of an [IKennel]. Run runs it until it ends, or
// until its context is done or it is closed, and Close stops it, making a
// Run under way return, and releases it. The kennel cancels the context
// when it closes, before closing the processes, so that a process may stop
// on either. It must be safe to Close it from another goroutine than the
// one running it.
type IWatchdogProcess interface {
	io.Closer
	IResource

	Run(ctx context.Context) error

	// IsRunning reports whether Run is under way, such as for the health
	// checks of the application. The kennel does not use it.
	IsRunning() bool
}

// IKennel holds the processes of an application, each with its logger
// adaptor, and runs the active ones, each under a watchdog running it
// again when it panics. It owns the processes added to it, and closes
// them. It may be used from several goroutines.
type IKennel interface {
	IResource

	// Available returns the ids of the processes, sorted.
	Available() []string

	// Has reports whether the kennel holds a process of the id.
	Has(id string) bool

	// Add adds the process of the id, run by the kennel when active, and
	// reporting to the logger adaptor of the id, unless empty. As the run
	// of the kennel takes the active processes when it starts, Add,
	// Activate and Deactivate fail while it runs.
	Add(id string, active bool, loggerAdaptorId string, process IWatchdogProcess) error

	// IsActive reports whether the process of the id is active, false for
	// an unknown one.
	IsActive(id string) bool

	// Activate makes the process of the id run with the next run of the
	// kennel.
	Activate(id string) error

	// Deactivate keeps the process of the id from running with the next
	// run of the kennel.
	Deactivate(id string) error

	// Process returns the process of the id, and whether the kennel holds
	// one.
	Process(id string) (IWatchdogProcess, bool)

	// IsRunning reports whether the kennel runs its active processes, such
	// as for the health checks of the application.
	IsRunning() bool
}
