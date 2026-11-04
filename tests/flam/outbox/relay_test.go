package outbox

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/outbox"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// payloads returns the topic and payload of the messages, as "topic:payload".
func payloads(messages []published) []string {
	got := []string{}
	for _, m := range messages {
		s := m.topic + ":" + m.payload
		if m.err {
			s += "!"
		}
		got = append(got, s)
	}
	return got
}

func Test_Relay_Run(t *testing.T) {
	t.Run("should publish the messages with the redis broker and mark them sent", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldBroker: "events"}})
		r := e.relay(t, "outbox-main")
		testutil.Equal(t, "IsRunning()", r.IsRunning(), false)

		var events flam.IBroker
		testutil.NoError(t, e.app.Invoke(func(f flam.IBrokerFactory) (err error) {
			events, err = f.Get("events")
			return err
		}))
		got := make(chan string, 10)
		_, err := events.Subscribe(context.Background(), "orders", func(_ context.Context, msg flam.IBrokerMessage) error {
			got <- string(msg.Payload()) + " " + fmt.Sprint(msg.Headers())
			return nil
		})
		testutil.NoError(t, err)

		testutil.NoError(t, e.conn.Transaction(func(tx *gorm.DB) error {
			testutil.NoError(t, outbox.Add(tx, "orders", []byte("one"), map[string]string{"h": "v"}))
			return outbox.Add(tx, "orders", []byte("two"), nil)
		}))
		done := run(t, r)
		for _, want := range []string{"one map[h:v]", "two map[]"} {
			select {
			case msg := <-got:
				testutil.Equal(t, "message", msg, want)
			case <-time.After(10 * time.Second):
				t.Fatal("no message")
			}
		}
		eventually(t, "sent", func() bool {
			rows := e.rows(t)
			return rows[0].SentAt != nil && rows[1].SentAt != nil
		})
		for _, row := range e.rows(t) {
			testutil.Equal(t, "SentAt", row.SentAt.Equal(ts), true)
			testutil.Equal(t, "LockedUntil", row.LockedUntil, (*time.Time)(nil))
		}
		stop(t, r, done)
		testutil.Equal(t, "IsRunning()", r.IsRunning(), false)
		testutil.Equal(t, "logged", e.logger.count(), 0)
	})

	t.Run("should delete the messages it published when told to", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldDelete: true}})
		r := e.relay(t, "outbox-main")
		e.add(t, "orders", "one", "two")
		done := run(t, r)
		eventually(t, "deleted", func() bool { return len(e.rows(t)) == 0 })
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()), []string{"orders:one", "orders:two"})
	})

	t.Run("should claim the next batch at once when the batch was full", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldBatch: 2, outbox.ConfigFieldPoll: "1h"}})
		r := e.relay(t, "outbox-main")
		e.add(t, "orders", "1", "2", "3", "4", "5")
		done := run(t, r)
		eventually(t, "published", func() bool { return len(e.fake.published()) == 5 })
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()),
			[]string{"orders:1", "orders:2", "orders:3", "orders:4", "orders:5"})
	})

	t.Run("should wait for the poll delay when no message of a full batch could be claimed", func(t *testing.T) {
		for name, claim := range map[string]func(db *gorm.DB, n int64){
			"claim failing": func(db *gorm.DB, _ int64) { _ = db.AddError(errors.New("boom")) },
			"claim lost": func(db *gorm.DB, n int64) {
				_ = db.Session(&gorm.Session{NewDB: true}).Exec("UPDATE flam_outbox SET token = ?", fmt.Sprint("stolen-", n)).Error
			},
		} {
			t.Run(name, func(t *testing.T) {
				e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldBatch: 2, outbox.ConfigFieldPoll: "1h"}})
				r := e.relay(t, "outbox-main")
				e.add(t, "a", "1")
				e.add(t, "b", "1")
				var claims atomic.Int64
				inject(t, e.conn, "update", "claim", func(db *gorm.DB) {
					if _, ok := updates(db)["token"]; ok {
						claim(db, claims.Add(1))
					}
				})
				done := run(t, r)
				eventually(t, "waiting", func() bool { return e.clock.Timers() == 1 })
				testutil.Equal(t, "claims", claims.Load(), int64(2))

				e.clock.Advance(time.Hour)
				eventually(t, "waiting again", func() bool { return claims.Load() == 4 && e.clock.Timers() == 1 })
				stop(t, r, done)
				testutil.Equal(t, "claims", claims.Load(), int64(4))
				testutil.Equal(t, "published", len(e.fake.published()), 0)
				for _, row := range e.rows(t) {
					testutil.Equal(t, "SentAt", row.SentAt, (*time.Time)(nil))
				}
			})
		}
	})

	t.Run("should hold back the later messages of a failing topic, keeping their order", func(t *testing.T) {
		var failures atomic.Int64
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldPoll: "500ms"}})
		e.fake.fail = func(topic, payload string) error {
			if topic == "a" && payload == "1" && failures.Add(1) == 1 {
				return errors.New("refused")
			}
			return nil
		}
		r := e.relay(t, "outbox-main")
		testutil.NoError(t, e.conn.Transaction(func(tx *gorm.DB) error {
			for _, m := range [][2]string{{"a", "1"}, {"b", "1"}, {"a", "2"}, {"b", "2"}, {"a", "3"}} {
				testutil.NoError(t, outbox.Add(tx, m[0], []byte(m[1]), nil))
			}
			return nil
		}))
		done := run(t, r)

		eventually(t, "first batch", func() bool {
			rows := e.rows(t)
			return rows[0].Attempts == 1 && rows[1].SentAt != nil && rows[3].SentAt != nil &&
				rows[2].LockedUntil == nil && rows[4].LockedUntil == nil
		})
		rows := e.rows(t)
		testutil.Equal(t, "NextAttemptAt", rows[0].NextAttemptAt.Equal(ts.Add(time.Second)), true)
		testutil.Equal(t, "LastError", rows[0].LastError, "unable to publish outbox message: unable to publish outbox message: refused")
		testutil.Equal(t, "LockedUntil", rows[0].LockedUntil, (*time.Time)(nil))
		testutil.Equal(t, "a held back", rows[2].SentAt == nil && rows[4].SentAt == nil, true)

		e.advance(t, 500*time.Millisecond)
		e.advance(t, 400*time.Millisecond)
		eventually(t, "waiting", func() bool { return e.clock.Timers() == 1 })
		testutil.Equal(t, "held back", len(e.fake.published()), 3)

		e.advance(t, 500*time.Millisecond)
		eventually(t, "all sent", func() bool {
			for _, row := range e.rows(t) {
				if row.SentAt == nil {
					return false
				}
			}
			return true
		})
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()),
			[]string{"a:1!", "b:1", "b:2", "a:1", "a:2", "a:3"})
		got := e.logged(t, "refused")
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogWarning, channel: outbox.LogChannel,
			message: "unable to publish outbox message: unable to publish outbox message: refused",
			ctx:     map[string]any{"id": "main", "message": rows[0].ID, "topic": "a", "broker": "fake"},
		})
	})

	t.Run("should double the backoff up to its maximum", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldBackoff: "1s", outbox.ConfigFieldMaxBackoff: "3s", outbox.ConfigFieldPoll: "1s"}})
		e.fake.fail = func(string, string) error { return errors.New("refused") }
		r := e.relay(t, "outbox-main")
		e.add(t, "orders", "1")
		done := run(t, r)
		elapsed := time.Duration(0)
		for attempt, delay := range []time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second} {
			eventually(t, "failed", func() bool { return e.rows(t)[0].Attempts == attempt+1 })
			testutil.Equal(t, "NextAttemptAt", e.rows(t)[0].NextAttemptAt.Equal(ts.Add(elapsed+delay)), true)
			for range delay / time.Second {
				e.advance(t, time.Second)
			}
			elapsed += delay
		}
		stop(t, r, done)
	})

	t.Run("should not claim the messages following one claimed by another relay", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldPoll: "1s"}})
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1", "2")
		e.add(t, "b", "1")
		until := ts.Add(1500 * time.Millisecond)
		testutil.NoError(t, e.conn.Model(&outbox.Row{}).Where("id = ?", e.rows(t)[0].ID).
			Updates(map[string]any{"token": "other", "locked_until": until}).Error)
		done := run(t, r)
		eventually(t, "b sent", func() bool { return e.rows(t)[2].SentAt != nil })
		testutil.Equal(t, "published", payloads(e.fake.published()), []string{"b:1"})

		e.advance(t, time.Second)
		e.advance(t, time.Second)
		eventually(t, "a sent", func() bool { return e.rows(t)[1].SentAt != nil })
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()), []string{"b:1", "a:1", "a:2"})
	})

	t.Run("should skip a topic whose message another relay claimed first", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldPoll: "1s"}})
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1", "2")
		var stolen atomic.Bool
		inject(t, e.conn, "update", "steal", func(db *gorm.DB) {
			if _, ok := updates(db)["token"]; ok && stolen.CompareAndSwap(false, true) {
				_ = db.Session(&gorm.Session{NewDB: true}).Exec("UPDATE flam_outbox SET token = 'stolen'").Error
			}
		})
		done := run(t, r)
		eventually(t, "polling", func() bool { return e.clock.Timers() == 1 })
		testutil.Equal(t, "published", len(e.fake.published()), 0)
		e.advance(t, time.Second)
		eventually(t, "sent", func() bool { return e.rows(t)[1].SentAt != nil })
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()), []string{"a:1", "a:2"})
	})

	t.Run("should log the failure to claim the messages", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1")
		inject(t, e.conn, "query", "fail", func(db *gorm.DB) { _ = db.AddError(errors.New("boom")) })
		done := run(t, r)
		got := e.logged(t, "boom")
		stop(t, r, done)
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogError, channel: outbox.LogChannel,
			message: "unable to claim outbox messages: unable to claim outbox messages: boom",
			ctx:     map[string]any{"id": "main"},
		})
	})

	t.Run("should log the failure to claim a message, skipping its topic", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1", "2")
		e.add(t, "b", "1")
		var failed atomic.Bool
		inject(t, e.conn, "update", "fail", func(db *gorm.DB) {
			if _, ok := updates(db)["token"]; ok && failed.CompareAndSwap(false, true) {
				_ = db.AddError(errors.New("boom"))
			}
		})
		done := run(t, r)
		got := e.logged(t, "boom")
		eventually(t, "b sent", func() bool { return e.rows(t)[2].SentAt != nil })
		stop(t, r, done)
		testutil.Equal(t, "published", payloads(e.fake.published()), []string{"b:1"})
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogError, channel: outbox.LogChannel,
			message: "unable to claim outbox messages: unable to claim outbox messages: boom",
			ctx:     map[string]any{"id": "main", "message": e.rows(t)[0].ID, "topic": "a"},
		})
	})

	t.Run("should retry the messages whose headers cannot be decoded", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		testutil.NoError(t, e.conn.Create(&outbox.Row{Topic: "a", Payload: []byte("1"), Headers: "{"}).Error)
		done := run(t, r)
		got := e.logged(t, "unable to decode outbox message headers")
		stop(t, r, done)
		testutil.Equal(t, "level", got.level, flam.LogWarning)
		testutil.Equal(t, "Attempts", e.rows(t)[0].Attempts, 1)
		testutil.Equal(t, "published", len(e.fake.published()), 0)
	})

	t.Run("should log the failure to record an outcome", func(t *testing.T) {
		for name, del := range map[string]bool{"update": false, "delete": true} {
			t.Run(name, func(t *testing.T) {
				e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldDelete: del, outbox.ConfigFieldCleanup: "1000h"}})
				r := e.relay(t, "outbox-main")
				e.add(t, "a", "1")
				var ran atomic.Bool
				fail := func(db *gorm.DB) {
					if ran.Load() {
						_ = db.AddError(errors.New("boom"))
					}
				}
				inject(t, e.conn, "update", "fail", func(db *gorm.DB) {
					if _, ok := updates(db)["sent_at"]; ok {
						fail(db)
					}
				})
				inject(t, e.conn, "delete", "fail", fail)
				ran.Store(true)
				done := run(t, r)
				got := e.logged(t, "unable to record outbox message outcome: boom")
				stop(t, r, done)
				testutil.Equal(t, "entry", got, entry{
					level: flam.LogError, channel: outbox.LogChannel,
					message: "unable to record outbox message outcome: unable to record outbox message outcome: boom",
					ctx:     map[string]any{"id": "main", "message": e.rows(t)[0].ID, "topic": "a"},
				})
			})
		}
	})

	t.Run("should log the loss of the lease of a message", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		e.fake.fail = func(string, string) error {
			return e.conn.Model(&outbox.Row{}).Where("1 = 1").Update("token", "other").Error
		}
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1")
		done := run(t, r)
		got := e.logged(t, "lease lost")
		stop(t, r, done)
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogWarning, channel: outbox.LogChannel,
			message: "unable to record outbox message outcome: outbox message lease lost",
			ctx:     map[string]any{"id": "main", "message": e.rows(t)[0].ID, "topic": "a"},
		})
		testutil.Equal(t, "SentAt", e.rows(t)[0].SentAt, (*time.Time)(nil))
	})

	t.Run("should delete the sent messages past their retention", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldRetention: "1h", outbox.ConfigFieldCleanup: "30m", outbox.ConfigFieldPoll: "30m"}})
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1")
		done := run(t, r)
		eventually(t, "sent", func() bool { return e.rows(t)[0].SentAt != nil })
		e.add(t, "a", "2")
		e.advance(t, 30*time.Minute)
		e.advance(t, 30*time.Minute)
		e.advance(t, 30*time.Minute)
		eventually(t, "deleted", func() bool { return len(e.rows(t)) == 1 })
		stop(t, r, done)
		testutil.Equal(t, "kept", string(e.rows(t)[0].Payload), "2")
	})

	t.Run("should log the failure to clean up", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		inject(t, e.conn, "delete", "fail", func(db *gorm.DB) { _ = db.AddError(errors.New("boom")) })
		done := run(t, r)
		got := e.logged(t, "boom")
		stop(t, r, done)
		testutil.Equal(t, "entry", got, entry{
			level: flam.LogError, channel: outbox.LogChannel,
			message: "unable to clean up outbox messages: unable to clean up outbox messages: boom",
			ctx:     map[string]any{"id": "main"},
		})
	})

	t.Run("should complete its batch when stopped", func(t *testing.T) {
		release := make(chan struct{})
		e := newEnv(t, options{relay: container.Bag{}})
		e.fake.fail = func(string, string) error { <-release; return nil }
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1", "2")
		done := run(t, r)
		eventually(t, "publishing", func() bool { return e.rows(t)[0].LockedUntil != nil })
		testutil.NoError(t, r.Close())
		close(release)
		select {
		case err := <-done:
			testutil.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not return")
		}
		for _, row := range e.rows(t) {
			testutil.Equal(t, "SentAt", row.SentAt != nil, true)
		}
	})

	t.Run("should stop when its context is done", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- r.Run(ctx) }()
		eventually(t, "running", r.IsRunning)
		cancel()
		select {
		case err := <-done:
			testutil.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("Run did not return")
		}
	})

	t.Run("should refuse to run twice at once", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		done := run(t, r)
		testutil.Error(t, r.Run(context.Background()), outbox.ErrRunning, "unable to run outbox relay: outbox relay running",
			map[string]any{"id": "main"})
		stop(t, r, done)
	})

	t.Run("should return at once once closed", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		testutil.NoError(t, r.Close())
		testutil.NoError(t, r.Close())
		var nilCtx context.Context
		testutil.NoError(t, r.Run(nilCtx))
	})

	t.Run("should run with a nil context", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		r := e.relay(t, "outbox-main")
		done := make(chan error, 1)
		var nilCtx context.Context
		go func() { done <- r.Run(nilCtx) }()
		eventually(t, "running", r.IsRunning)
		stop(t, r, done)
	})

	t.Run("should log nothing without logger", func(t *testing.T) {
		e := newEnv(t, options{noLogger: true, relay: container.Bag{}})
		e.fake.fail = func(string, string) error { return errors.New("refused") }
		r := e.relay(t, "outbox-main")
		e.add(t, "a", "1")
		done := run(t, r)
		eventually(t, "failed", func() bool { return e.rows(t)[0].Attempts == 1 })
		stop(t, r, done)
		testutil.Equal(t, "logged", e.logger.count(), 0)
	})
}
