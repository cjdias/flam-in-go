package funit

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"time"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Run takes the application through the stages of the test: it adds the
// configuration, activates the processes, boots the application, runs the
// arrange functions, runs the application until its processes run, runs
// the act and the assertions, then, whatever happened before, closes the
// processes, waits for the run of the application to end, runs the
// teardown functions, rolls back the transaction, cancels the
// subscriptions and removes the configuration sources.
//
// It returns the first error of the stages, or else of the end of the
// run, reported to the test as well, and fails with ErrRan when the runner
// ran already. A stage panicking still ends the run before the panic goes
// on. The run of the application ends with the close of its kennel, so
// that it does not run again.
func (r *Runner) Run() (err error) {
	r.t.Helper()
	defer func() {
		if err != nil {
			r.t.Errorf("funit: %v", err)
		}
	}()

	switch {
	case r.err != nil:
		return r.err
	case r.ran:
		return newErr(ErrRan, runMessage, nil)
	}
	r.ran = true

	defer func() {
		if endErr := r.end(); err == nil {
			err = endErr
		}
	}()

	if err = r.arrange(); err != nil {
		return err
	}
	if r.act != nil {
		r.logger.Stage(StageAct)
		if err = r.invoke(StageAct, *r.act); err != nil {
			return err
		}
	}
	r.logger.Stage(StageAssert)
	for _, s := range r.asserts {
		if err = r.invoke(StageAssert, s); err != nil {
			return err
		}
	}
	return nil
}

// Message of the errors returned by Run.
const runMessage = "unable to run funit runner"

// arrange adds the configuration, activates the processes, boots the
// application, runs the arrange functions, then runs the application.
func (r *Runner) arrange() error {
	r.logger.Stage(StageConfig)
	for i, reg := range r.configs {
		if err := r.addConfig(i, reg); err != nil {
			r.logger.Error(StageConfig, reg.file, err)
			return err
		}
	}

	if len(r.processes) > 0 {
		if err := r.activate(); err != nil {
			return err
		}
	}

	r.logger.Stage(StageBoot)
	if err := r.app.Boot(); err != nil {
		r.logger.Error(StageBoot, "", err)
		return newErr(ErrBoot, runMessage, err)
	}

	r.logger.Stage(StageArrange)
	for _, s := range r.arranges {
		if err := r.invoke(StageArrange, s); err != nil {
			return err
		}
	}

	r.logger.Stage(StageRun)
	if err := r.start(); err != nil {
		r.logger.Error(StageRun, "", err)
		return err
	}
	return nil
}

// addConfig adds the values of the i-th configuration file, or the
// values, as a source of the configuration, recording its id for its
// removal.
func (r *Runner) addConfig(
	i int,
	reg configReg,
) error {
	id := fmt.Sprintf("%s/config/%d", r.id, i)
	values := reg.values
	if values == nil {
		r.logger.Step(StageConfig, reg.file)
		var err error
		if values, err = r.readConfig(reg); err != nil {
			return err
		}
	} else {
		r.logger.Step(StageConfig, id)
	}

	src := &source{id: id, priority: reg.priority, values: values}
	if err := r.app.Invoke(func(config flam.IConfig) error { return config.AddSource(src) }); err != nil {
		return newErr(ErrConfig, runMessage, err).With(ErrorFieldFile, reg.file)
	}
	r.added = append(r.added, src)
	return nil
}

// readConfig reads the configuration file with the configuration parser
// of its parser id, taken from the parser factory of the application.
func (r *Runner) readConfig(
	reg configReg,
) (flam.IBag, error) {
	var values flam.IBag
	err := r.app.Invoke(func(parsers flam.IConfigParserFactory) error {
		parser, err := parsers.Get(reg.parser)
		if err != nil {
			return err
		}
		file, err := os.Open(reg.file)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		values, err = parser.Parse(file)
		return err
	})
	if err != nil {
		return nil, newErr(ErrConfig, runMessage, err).
			With(ErrorFieldFile, reg.file).
			With(ErrorFieldParser, reg.parser)
	}
	return values, nil
}

// activate activates the processes of the test in the kennel of the
// application, recording those inactive before, deactivated once the run
// ends, failing with ErrProcess without kennel, or for a process the
// kennel does not hold.
func (r *Runner) activate() error {
	var process string
	err := r.app.Invoke(func(kennel flam.IKennel) error {
		r.kennel = kennel
		for _, process = range r.processes {
			if kennel.IsActive(process) {
				continue
			}
			if err := kennel.Activate(process); err != nil {
				return err
			}
			r.activated = append(r.activated, process)
		}
		return nil
	})
	if err != nil {
		return newErr(ErrProcess, runMessage, err).With(ErrorFieldProcess, process)
	}
	return nil
}

