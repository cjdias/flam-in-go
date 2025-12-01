package flam

type watchdog struct {
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
	watchdog.isRunning = true
	for {
		e = runner()
		if panicErr != nil {
			watchdog.logError(panicErr)
			panicErr = nil

			continue
		}

		break
	}
	watchdog.isRunning = false
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
