package scheduler

import (
	"context"
	"runtime/debug"
	"sync"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// schedulerConfig is what a scheduler is created from.
type schedulerConfig struct {
	jobs     []*job
	triggers flam.ICronTriggerFactory
	clock    flam.IClock
	logger   flam.ILogger // nil for no logging
	channel  string
}

// scheduler is the [flam.IWatchdogProcess] running the jobs: its Run
// method creates a cron trigger per active job, calling the job at each
// time matched by its schedule, until its context is done or it is
// closed.
type scheduler struct {
	schedulerConfig

	// mu guards the state of the scheduler: closed, set by Close, which
	// also closes closeCh, and the run under way, if any, with the done
	// channel closed once it ended.
	mu      sync.Mutex
	closed  bool
	closeCh chan struct{}
	running bool
	done    chan struct{}

	// gate is read locked by each run of a job, and write locked by the
	// stop of the run of the scheduler, once the context of the jobs is
	// cancelled, to wait for the runs under way.
	gate sync.RWMutex
}

// compile time check that *scheduler implements IWatchdogProcess
var _ flam.IWatchdogProcess = (*scheduler)(nil)

// newScheduler returns a scheduler, not running.
func newScheduler(
	config schedulerConfig,
) *scheduler {
	return &scheduler{
		schedulerConfig: config,
		closeCh:         make(chan struct{}),
	}
}

// Close stops the scheduler: a run under way, if any, stops creating
// runs of the jobs, cancels the context of the runs under way and waits
// for them, Close returning once it ended. It returns nil, and calling it
// again is a no-op.
func (s *scheduler) Close() error {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		close(s.closeCh)
	}
	done := s.done
	s.mu.Unlock()

	if done != nil {
		<-done
	}
	return nil
}

func (s *scheduler) Id() string {
	return ProcessId
}

// Run schedules the active jobs, and runs each at the times matched by
// its schedule until the context is done or the scheduler is closed,
// then stops, as Close tells, returning nil. It fails when a run is under
// way, once the scheduler is closed, and when the trigger of a job cannot
// be created.
func (s *scheduler) Run(
	ctx context.Context,
) error {
	s.mu.Lock()
	switch {
	case s.closed:
		s.mu.Unlock()
		return newErr(ErrClosed, schedulerRunMessage, ProcessId)
	case s.running:
		s.mu.Unlock()
		return newErr(ErrRunning, schedulerRunMessage, ProcessId)
	}
	s.running = true
	s.done = make(chan struct{})
	s.mu.Unlock()

	runCtx, cancel := context.WithCancel(ctx)
	triggers, err := s.schedule(runCtx)
	if err == nil {
		select {
		case <-ctx.Done():
		case <-s.closeCh:
		}
	}
	s.stop(cancel, triggers)
	return err
}

func (s *scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.running
}

// schedule creates the trigger of each active job, returning them, along
// with the error of the first one that cannot be created.
func (s *scheduler) schedule(
	ctx context.Context,
) ([]flam.ICronTrigger, error) {
	var triggers []flam.ICronTrigger
	for _, j := range s.jobs {
		if !j.active {
			continue
		}
		t, err := s.triggers.NewCron(TriggerIdPrefix+j.id, j.schedule, s.callback(ctx, j))
		if err != nil {
			return triggers, newErrJoined(ErrSchedule, schedulerRunMessage, j.id, err)
		}
		triggers = append(triggers, t)
	}
	return triggers, nil
}

// stop closes the triggers, so that no run of a job starts, cancels the
// context of the runs under way, waits for them, and ends the run.
func (s *scheduler) stop(
	cancel context.CancelFunc,
	triggers []flam.ICronTrigger,
) {
	for _, t := range triggers {
		_ = t.Close() // always nil
	}
	cancel()
	s.gate.Lock() // waits for the runs under way
	defer s.gate.Unlock()

	s.mu.Lock()
	s.running = false
	close(s.done)
	s.done = nil
	s.mu.Unlock()
}

