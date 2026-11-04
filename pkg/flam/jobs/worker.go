package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"runtime/debug"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// worker is the [flam.IWatchdogProcess] running the jobs of a queue.
type worker struct {
	id          string
	queue       *queue
	handlers    map[string]flam.IJobHandler
	poll        time.Duration
	concurrency int
	lease       time.Duration
	backoff     time.Duration
	maxBackoff  time.Duration
	retention   time.Duration
	cleanup     time.Duration
	stopTimeout time.Duration
	skipLocked  bool

	wakeCh chan struct{} // holds a pending wakeup
	stop   chan struct{} // closed by Close

	mu      sync.Mutex
	running bool
	closed  bool

	lastCleanup time.Time // read and written by the dispatcher only
}

// compile time check that *worker implements IWatchdogProcess
var _ flam.IWatchdogProcess = (*worker)(nil)

func (w *worker) Id() string {
	return w.id
}

func (w *worker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// Run claims and runs the jobs of the queue, up to the concurrency at
// once, until the context is done or the worker is closed, then waits for
// the jobs under way, whose context is done once the stop timeout
// elapses. A nil context never ends. It returns nil, the failures being
// logged, fails with ErrRunning when the worker runs already, and returns
// at once once the worker is closed.
func (w *worker) Run(
	ctx context.Context,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	w.mu.Lock()
	switch {
	case w.closed:
		w.mu.Unlock()
		return nil
	case w.running:
		w.mu.Unlock()
		return newErrQueue(ErrRunning, workerRunMessage, w.queue.id, nil)
	}
	w.running = true
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-runCtx.Done():
		}
	}()
	base := context.WithoutCancel(ctx)
	handlerCtx, handlerCancel := context.WithCancel(base)
	defer handlerCancel()

	if sub := w.subscribe(runCtx); sub != nil {
		defer func() { _ = sub.Close() }()
	}

	var wg sync.WaitGroup
	w.dispatch(runCtx, base, handlerCtx, &wg)
	timer := w.queue.clock.AfterFunc(w.stopTimeout, handlerCancel)
	wg.Wait()
	timer.Stop()
	return nil
}

// Close stops the worker, making a Run under way stop claiming jobs and
// return once the jobs under way end, and the later ones return at once.
func (w *worker) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.closed {
		w.closed = true
		close(w.stop)
	}
	return nil
}

// wake makes an idle worker poll at once.
func (w *worker) wake() {
	select {
	case w.wakeCh <- struct{}{}:
	default:
	}
}

// subscribe subscribes to the wakeup notifications of the queue, when it
// has a broker, returning the subscription, or nil without one or when
// the subscription fails, which is logged.
func (w *worker) subscribe(
	ctx context.Context,
) flam.IBrokerSubscription {
	b := w.queue.broker
	if b == nil {
		return nil
	}
	sub, err := b.Subscribe(ctx, w.queue.topic, func(context.Context, flam.IBrokerMessage) error {
		w.wake()
		return nil
	})
	if err != nil {
		w.queue.log(flam.LogWarning, newErrQueue(ErrWakeup, workerRunMessage, w.queue.id, err).
			With(ErrorFieldBroker, b.Id()))
		return nil
	}
	return sub
}

// dispatch claims the jobs while a slot is free, running each in a
// goroutine of the wait group, and waits for the poll delay or a wakeup
// when none is pending, until the run context is done.
func (w *worker) dispatch(
	runCtx context.Context,
	base context.Context,
	handlerCtx context.Context,
	wg *sync.WaitGroup,
) {
	slots := make(chan struct{}, w.concurrency)
	for runCtx.Err() == nil {
		w.clean(base)
		select {
		case slots <- struct{}{}:
		case <-runCtx.Done():
			return
		}

		j, err := w.claim(base)
		if err != nil {
			w.queue.log(flam.LogError, err)
		}
		if j == nil {
			<-slots
			if !w.wait(runCtx) {
				return
			}
			continue
		}
		wg.Go(func() {
			defer func() { <-slots }()
			w.execute(base, handlerCtx, j)
		})
	}
}

// wait waits for the poll delay or a wakeup, returning false once the run
// context is done.
func (w *worker) wait(
	runCtx context.Context,
) bool {
	timer := w.queue.clock.NewTimer(w.poll)
	defer timer.Stop()
	select {
	case <-timer.C():
	case <-w.wakeCh:
	case <-runCtx.Done():
		return false
	}
	return true
}

// clean deletes the completed jobs of the queue past their retention,
// once per cleanup delay, logging the failure.
func (w *worker) clean(
	ctx context.Context,
) {
	now := w.queue.now()
	if !w.lastCleanup.IsZero() && now.Sub(w.lastCleanup) < w.cleanup {
		return
	}
	w.lastCleanup = now
	err := w.queue.conn.WithContext(ctx).
		Where("queue = ? AND state = ? AND finished_at < ?", w.queue.id, StateDone, now.Add(-w.retention)).
		Delete(&Row{}).Error
	if err != nil {
		w.queue.log(flam.LogError, newErrQueue(ErrCleanup, workerCleanMessage, w.queue.id, err))
	}
}

