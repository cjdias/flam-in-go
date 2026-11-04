package jobs

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// seen is what a handler saw of a job.
type seen struct {
	id          string
	queue       string
	kind        string
	key         string
	payload     string
	decoded     map[string]int
	decodeErr   bool
	attempt     int
	maxAttempts int
	enqueuedAt  time.Time
}

// recorder returns the handler of the kind sending what it sees of each
// job to the channel, returned along with it, then returning the error of
// fail for the number of the call, from 1, when fail is not nil.
func recorder(kind string, fail func(n int) error) (*handler, <-chan seen) {
	ch := make(chan seen, 100)
	var n atomic.Int64
	return &handler{kind: kind, fn: func(_ context.Context, job flam.IJob) error {
		count := int(n.Add(1))
		s := seen{
			id: job.Id(), queue: job.Queue(), kind: job.Kind(), key: job.Key(), payload: string(job.Payload()),
			attempt: job.Attempt(), maxAttempts: job.MaxAttempts(), enqueuedAt: job.EnqueuedAt(),
		}
		s.decodeErr = job.Decode(&s.decoded) != nil
		ch <- s
		if fail != nil {
			return fail(count)
		}
		return nil
	}}, ch
}

// receive returns the next value of the channel, failing the test when
// none comes in time.
func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(10 * time.Second):
		t.Fatal("nothing received in time")
	}
	var zero T
	return zero
}