// start runs the application: on a goroutine of its own when it has a
// kennel, waiting for the run to end or for its processes to run, as the
// run of the kennel lasts as long as they do, and else at once, its run
// then ending once its bundles ran.
func (r *Runner) start() error {
	// the kennel is optional, and the application booted, so that this
	// invocation does not fail
	var args kennelArgs
	_ = r.app.Invoke(func(a kennelArgs) { args = a })

	if args.Kennel == nil {
		if err := r.app.Run(); err != nil {
			return newErr(ErrRun, runMessage, err)
		}
		return nil
	}

	r.kennel = args.Kennel
	r.done = make(chan error, 1)
	go func() {
		r.done <- r.app.Run()
	}()

	deadline := time.NewTimer(r.startTimeout)
	defer deadline.Stop()
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		select {
		case err := <-r.done:
			r.done = nil
			switch {
			case err != nil:
				return newErr(ErrRun, runMessage, err)
			case len(r.processes) > 0:
				// the act would call processes not running
				return newErr(ErrNotRunning, runMessage, nil).
					With(ErrorFieldProcess, r.processes)
			}
			return nil
		case <-deadline.C:
			return newErr(ErrStartTimeout, runMessage, nil).
				With(ErrorFieldTimeout, r.startTimeout)
		case <-poll.C:
			if r.started() {
				return nil
			}
		}
	}
}

// kennelArgs is the kennel of the application, taken from its container
// when provided.
type kennelArgs struct {
	dig.In

	Kennel flam.IKennel `optional:"true"`
}

// started reports whether the kennel runs, along with the processes of
// the test.
func (r *Runner) started() bool {
	return r.kennel.IsRunning() && slices.IndexFunc(r.processes, func(id string) bool {
		process, ok := r.kennel.Process(id)
		return !ok || !process.IsRunning()
	}) < 0
}

// end ends the run: it stops the application, runs the teardown
// functions, rolls back the transaction, cancels the subscriptions,
// deactivates the processes activated and removes the configuration
// sources, all of them even when one fails, returning their errors joined.
// While the run of the application goes on, past the stop timeout, what
// goes through its container is skipped, as the run holds it.
func (r *Runner) end() error {
	stopped, err := r.stop()
	errs := []error{err}

	r.logger.Stage(StageTeardown)
	if !stopped {
		r.logger.Error(StageTeardown, "", errStillRunning)
	} else {
		for _, s := range r.teardowns {
			errs = append(errs, r.invoke(StageTeardown, s))
		}
	}
	if r.tx != nil {
		if err := r.tx.Rollback().Error; err != nil {
			errs = append(errs, newErr(ErrRollback, runMessage, err))
		}
	}
	for _, topic := range r.topics {
		errs = append(errs, r.pubsub.Unsubscribe(r.subscription(topic)))
	}
	if !stopped {
		return errors.Join(errs...)
	}
	for _, id := range r.activated {
		if err := r.kennel.Deactivate(id); err != nil {
			errs = append(errs, newErr(ErrProcess, runMessage, err).With(ErrorFieldProcess, id))
		}
	}
	if len(r.added) > 0 {
		errs = append(errs, r.app.Invoke(func(config flam.IConfig) error {
			var errs []error
			for _, src := range r.added {
				errs = append(errs, config.RemoveSource(src.id), src.Close())
			}
			return errors.Join(errs...)
		}))
	}
	return errors.Join(errs...)
}

// errStillRunning is the error logged when the teardown is skipped, as
// the run of the application goes on.
var errStillRunning = errors.New("skipped, as the application still runs")

// stop closes the kennel, and so its processes, ending the run of the
// application, and waits for it to end, reporting whether it ended,
// failing with ErrStopTimeout when it does not end in time, and with
// ErrRun when it failed.
func (r *Runner) stop() (bool, error) {
	if r.done == nil {
		return true, nil
	}
	r.logger.Stage(StageStop)

	errs := []error{r.kennel.Close()}
	timer := time.NewTimer(r.stopTimeout)
	defer timer.Stop()
	select {
	case err := <-r.done:
		r.done = nil
		if err != nil {
			errs = append(errs, newErr(ErrRun, runMessage, err))
		}
		return true, errors.Join(errs...)
	case <-timer.C:
		errs = append(errs, newErr(ErrStopTimeout, runMessage, nil).
			With(ErrorFieldTimeout, r.stopTimeout))
		return false, errors.Join(errs...)
	}
}

// invoke invokes the function of the stage through the container of the
// application, failing with ErrStage when it fails.
func (r *Runner) invoke(
	name string,
	s stage,
) error {
	r.logger.Step(name, s.id)
	if err := r.app.Invoke(s.function); err != nil {
		r.logger.Error(name, s.id, err)
		return newErr(ErrStage, runMessage, err).With(ErrorFieldStage, s.id)
	}
	return nil
}
