package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Queue_Enqueue(t *testing.T) {
	ctx := context.Background()

	t.Run("should store a pending job", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		testutil.Equal(t, "Id()", q.Id(), "main")

		id := enqueue(t, q, "mail", map[string]int{"user": 7}, nil)
		testutil.Equal(t, "id", id, "1")
		row := e.row(t, id)
		testutil.Equal(t, "Queue", row.Queue, "main")
		testutil.Equal(t, "Kind", row.Kind, "mail")
		testutil.Equal(t, "DedupKey", row.DedupKey, (*string)(nil))
		testutil.Equal(t, "Payload", string(row.Payload), `{"user":7}`)
		testutil.Equal(t, "State", row.State, jobs.StatePending)
		testutil.Equal(t, "Attempt", row.Attempt, 0)
		testutil.Equal(t, "MaxAttempts", row.MaxAttempts, 5)
		testutil.Equal(t, "RunAt", row.RunAt.Equal(ts), true)
		testutil.Equal(t, "EnqueuedAt", row.EnqueuedAt.Equal(ts), true)
		testutil.Equal(t, "LockedUntil", row.LockedUntil, (*time.Time)(nil))
		testutil.Equal(t, "FinishedAt", row.FinishedAt, (*time.Time)(nil))

		testutil.Equal(t, "second id", enqueue(t, q, "mail", nil), "2")
	})

	t.Run("should apply the options", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)

		id := enqueue(t, q, "mail", nil, jobs.WithDelay(time.Minute), jobs.WithMaxAttempts(2))
		row := e.row(t, id)
		testutil.Equal(t, "RunAt", row.RunAt.Equal(ts.Add(time.Minute)), true)
		testutil.Equal(t, "MaxAttempts", row.MaxAttempts, 2)

		at := time.Date(2030, 1, 1, 0, 0, 0, 1500, time.FixedZone("x", 3600))
		id = enqueue(t, q, "mail", nil, jobs.WithRunAt(at), jobs.WithDelay(time.Minute))
		row = e.row(t, id)
		testutil.Equal(t, "RunAt", row.RunAt.Equal(at.Truncate(time.Microsecond)), true)
	})

	t.Run("should not store a job of a stored key again", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)

		first := enqueue(t, q, "mail", 1, jobs.WithKey("user-7"))
		testutil.Equal(t, "Key", *e.row(t, first).DedupKey, "user-7")
		testutil.Equal(t, "again", enqueue(t, q, "mail", 2, jobs.WithKey("user-7")), first)
		testutil.Equal(t, "rows", e.rows(t), int64(1))
		testutil.Equal(t, "Payload", string(e.row(t, first).Payload), "1")

		other := enqueue(t, q, "mail", 3, jobs.WithKey("user-8"))
		testutil.Equal(t, "other key", other == first, false)
	})

	t.Run("should store the job with the transaction", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)

		boom := errors.New("boom")
		err := e.conn.Transaction(func(tx *gorm.DB) error {
			enqueue(t, q, "mail", nil, jobs.WithTx(tx))
			return boom
		})
		testutil.Equal(t, "rolled back", err, boom)
		testutil.Equal(t, "rows", e.rows(t), int64(0))

		testutil.NoError(t, e.conn.Transaction(func(tx *gorm.DB) error {
			enqueue(t, q, "mail", nil, jobs.WithTx(tx))
			return nil
		}))
		testutil.Equal(t, "rows", e.rows(t), int64(1))
	})

	t.Run("should skip the nil options", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		enqueue(t, e.queue(t), "mail", nil, nil)
		testutil.Equal(t, "rows", e.rows(t), int64(1))
	})

	t.Run("should refuse the invalid arguments", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)

		_, err := q.Enqueue(ctx, "", nil)
		testutil.Error(t, err, jobs.ErrInvalidKind, "unable to enqueue job: invalid job kind",
			map[string]any{"id": "main", "kind": ""})
		_, err = q.Enqueue(ctx, "mail", nil, jobs.WithMaxAttempts(-1))
		testutil.Error(t, err, jobs.ErrInvalidOption, "unable to enqueue job: invalid job option",
			map[string]any{"id": "main", "kind": "mail"})
		_, err = q.Enqueue(ctx, "mail", make(chan int))
		testutil.Error(t, err, jobs.ErrEncode, "unable to enqueue job: unable to encode job payload: "+
			"json: unsupported type: chan int", map[string]any{"id": "main", "kind": "mail"})
		testutil.Equal(t, "rows", e.rows(t), int64(0))
	})

	t.Run("should fail once closed", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		testutil.NoError(t, q.Close())
		testutil.NoError(t, q.Close())
		_, err := q.Enqueue(ctx, "mail", nil)
		testutil.Error(t, err, jobs.ErrClosed, "unable to enqueue job: job queue closed",
			map[string]any{"id": "main", "kind": "mail"})
	})

	t.Run("should fail when the job cannot be stored", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		boom := errors.New("boom")
		inject(t, e.conn, "create", "fail", func(db *gorm.DB) { _ = db.AddError(boom) })

		_, err := q.Enqueue(ctx, "mail", nil)
		testutil.Error(t, err, jobs.ErrEnqueue, "unable to enqueue job: unable to enqueue job: boom",
			map[string]any{"id": "main", "kind": "mail"})
		if !errors.Is(err, boom) {
			t.Errorf("error = %v, want it to wrap boom", err)
		}
	})

	t.Run("should fail when the job of the key cannot be read", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		enqueue(t, q, "mail", nil, jobs.WithKey("k"))
		boom := errors.New("boom")
		inject(t, e.conn, "query", "fail", func(db *gorm.DB) { _ = db.AddError(boom) })

		_, err := q.Enqueue(ctx, "mail", nil, jobs.WithKey("k"))
		testutil.Error(t, err, jobs.ErrEnqueue, "unable to enqueue job: unable to enqueue job: boom",
			map[string]any{"id": "main", "kind": "mail"})
	})

	t.Run("should publish a wakeup notification", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "events"}})
		q := e.queue(t)
		b, err := e.brokers(t).Get("events")
		testutil.NoError(t, err)
		got := make(chan string, 1)
		_, err = b.Subscribe(ctx, "flam.jobs.main", func(_ context.Context, msg flam.IBrokerMessage) error {
			got <- string(msg.Payload())
			return nil
		})
		testutil.NoError(t, err)

		enqueue(t, q, "mail", nil)
		select {
		case payload := <-got:
			testutil.Equal(t, "payload", payload, "main")
		case <-time.After(10 * time.Second):
			t.Fatal("no wakeup notification")
		}
	})

	t.Run("should log the failure to publish a wakeup notification", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "events", jobs.ConfigFieldTopic: "wake"}})
		q := e.queue(t)
		testutil.NoError(t, e.brokers(t).Remove("events"))

		enqueue(t, q, "mail", nil)
		got, ok := e.logger.find("unable to notify job queue")
		testutil.Equal(t, "logged", ok, true)
		testutil.Equal(t, "level", got.level, flam.LogWarning)
		testutil.Equal(t, "channel", got.channel, jobs.LogChannel)
		testutil.Equal(t, "ctx", got.ctx, map[string]any{"id": "main", "broker": "events"})
		testutil.Equal(t, "rows", e.rows(t), int64(1))
	})
}