func Test_Worker_Run(t *testing.T) {
	t.Run("should run the jobs and record them done", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		w := q.Worker()
		testutil.Equal(t, "Id()", w.Id(), "jobs-main")
		testutil.Equal(t, "IsRunning()", w.IsRunning(), false)
		done := run(t, w)

		id := enqueue(t, q, "mail", map[string]int{"user": 7}, jobs.WithKey("k"))
		testutil.Equal(t, "seen", receive(t, ch), seen{
			id: id, queue: "main", kind: "mail", key: "k", payload: `{"user":7}`, decoded: map[string]int{"user": 7},
			attempt: 1, maxAttempts: 5, enqueuedAt: e.row(t, id).EnqueuedAt,
		})
		eventually(t, "done", e.state(t, id, jobs.StateDone))
		row := e.row(t, id)
		testutil.Equal(t, "FinishedAt", row.FinishedAt.Equal(ts), true)
		testutil.Equal(t, "LockedUntil", row.LockedUntil, (*time.Time)(nil))
		testutil.Equal(t, "Attempt", row.Attempt, 1)

		id = enqueue(t, q, "mail", "text")
		s := receive(t, ch)
		testutil.Equal(t, "key", s.key, "")
		testutil.Equal(t, "decode error", s.decodeErr, true)
		eventually(t, "done", e.state(t, id, jobs.StateDone))

		stop(t, w, done)
		testutil.Equal(t, "IsRunning()", w.IsRunning(), false)
		testutil.Equal(t, "logged", e.logger.count(), 0)
	})

	t.Run("should run the jobs in run time order, once their time is reached", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		late := enqueue(t, q, "mail", "late", jobs.WithDelay(2*time.Second))
		early := enqueue(t, q, "mail", "early", jobs.WithDelay(time.Second))
		done := run(t, q.Worker())

		e.advance(t, 1, time.Second)
		testutil.Equal(t, "first", receive(t, ch).id, early)
		e.advance(t, 1, time.Second)
		testutil.Equal(t, "second", receive(t, ch).id, late)
		eventually(t, "done", e.state(t, late, jobs.StateDone))
		stop(t, q.Worker(), done)
	})

	t.Run("should retry a failing job after a backoff, then make it dead", func(t *testing.T) {
		h, ch := recorder("mail", func(n int) error { return fmt.Errorf("failure %d", n) })
		e := newEnv(t, options{
			queue:    container.Bag{jobs.ConfigFieldMaxAttempts: 4, jobs.ConfigFieldBackoff: "1s", jobs.ConfigFieldMaxBackoff: "3s"},
			handlers: []flam.IJobHandler{h},
		})
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "mail", nil)

		elapsed := time.Duration(0)
		for attempt, delay := range []time.Duration{time.Second, 2 * time.Second, 3 * time.Second} {
			testutil.Equal(t, "attempt", receive(t, ch).attempt, attempt+1)
			eventually(t, "pending", func() bool {
				row := e.row(t, id)
				return row.State == jobs.StatePending && row.Attempt == attempt+1
			})
			row := e.row(t, id)
			testutil.Equal(t, "RunAt", row.RunAt.Equal(ts.Add(elapsed+delay)), true)
			testutil.Equal(t, "LastError", row.LastError, fmt.Sprintf("unable to run job: job handler failed: failure %d", attempt+1))
			testutil.Equal(t, "LockedUntil", row.LockedUntil, (*time.Time)(nil))
			e.advance(t, 1, delay)
			elapsed += delay
		}
		testutil.Equal(t, "attempt", receive(t, ch).attempt, 4)
		eventually(t, "dead", e.state(t, id, jobs.StateDead))
		row := e.row(t, id)
		testutil.Equal(t, "LastError", row.LastError, "unable to run job: job handler failed: failure 4")
		testutil.Equal(t, "FinishedAt", row.FinishedAt.Equal(ts.Add(elapsed)), true)
		stop(t, q.Worker(), done)

		warning, _ := e.logger.find("failure 1")
		testutil.Equal(t, "warning", warning, entry{
			level: flam.LogWarning, channel: jobs.LogChannel, message: "unable to run job: job handler failed: failure 1",
			ctx: map[string]any{"id": "main", "job": id, "kind": "mail", "attempt": 1},
		})
		dead, _ := e.logger.find("job dead")
		testutil.Equal(t, "dead", dead, entry{
			level: flam.LogError, channel: jobs.LogChannel,
			message: "unable to run job: job dead: unable to run job: job handler failed: failure 4",
			ctx:     map[string]any{"id": "main", "job": id, "kind": "mail", "attempt": 4},
		})
	})

	t.Run("should fail the jobs of an unknown kind", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldMaxAttempts: 1}})
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "other", nil)
		eventually(t, "dead", e.state(t, id, jobs.StateDead))
		testutil.Equal(t, "LastError", e.row(t, id).LastError, "unable to run job: unknown job kind")
		stop(t, q.Worker(), done)
	})

	t.Run("should fail the jobs whose handler panics", func(t *testing.T) {
		boom := errors.New("boom")
		values := []any{boom, "text"}
		var n atomic.Int64
		h := &handler{kind: "mail", fn: func(context.Context, flam.IJob) error { panic(values[n.Add(1)-1]) }}
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldMaxAttempts: 1}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		done := run(t, q.Worker())

		first := enqueue(t, q, "mail", nil)
		eventually(t, "dead", e.state(t, first, jobs.StateDead))
		testutil.Equal(t, "LastError", e.row(t, first).LastError, "unable to run job: job handler panicked: boom")
		second := enqueue(t, q, "mail", nil)
		eventually(t, "dead", e.state(t, second, jobs.StateDead))
		testutil.Equal(t, "LastError", e.row(t, second).LastError, "unable to run job: job handler panicked")
		stop(t, q.Worker(), done)

		got, _ := e.logger.find("panicked: boom")
		testutil.Equal(t, "panic", got.ctx["panic"], any(boom))
		if !strings.Contains(got.ctx["stack"].(string), "goroutine") {
			t.Errorf("stack = %q, want a stack trace", got.ctx["stack"])
		}
	})

	t.Run("should delete the completed jobs without retention", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldRetention: "0s"}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		done := run(t, q.Worker())
		enqueue(t, q, "mail", nil)
		receive(t, ch)
		eventually(t, "deleted", func() bool { return e.rows(t) == 0 })
		stop(t, q.Worker(), done)
	})

	t.Run("should delete the completed jobs past their retention", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{
			queue:    container.Bag{jobs.ConfigFieldRetention: "1h", jobs.ConfigFieldCleanup: "30m", jobs.ConfigFieldPoll: "30m"},
			handlers: []flam.IJobHandler{h},
		})
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "mail", nil)
		receive(t, ch)
		eventually(t, "done", e.state(t, id, jobs.StateDone))
		dead := enqueue(t, q, "other", nil, jobs.WithMaxAttempts(1))
		eventually(t, "dead", e.state(t, dead, jobs.StateDead))

		e.advance(t, 1, 30*time.Minute)
		e.advance(t, 1, 30*time.Minute)
		e.advance(t, 1, 30*time.Minute)
		eventually(t, "deleted", func() bool { return e.rows(t) == 1 })
		testutil.Equal(t, "dead kept", e.row(t, dead).State, jobs.StateDead)
		stop(t, q.Worker(), done)
	})

	t.Run("should log the failure to clean up", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		inject(t, e.conn, "delete", "fail", func(db *gorm.DB) { _ = db.AddError(errors.New("boom")) })
		done := run(t, q.Worker())
		eventually(t, "logged", func() bool { _, ok := e.logger.find("unable to clean up jobs"); return ok })
		stop(t, q.Worker(), done)
		got, _ := e.logger.find("unable to clean up jobs")
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogError, channel: jobs.LogChannel, message: "unable to clean up jobs: unable to clean up jobs: boom",
			ctx: map[string]any{"id": "main"},
		})
	})

	t.Run("should claim again the jobs whose lease expired", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		past := ts.Add(-time.Second)
		crashed := jobs.Row{
			Queue: "main", Kind: "mail", Payload: []byte("null"), State: jobs.StateRunning, Attempt: 1, MaxAttempts: 2,
			RunAt: past, LockedUntil: &past, Token: "crashed", EnqueuedAt: past,
		}
		testutil.NoError(t, e.conn.Create(&crashed).Error)
		future := ts.Add(time.Hour)
		held := jobs.Row{
			Queue: "main", Kind: "mail", Payload: []byte("null"), State: jobs.StateRunning, Attempt: 1, MaxAttempts: 2,
			RunAt: past, LockedUntil: &future, Token: "held", EnqueuedAt: past,
		}
		testutil.NoError(t, e.conn.Create(&held).Error)
		exhausted := jobs.Row{
			Queue: "main", Kind: "mail", Payload: []byte("null"), State: jobs.StateRunning, Attempt: 2, MaxAttempts: 2,
			RunAt: past, LockedUntil: &past, Token: "exhausted", EnqueuedAt: past,
		}
		testutil.NoError(t, e.conn.Create(&exhausted).Error)

		done := run(t, q.Worker())
		s := receive(t, ch)
		testutil.Equal(t, "id", s.id, fmt.Sprint(crashed.ID))
		testutil.Equal(t, "attempt", s.attempt, 2)
		eventually(t, "done", e.state(t, s.id, jobs.StateDone))
		eventually(t, "dead", e.state(t, fmt.Sprint(exhausted.ID), jobs.StateDead))
		testutil.Equal(t, "LastError", e.row(t, fmt.Sprint(exhausted.ID)).LastError, "unable to run job: job lease expired")
		testutil.Equal(t, "held", e.row(t, fmt.Sprint(held.ID)).State, jobs.StateRunning)
		stop(t, q.Worker(), done)
	})

	t.Run("should drop the outcome of a job whose lease was lost", func(t *testing.T) {
		var conn flam.IDatabaseConnection
		h := &handler{kind: "mail", fn: func(_ context.Context, job flam.IJob) error {
			return conn.Model(&jobs.Row{}).Where("id = ?", job.Id()).Update("token", "other").Error
		}}
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		conn = e.conn
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "mail", nil)
		eventually(t, "logged", func() bool { _, ok := e.logger.find("job lease lost"); return ok })
		stop(t, q.Worker(), done)
		testutil.Equal(t, "state", e.row(t, id).State, jobs.StateRunning)
		got, _ := e.logger.find("job lease lost")
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogWarning, channel: jobs.LogChannel, message: "unable to record job outcome: job lease lost",
			ctx: map[string]any{"id": "main", "job": id, "kind": "mail", "attempt": 1},
		})
	})

	t.Run("should log the failure to record an outcome", func(t *testing.T) {
		for name, retention := range map[string]string{"update": "1h", "delete": "0s"} {
			t.Run(name, func(t *testing.T) {
				h, ch := recorder("mail", nil)
				e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldRetention: retention, jobs.ConfigFieldCleanup: "1000h"}, handlers: []flam.IJobHandler{h}})
				q := e.queue(t)
				var ran atomic.Bool
				fail := func(db *gorm.DB) {
					if ran.Load() {
						_ = db.AddError(errors.New("boom"))
					}
				}
				inject(t, e.conn, "update", "fail", func(db *gorm.DB) {
					if updates(db)["state"] == jobs.StateDone {
						fail(db)
					}
				})
				inject(t, e.conn, "delete", "fail", fail)
				done := run(t, q.Worker())
				id := enqueue(t, q, "mail", nil)
				ran.Store(true)
				receive(t, ch)
				eventually(t, "logged", func() bool { _, ok := e.logger.find("unable to record job outcome: boom"); return ok })
				stop(t, q.Worker(), done)
				got, _ := e.logger.find("unable to record job outcome: boom")
				testutil.Equal(t, "entry", got, entry{
					level: flam.LogError, channel: jobs.LogChannel, message: "unable to record job outcome: unable to record job outcome: boom",
					ctx: map[string]any{"id": "main", "job": id, "kind": "mail", "attempt": 1},
				})
			})
		}
	})

	t.Run("should claim the jobs in a transaction with skip locked", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldLocking: jobs.LockingSkipLocked}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		var locked atomic.Bool
		inject(t, e.conn, "query", "locking", func(db *gorm.DB) {
			if _, ok := db.Statement.Clauses["FOR"]; ok {
				locked.Store(true)
			}
		})
		done := run(t, q.Worker())
		id := enqueue(t, q, "mail", nil)
		testutil.Equal(t, "id", receive(t, ch).id, id)
		eventually(t, "done", e.state(t, id, jobs.StateDone))
		stop(t, q.Worker(), done)
		testutil.Equal(t, "locked", locked.Load(), true)
	})

	t.Run("should log the failure to claim a job", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		inject(t, e.conn, "query", "fail", func(db *gorm.DB) { _ = db.AddError(errors.New("boom")) })
		done := run(t, q.Worker())
		eventually(t, "logged", func() bool { _, ok := e.logger.find("unable to claim job"); return ok })
		stop(t, q.Worker(), done)
		got, _ := e.logger.find("unable to claim job")
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogError, channel: jobs.LogChannel, message: "unable to claim job: unable to claim job: boom",
			ctx: map[string]any{"id": "main"},
		})
	})

	t.Run("should log the failure to update a claimed job", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		inject(t, e.conn, "update", "fail", func(db *gorm.DB) {
			if _, ok := updates(db)["attempt"]; ok {
				_ = db.AddError(errors.New("boom"))
			}
		})
		enqueue(t, q, "mail", nil)
		done := run(t, q.Worker())
		eventually(t, "logged", func() bool { _, ok := e.logger.find("unable to claim job: boom"); return ok })
		stop(t, q.Worker(), done)
	})

	t.Run("should claim another job when one is claimed first by another worker", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		var stolen atomic.Bool
		inject(t, e.conn, "update", "steal", func(db *gorm.DB) {
			if _, ok := updates(db)["attempt"]; ok && stolen.CompareAndSwap(false, true) {
				_ = db.Session(&gorm.Session{NewDB: true}).Exec("UPDATE flam_jobs SET token = 'stolen'").Error
			}
		})
		id := enqueue(t, q, "mail", nil)
		done := run(t, q.Worker())
		testutil.Equal(t, "id", receive(t, ch).id, id)
		eventually(t, "done", e.state(t, id, jobs.StateDone))
		stop(t, q.Worker(), done)
		testutil.Equal(t, "stolen", stolen.Load(), true)
	})

	t.Run("should run up to the concurrency at once", func(t *testing.T) {
		release := make(chan struct{})
		var running, peak atomic.Int64
		h := &handler{kind: "mail", fn: func(context.Context, flam.IJob) error {
			n := running.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			<-release
			running.Add(-1)
			return nil
		}}
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldConcurrency: 3}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		ids := []string{}
		for range 5 {
			ids = append(ids, enqueue(t, q, "mail", nil))
		}
		done := run(t, q.Worker())
		eventually(t, "three running", func() bool { return running.Load() == 3 })
		close(release)
		for _, id := range ids {
			eventually(t, "done", e.state(t, id, jobs.StateDone))
		}
		stop(t, q.Worker(), done)
		testutil.Equal(t, "peak", peak.Load(), int64(3))
	})

	t.Run("should wait for the jobs under way when stopping, then make their context done", func(t *testing.T) {
		started := make(chan struct{})
		h := &handler{kind: "mail", fn: func(ctx context.Context, _ flam.IJob) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}}
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldStopTimeout: "10s"}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "mail", nil)
		<-started

		testutil.NoError(t, q.Worker().Close())
		select {
		case err := <-done:
			t.Fatalf("Run returned %v before the job ended", err)
		case <-time.After(20 * time.Millisecond):
		}
		e.advance(t, 1, 10*time.Second)
		testutil.NoError(t, receive(t, done))
		testutil.Equal(t, "state", e.row(t, id).State, jobs.StatePending)
		testutil.Equal(t, "LastError", e.row(t, id).LastError, "unable to run job: job handler failed: context canceled")
	})

	t.Run("should stop when its context is done", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldConcurrency: 1}})
		q := e.queue(t)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- q.Worker().Run(ctx) }()
		eventually(t, "running", q.Worker().IsRunning)
		cancel()
		testutil.NoError(t, receive(t, done))
	})

	t.Run("should return at once with a done context", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		enqueue(t, q, "mail", nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		testutil.NoError(t, q.Worker().Run(ctx))
		select {
		case <-ch:
			t.Fatal("a job ran")
		default:
		}
	})

	t.Run("should stop when closed while all its slots are busy", func(t *testing.T) {
		release := make(chan struct{})
		h := &handler{kind: "mail", fn: func(context.Context, flam.IJob) error { <-release; return nil }}
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		id := enqueue(t, q, "mail", nil)
		done := run(t, q.Worker())
		eventually(t, "running", e.state(t, id, jobs.StateRunning))
		testutil.NoError(t, q.Worker().Close())
		close(release)
		testutil.NoError(t, receive(t, done))
		testutil.Equal(t, "state", e.row(t, id).State, jobs.StateDone)
	})

	t.Run("should refuse to run twice at once", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		w := e.queue(t).Worker()
		done := run(t, w)
		testutil.Error(t, w.Run(context.Background()), jobs.ErrRunning, "unable to run job worker: job worker running",
			map[string]any{"id": "main"})
		stop(t, w, done)
	})

	t.Run("should return at once once closed", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		w := e.queue(t).Worker()
		testutil.NoError(t, w.Close())
		testutil.NoError(t, w.Close())
		var nilCtx context.Context
		testutil.NoError(t, w.Run(nilCtx))
	})

	t.Run("should run with a nil context", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		w := e.queue(t).Worker()
		done := make(chan error, 1)
		var nilCtx context.Context
		go func() { done <- w.Run(nilCtx) }()
		eventually(t, "running", w.IsRunning)
		stop(t, w, done)
	})

	t.Run("should log nothing without logger", func(t *testing.T) {
		e := newEnv(t, options{noLogger: true, queue: container.Bag{jobs.ConfigFieldMaxAttempts: 1}})
		q := e.queue(t)
		done := run(t, q.Worker())
		id := enqueue(t, q, "other", nil)
		eventually(t, "dead", e.state(t, id, jobs.StateDead))
		stop(t, q.Worker(), done)
		testutil.Equal(t, "logged", e.logger.count(), 0)
	})
}

