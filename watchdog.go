package flam

import "sync"

type watchdog struct {
	mu             sync.Mutex
	isRunning      bool
	process        Process
	watchdogLogger WatchdogLogger
}

func newWatchdog(
	process Process,
	watchdogLogger WatchdogLogger,
) *watchdog {
	return &watchdog{
		process:        process,
		watchdogLogger: watchdogLogger,
	}
}

func (watchdog *watchdog) Close() error {
	watchdog.mu.Lock()
	defer watchdog.mu.Unlock()

	if watchdog.isRunning {
		watchdog.process.Terminate()
	}

	return nil
}

func (watchdog *watchdog) Run() error {
	var panicErr error
	var e error

	runner := func() error {
		defer func() {
			if response := recover(); response != nil {
				switch typedResponse := response.(type) {
				case error:
					panicErr = typedResponse
				default:
					panicErr = newErrProcessRunningError(response)
				}
			}
		}()

		return watchdog.process.Run()
	}

	watchdog.logStart()
	watchdog.mu.Lock()
	watchdog.isRunning = true
	watchdog.mu.Unlock()
	for {
		e = runner()
		if panicErr != nil {
			watchdog.logError(panicErr)
			panicErr = nil

			continue
		}

		break
	}
	watchdog.mu.Lock()
	watchdog.isRunning = false
	watchdog.mu.Unlock()
	watchdog.logDone()

	return e
}

func (watchdog *watchdog) logStart() {
	if watchdog.watchdogLogger != nil {
		watchdog.watchdogLogger.LogStart(watchdog.process.Id())
	}
}

func (watchdog *watchdog) logError(
	e error,
) {
	if watchdog.watchdogLogger != nil {
		watchdog.watchdogLogger.LogError(watchdog.process.Id(), e)
	}
}

func (watchdog *watchdog) logDone() {
	if watchdog.watchdogLogger != nil {
		watchdog.watchdogLogger.LogDone(watchdog.process.Id())
	}
}