// callback returns the callback of the trigger of the job, running it
// with the context, unless the context is done, as the scheduler stops.
func (s *scheduler) callback(
	ctx context.Context,
	j *job,
) flam.TriggerCallback {
	return func() error {
		s.gate.RLock()
		defer s.gate.RUnlock()

		if ctx.Err() == nil {
			s.runJob(ctx, j)
		}
		return nil
	}
}

// runJob runs the job once, holding its lock when a singleton, the lock
// being acquired from its locker, or the run skipped when another holds
// it, and signals the run to the logger.
func (s *scheduler) runJob(
	ctx context.Context,
	j *job,
) {
	if j.locker != nil {
		lock, acquired, err := j.locker.TryAcquire(ctx, j.lockKey, j.lockTTL)
		switch {
		case err != nil:
			s.signal(flam.LogError, schedulerFailedMessage, j, container.Bag{
				ContextFieldError: newErrJoined(ErrLock, schedulerJobMessage, j.id, err),
			})
			return
		case !acquired:
			s.signal(flam.LogDebug, schedulerSkippedMessage, j, container.Bag{})
			return
		}
		defer func() {
			if err := lock.Release(context.WithoutCancel(ctx)); err != nil {
				s.signal(flam.LogError, schedulerFailedMessage, j, container.Bag{
					ContextFieldError: newErrJoined(ErrLock, schedulerJobMessage, j.id, err),
				})
			}
		}()
	}

	s.signal(flam.LogInfo, schedulerStartedMessage, j, container.Bag{})
	start := s.clock.Now()
	err := s.call(ctx, j)
	ctxBag := container.Bag{ContextFieldDuration: s.clock.Since(start)}
	if err != nil {
		ctxBag[ContextFieldError] = err
		s.signal(flam.LogError, schedulerFailedMessage, j, ctxBag)
		return
	}
	s.signal(flam.LogInfo, schedulerDoneMessage, j, ctxBag)
}

// call runs the job on a goroutine of its own, so that neither a panic
// nor a runtime.Goexit ends the trigger goroutine, with a context
// cancelled once its timeout elapses, and returns its failure, or nil. A
// nil error held by a non-nil error interface is not a failure.
func (s *scheduler) call(
	ctx context.Context,
	j *job,
) (err error) {
	jobCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	var timer flam.ITimer
	if j.timeout > 0 {
		cause := newErrTimeout(schedulerJobMessage, j.id, j.timeout, nil)
		timer = s.clock.AfterFunc(j.timeout, func() { cancel(cause) })
	}

	done := make(chan struct{})
	go func() {
		defer close(done)

		returned := false
		defer func() {
			r := recover()
			switch {
			case r != nil:
				err = newErrPanic(schedulerJobMessage, j.id, r, debug.Stack())
			case !returned:
				err = newErr(ErrExit, schedulerJobMessage, j.id)
			}
		}()

		result := j.job.Run(jobCtx)
		returned = true
		if !value.IsNil(result) {
			err = newErrJoined(ErrJob, schedulerJobMessage, j.id, result)
		}
	}()
	<-done

	// a timer that cannot be stopped has fired, or is firing
	if timer != nil && !timer.Stop() {
		err = newErrTimeout(schedulerJobMessage, j.id, j.timeout, err)
	}
	return err
}

// signal signals the message about the job to the logger, if any, at the
// level on the channel, with the job id added to the context.
func (s *scheduler) signal(
	level flam.LogLevel,
	message string,
	j *job,
	ctx container.Bag,
) {
	if s.logger == nil {
		return
	}
	ctx[ContextFieldJob] = j.id
	s.logger.Signal(level, s.channel, message, ctx)
}

// Messages of the errors of the scheduler and of the jobs, and signalled
// to the logger.
const (
	schedulerRunMessage     = "unable to run scheduler"
	schedulerJobMessage     = "unable to run scheduled job"
	schedulerStartedMessage = "scheduled job started"
	schedulerDoneMessage    = "scheduled job done"
	schedulerFailedMessage  = "scheduled job failed"
	schedulerSkippedMessage = "scheduled job skipped"
)