func Test_Worker_Wakeup(t *testing.T) {
	t.Run("should run at once the jobs enqueued by another process", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "events", jobs.ConfigFieldPoll: "1h"}, handlers: []flam.IJobHandler{h}})
		consumer := e.queue(t)
		producer, err := e.queues(t).Generate("main")
		testutil.NoError(t, err)
		defer func() { _ = producer.Close() }()

		done := run(t, consumer.Worker())
		eventually(t, "waiting", func() bool { return e.clock.Timers() == 1 })
		time.Sleep(20 * time.Millisecond) // lets the subscription settle
		id := enqueue(t, producer, "mail", nil)
		testutil.Equal(t, "id", receive(t, ch).id, id)
		stop(t, consumer.Worker(), done)
	})

	t.Run("should log the failure to subscribe, and poll", func(t *testing.T) {
		h, ch := recorder("mail", nil)
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "events"}, handlers: []flam.IJobHandler{h}})
		q := e.queue(t)
		testutil.NoError(t, e.brokers(t).Remove("events"))
		done := run(t, q.Worker())
		eventually(t, "logged", func() bool { _, ok := e.logger.find("unable to run job worker"); return ok })
		got, _ := e.logger.find("unable to run job worker")
		testutil.Equal(t, "level", got.level, flam.LogWarning)
		testutil.Equal(t, "ctx", got.ctx, map[string]any{"id": "main", "broker": "events"})

		id := enqueue(t, q, "mail", nil)
		testutil.Equal(t, "id", receive(t, ch).id, id)
		stop(t, q.Worker(), done)
	})
}