// claim claims the next job of the queue to run, a pending one whose run
// time is reached or a running one whose lease expired, making dead the
// ones out of attempts, and returns it, or nil when none is.
func (w *worker) claim(
	ctx context.Context,
) (*job, *flam.Error) {
	for {
		now := w.queue.now()
		var claimed *job
		lost := false
		attempt := func(tx *gorm.DB) error {
			var rows []Row
			sel := tx.Where("queue = ? AND ((state = ? AND run_at <= ?) OR (state = ? AND locked_until <= ?))",
				w.queue.id, StatePending, now, StateRunning, now).
				Order("run_at, id").
				Limit(1)
			if w.skipLocked {
				sel = sel.Clauses(clause.Locking{Strength: clause.LockingStrengthUpdate, Options: clause.LockingOptionsSkipLocked})
			}
			if err := sel.Find(&rows).Error; err != nil || len(rows) == 0 {
				return err
			}

			row := rows[0]
			until := now.Add(w.lease)
			token := newToken()
			res := tx.Model(&Row{}).
				Where("id = ? AND token = ?", row.ID, row.Token).
				Updates(map[string]any{"state": StateRunning, "token": token, "locked_until": until, "attempt": row.Attempt + 1})
			switch {
			case res.Error != nil:
				return res.Error
			case res.RowsAffected == 0:
				lost = true
				return nil
			}
			row.State, row.Token, row.LockedUntil, row.Attempt = StateRunning, token, &until, row.Attempt+1
			claimed = &job{row: row}
			return nil
		}

		db := w.queue.conn.WithContext(ctx)
		var err error
		if w.skipLocked {
			err = db.Transaction(attempt)
		} else {
			err = attempt(db)
		}
		switch {
		case err != nil:
			return nil, newErrQueue(ErrClaim, workerClaimMessage, w.queue.id, err)
		case lost:
			continue
		case claimed != nil && claimed.row.Attempt > claimed.row.MaxAttempts:
			w.bury(ctx, claimed, newErrJob(ErrLeaseExpired, workerExecuteMessage, claimed, nil), now)
			continue
		}
		return claimed, nil
	}
}

// execute runs the job with its handler, and records its outcome: done,
// pending again after a backoff, or dead once out of attempts.
func (w *worker) execute(
	ctx context.Context,
	handlerCtx context.Context,
	j *job,
) {
	err := w.handle(handlerCtx, j)
	now := w.queue.now()
	switch {
	case err == nil:
		w.complete(ctx, j, now)
	case j.row.Attempt >= j.row.MaxAttempts:
		w.bury(ctx, j, err, now)
	default:
		w.queue.log(flam.LogWarning, err)
		w.record(ctx, j, map[string]any{
			"state":        StatePending,
			"run_at":       now.Add(w.delay(j.row.Attempt)),
			"locked_until": nil,
			"last_error":   err.Error(),
		})
	}
}

// handle calls the handler of the kind of the job, returning the error
// failing the attempt, if any.
func (w *worker) handle(
	ctx context.Context,
	j *job,
) (err *flam.Error) {
	h, ok := w.handlers[j.row.Kind]
	if !ok {
		return newErrJob(ErrUnknownKind, workerExecuteMessage, j, nil)
	}
	defer func() {
		if r := recover(); r != nil {
			var cause error
			if panicErr, isErr := r.(error); isErr {
				cause = panicErr
			}
			err = newErrJob(ErrHandlerPanic, workerExecuteMessage, j, cause).
				With(ErrorFieldPanic, r).
				With(ErrorFieldStack, string(debug.Stack()))
		}
	}()
	if e := h.Handle(ctx, j); e != nil {
		return newErrJob(ErrHandler, workerExecuteMessage, j, e)
	}
	return nil
}

// complete records the job as done, or deletes it without retention.
func (w *worker) complete(
	ctx context.Context,
	j *job,
	now time.Time,
) {
	if w.retention == 0 {
		w.record(ctx, j, nil)
		return
	}
	w.record(ctx, j, map[string]any{
		"state":        StateDone,
		"finished_at":  now,
		"locked_until": nil,
	})
}

// bury records the job as dead with the error, logging it with the
// context of the error, such as the panic of the handler.
func (w *worker) bury(
	ctx context.Context,
	j *job,
	err *flam.Error,
	now time.Time,
) {
	dead := newErrJob(ErrDead, workerExecuteMessage, j, err)
	for field, value := range err.Context() {
		dead = dead.With(field, value)
	}
	w.queue.log(flam.LogError, dead)
	w.record(ctx, j, map[string]any{
		"state":        StateDead,
		"finished_at":  now,
		"locked_until": nil,
		"last_error":   err.Error(),
	})
}

// record updates the job with the values, or deletes it when nil, as long
// as the worker holds its claim, logging the failure.
func (w *worker) record(
	ctx context.Context,
	j *job,
	values map[string]any,
) {
	db := w.queue.conn.WithContext(ctx).Model(&Row{}).Where("id = ? AND token = ?", j.row.ID, j.row.Token)
	var res *gorm.DB
	if values == nil {
		res = db.Delete(&Row{})
	} else {
		res = db.Updates(values)
	}
	switch {
	case res.Error != nil:
		w.queue.log(flam.LogError, newErrJob(ErrRecord, workerRecordMessage, j, res.Error))
	case res.RowsAffected == 0:
		w.queue.log(flam.LogWarning, newErrJob(ErrLeaseLost, workerRecordMessage, j, nil))
	}
}

// delay returns the backoff delay of the retry following the attempt:
// the backoff doubled at each attempt after the first, up to the maximum.
func (w *worker) delay(
	attempt int,
) time.Duration {
	d := w.backoff
	for i := 1; i < attempt && d < w.maxBackoff; i++ {
		if d > w.maxBackoff/2 {
			d = w.maxBackoff
		} else {
			d *= 2
		}
	}
	return min(d, w.maxBackoff)
}

// newToken returns a new random claim token.
func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails
	return hex.EncodeToString(b)
}

// Messages of the errors of the worker.
const (
	workerRunMessage     = "unable to run job worker"
	workerClaimMessage   = "unable to claim job"
	workerExecuteMessage = "unable to run job"
	workerRecordMessage  = "unable to record job outcome"
	workerCleanMessage   = "unable to clean up jobs"
)
