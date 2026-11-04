package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// assertCause fails the test unless err wraps the sentinel and holds the
// context.
func assertCause(t *testing.T, err error, sentinel error, ctx map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	testutil.Context(t, e, ctx)
}

func Test_Creator(t *testing.T) {
	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, value := range map[string]any{
			jobs.ConfigFieldConnection:  "",
			jobs.ConfigFieldProcess:     "",
			jobs.ConfigFieldTopic:       1,
			jobs.ConfigFieldPoll:        "0s",
			jobs.ConfigFieldConcurrency: 0,
			jobs.ConfigFieldLease:       "-1s",
			jobs.ConfigFieldBackoff:     "x",
			jobs.ConfigFieldMaxBackoff:  "1ms",
			jobs.ConfigFieldMaxAttempts: 0,
			jobs.ConfigFieldRetention:   "-1s",
			jobs.ConfigFieldCleanup:     "0s",
			jobs.ConfigFieldStopTimeout: true,
			jobs.ConfigFieldLocking:     "other",
			jobs.ConfigFieldMigrate:     "yes",
			jobs.ConfigFieldBroker:      1,
		} {
			e := newEnv(t, options{queue: container.Bag{field: value}})
			_, err := e.queues(t).Get("main")
			assertCause(t, errors.Unwrap(err), jobs.ErrInvalidField, map[string]any{"id": "main", "field": field})
		}
	})

	t.Run("should take the missing fields from their defaults", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}, values: map[string]any{jobs.DefaultMaxAttemptsConfigPath: 7}})
		id := enqueue(t, e.queue(t), "kind", nil)
		testutil.Equal(t, "MaxAttempts", e.row(t, id).MaxAttempts, 7)
	})

	t.Run("should allow five attempts unless configured", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		id := enqueue(t, e.queue(t), "kind", nil)
		testutil.Equal(t, "MaxAttempts", e.row(t, id).MaxAttempts, 5)
	})

	t.Run("should refuse a nil handler", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{nil}})
		_, err := e.queues(t).Get("main")
		testutil.Error(t, errors.Unwrap(err), jobs.ErrNilHandler, "unable to create job queue: nil job handler",
			map[string]any{"id": "main"})
	})

	t.Run("should refuse two handlers of a kind", func(t *testing.T) {
		h := &handler{kind: "kind"}
		e := newEnv(t, options{queue: container.Bag{}, handlers: []flam.IJobHandler{h, h}})
		_, err := e.queues(t).Get("main")
		testutil.Error(t, errors.Unwrap(err), jobs.ErrDuplicateKind,
			"unable to create job queue: duplicate job handler kind", map[string]any{"id": "main", "kind": "kind"})
	})

	t.Run("should fail with an unknown connection", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldConnection: "missing"}})
		_, err := e.queues(t).Get("main")
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrUnknownID", err)
		}
		assertCause(t, errors.Unwrap(err), jobs.ErrConnection, map[string]any{"id": "main", "connection": "missing"})
	})

	t.Run("should fail without the database bundle", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}, bundles: []flam.IBundle{config.NewBundle(), jobs.NewBundle()}})
		_, err := e.queues(t).Get("main")
		testutil.Error(t, errors.Unwrap(err), jobs.ErrConnection, "unable to create job queue: unable to get job queue connection: "+
			"no database connection factory, register the database bundle", map[string]any{"id": "main", "connection": "main"})
	})

	t.Run("should fail with an unknown broker", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "missing"}})
		_, err := e.queues(t).Get("main")
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("Get() = %v, want ErrUnknownID", err)
		}
		assertCause(t, errors.Unwrap(err), jobs.ErrBroker, map[string]any{"id": "main", "broker": "missing"})
	})

	t.Run("should fail without the broker bundle", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldBroker: "events"}, bundles: []flam.IBundle{
			config.NewBundle(), database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(), jobs.NewBundle(),
		}})
		_, err := e.queues(t).Get("main")
		testutil.Error(t, errors.Unwrap(err), jobs.ErrBroker, "unable to create job queue: unable to get job queue broker: "+
			"no broker factory, register the broker bundle", map[string]any{"id": "main", "broker": "events"})
	})

	t.Run("should fail when the table cannot be migrated", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		testutil.NoError(t, e.conn.Exec("CREATE VIEW flam_jobs AS SELECT 1 AS id").Error)
		_, err := e.queues(t).Get("main")
		assertCause(t, errors.Unwrap(err), jobs.ErrMigrate, map[string]any{"id": "main", "connection": "main"})
	})

	t.Run("should not migrate the table when told not to", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldMigrate: false}})
		_, err := e.queue(t).Enqueue(context.Background(), "kind", nil)
		assertCause(t, err, jobs.ErrEnqueue, map[string]any{"id": "main", "kind": "kind"})
	})
}