func Test_Worker_Concurrency(t *testing.T) {
	t.Run("should run each job once with two workers", func(t *testing.T) {
		var mu sync.Mutex
		runs := map[string]int{}
		h := &handler{kind: "mail", fn: func(_ context.Context, job flam.IJob) error {
			mu.Lock()
			runs[job.Id()]++
			mu.Unlock()
			time.Sleep(time.Millisecond)
			return nil
		}}
		e := newEnv(t, options{
			real:     true,
			queue:    container.Bag{jobs.ConfigFieldConcurrency: 4, jobs.ConfigFieldPoll: "5ms", jobs.ConfigFieldLocking: jobs.LockingNone},
			handlers: []flam.IJobHandler{h},
		})
		first := e.queue(t)
		second, err := e.queues(t).Generate("main")
		testutil.NoError(t, err)
		defer func() { _ = second.Close() }()

		ids := []string{}
		for i := range 60 {
			ids = append(ids, enqueue(t, first, "mail", i))
		}
		firstDone := run(t, first.Worker())
		secondDone := run(t, second.Worker())
		for _, id := range ids {
			eventually(t, "done", e.state(t, id, jobs.StateDone))
		}
		stop(t, first.Worker(), firstDone)
		stop(t, second.Worker(), secondDone)

		mu.Lock()
		defer mu.Unlock()
		testutil.Equal(t, "jobs", len(runs), len(ids))
		for id, n := range runs {
			if n != 1 {
				t.Errorf("job %s ran %d times, want once", id, n)
			}
		}
		if _, ok := e.logger.find("lease lost"); ok {
			t.Error("a lease was lost")
		}
	})
}
