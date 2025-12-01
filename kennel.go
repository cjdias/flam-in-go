package flam

import (
	"io"
	"slices"
	"sync"

	"go.uber.org/dig"
)

type Kennel interface {
	io.Closer

	Available() []string
	Has(id string) bool
	IsActive(id string) bool
	Activate(id string) error
	Deactivate(id string) error
}

type kennelReg struct {
	config         Bag
	watchdogLogger WatchdogLogger
	process        Process
	watchdog       *watchdog
}

type kennel struct {
	config                Config
	watchdogLoggerFactory WatchdogLoggerFactory
	regs                  map[string]kennelReg
}

var _ Kennel = (*kennel)(nil)

func newKennel(args struct {
	dig.In

	Config                Config
	Processes             []Process `group:"flam.process"`
	WatchdogLoggerFactory WatchdogLoggerFactory
}) (*kennel, error) {
	kennel := &kennel{
		config:                args.Config,
		watchdogLoggerFactory: args.WatchdogLoggerFactory,
		regs:                  map[string]kennelReg{}}

	factoryConfig := args.Config.Bag(PathProcesses, Bag{})

	for _, process := range args.Processes {
		id := process.Id()
		processConfig := factoryConfig.Bag(id, Bag{})

		var watchdogLogger WatchdogLogger
		loggerId := processConfig.String("logger_id", args.Config.String(DefaultWatchdogLoggerId))
		if loggerId != "" {
			logger, e := args.WatchdogLoggerFactory.Get(loggerId)
			if e != nil {
				return nil, e
			}
			watchdogLogger = logger
		}

		kennel.regs[id] = kennelReg{
			config:         processConfig,
			watchdogLogger: watchdogLogger,
			process:        process,
			watchdog:       nil}
	}

	return kennel, nil
}

func (kennel *kennel) Close() error {
	for _, reg := range kennel.regs {
		if reg.watchdog != nil {
			_ = reg.watchdog.Close()
		}
	}

	return nil
}

func (kennel *kennel) Available() []string {
	var available []string
	for id := range kennel.regs {
		available = append(available, id)
	}

	return available
}

func (kennel *kennel) Has(
	id string,
) bool {
	return slices.Contains(kennel.Available(), id)
}

func (kennel *kennel) IsActive(
	id string,
) bool {
	reg, ok := kennel.regs[id]
	if !ok {
		return false
	}

	return reg.config.Bool("active", false)
}

func (kennel *kennel) Activate(
	id string,
) error {
	reg, ok := kennel.regs[id]
	switch {
	case !ok:
		return newErrProcessNotFound(id)
	case reg.process.IsRunning():
		return newErrProcessIsRunning(id)
	}

	return reg.config.Set("active", true)
}

func (kennel *kennel) Deactivate(
	id string,
) error {
	reg, ok := kennel.regs[id]
	switch {
	case !ok:
		return newErrProcessNotFound(id)
	case reg.process.IsRunning():
		return newErrProcessIsRunning(id)
	}

	return reg.config.Set("active", false)
}

func (kennel *kennel) run() error {
	if !kennel.config.Bool(PathKennelRun, false) {
		return nil
	}

	var result error
	wg := sync.WaitGroup{}
	for id, reg := range kennel.regs {
		if !reg.config.Bool("active", false) {
			continue
		}

		wd := newWatchdog(reg.process, reg.watchdogLogger)
		kennel.regs[id] = kennelReg{
			config:         reg.config,
			watchdogLogger: reg.watchdogLogger,
			process:        reg.process,
			watchdog:       wd}

		wg.Add(1)
		go func(watchdog *watchdog) {
			defer wg.Done()
			if e := watchdog.Run(); e != nil {
				result = e
			}
		}(wd)
	}
	wg.Wait()

	return result
}
