package jobs

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog"
	"github.com/cjdias/flam-in-go/pkg/flam/watchdog/loggers"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// processes returns the processes of the watchdog process group of the
// app, or the error of their providers.
func processes(a *testutil.App) ([]flam.IWatchdogProcess, error) {
	type in struct {
		dig.In

		Processes []flam.IWatchdogProcess `group:"flam.watchdog.processes"`
	}
	var got []flam.IWatchdogProcess
	err := a.Invoke(func(args in) { got = args.Processes })
	return got, err
}

func Test_Declarations(t *testing.T) {
	for _, pair := range [][2]string{
		{jobs.QueueConfigPath, "flam.jobs.queues"},
		{jobs.QueueCreatorGroup, "flam.jobs.queues.creator"},
		{jobs.HandlerGroup, "flam.jobs.handlers"},
		{jobs.Driver, "flam.jobs.queues.driver.database"},
		{jobs.ConfigFieldConnection, "connection"},
		{jobs.ConfigFieldProcess, "process"},
		{jobs.ConfigFieldPoll, "poll"},
		{jobs.ConfigFieldConcurrency, "concurrency"},
		{jobs.ConfigFieldLease, "lease"},
		{jobs.ConfigFieldBackoff, "backoff"},
		{jobs.ConfigFieldMaxBackoff, "max_backoff"},
		{jobs.ConfigFieldMaxAttempts, "max_attempts"},
		{jobs.ConfigFieldRetention, "retention"},
		{jobs.ConfigFieldCleanup, "cleanup"},
		{jobs.ConfigFieldStopTimeout, "stop_timeout"},
		{jobs.ConfigFieldLocking, "locking"},
		{jobs.ConfigFieldMigrate, "migrate"},
		{jobs.ConfigFieldBroker, "broker"},
		{jobs.ConfigFieldTopic, "topic"},
		{jobs.DefaultConnectionConfigPath, "flam.jobs.defaults.queues.connection"},
		{jobs.DefaultPollConfigPath, "flam.jobs.defaults.queues.poll"},
		{jobs.DefaultConcurrencyConfigPath, "flam.jobs.defaults.queues.concurrency"},
		{jobs.DefaultLeaseConfigPath, "flam.jobs.defaults.queues.lease"},
		{jobs.DefaultBackoffConfigPath, "flam.jobs.defaults.queues.backoff"},
		{jobs.DefaultMaxBackoffConfigPath, "flam.jobs.defaults.queues.max_backoff"},
		{jobs.DefaultMaxAttemptsConfigPath, "flam.jobs.defaults.queues.max_attempts"},
		{jobs.DefaultRetentionConfigPath, "flam.jobs.defaults.queues.retention"},
		{jobs.DefaultCleanupConfigPath, "flam.jobs.defaults.queues.cleanup"},
		{jobs.DefaultStopTimeoutConfigPath, "flam.jobs.defaults.queues.stop_timeout"},
		{jobs.DefaultLockingConfigPath, "flam.jobs.defaults.queues.locking"},
		{jobs.DefaultMigrateConfigPath, "flam.jobs.defaults.queues.migrate"},
		{jobs.DefaultBrokerConfigPath, "flam.jobs.defaults.queues.broker"},
		{jobs.LockingAuto, "auto"},
		{jobs.LockingSkipLocked, "skip_locked"},
		{jobs.LockingNone, "none"},
		{jobs.Table, "flam_jobs"},
		{jobs.StatePending, "pending"},
		{jobs.StateRunning, "running"},
		{jobs.StateDone, "done"},
		{jobs.StateDead, "dead"},
		{jobs.LogChannel, "flam.jobs"},
		{jobs.ErrorFieldID, "id"},
		{jobs.ErrorFieldField, "field"},
		{jobs.ErrorFieldConnection, "connection"},
		{jobs.ErrorFieldBroker, "broker"},
		{jobs.ErrorFieldKind, "kind"},
		{jobs.ErrorFieldJob, "job"},
		{jobs.ErrorFieldAttempt, "attempt"},
		{jobs.ErrorFieldPanic, "panic"},
		{jobs.ErrorFieldStack, "stack"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("constant = %q, want %q", pair[0], pair[1])
		}
	}
	testutil.Equal(t, "TableName()", jobs.Row{}.TableName(), "flam_jobs")
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the factory, the creator and the processes", func(t *testing.T) {
		b := jobs.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.jobs")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.jobs", "provider/0", "provider/1", "provider/1"})
	})

	t.Run("should fail when the registration fails", func(t *testing.T) {
		a := testutil.NewApp()
		a.Fail = func(kind string) error {
			if kind == "provider" {
				return errors.New("refused")
			}
			return nil
		}
		if err := a.RegisterBundle(jobs.NewBundle()); err == nil {
			t.Fatal("RegisterBundle() did not fail")
		}
	})

	t.Run("should provide the workers of the queues as processes", func(t *testing.T) {
		e := newEnv(t, options{values: map[string]any{
			jobs.QueueConfigPath + ".b": container.Bag{},
			jobs.QueueConfigPath + ".a": container.Bag{jobs.ConfigFieldProcess: "custom"},
		}})
		got, err := processes(e.app)
		testutil.NoError(t, err)
		byId := map[string]flam.IWatchdogProcess{}
		for _, p := range got {
			byId[p.Id()] = p
		}
		testutil.Equal(t, "processes", len(byId), 2)
		for id, process := range map[string]string{"a": "custom", "b": "jobs-b"} {
			q, err := e.queues(t).Get(id)
			testutil.NoError(t, err)
			testutil.Equal(t, "Worker()", q.Worker() == byId[process], true)
		}
	})

	t.Run("should fail to provide the processes of an invalid queue", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{jobs.ConfigFieldPoll: "0s"}})
		_, err := processes(e.app)
		if !errors.Is(err, jobs.ErrInvalidField) {
			t.Fatalf("processes error = %v, want ErrInvalidField", err)
		}
	})

	t.Run("should run the workers in the kennel", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}, setups: []func(a *testutil.App){
			func(a *testutil.App) {
				testutil.NoError(t, a.RegisterBundle(watchdog.NewBundle()))
				testutil.NoError(t, a.RegisterBundle(loggers.NewBundle()))
			},
		}})
		testutil.NoError(t, e.app.Invoke(func(k flam.IKennel) {
			testutil.Equal(t, "Available()", k.Available(), []string{"jobs-main"})
		}))
	})

	t.Run("should close the queue factory", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{}})
		q := e.queue(t)
		testutil.NoError(t, jobs.NewBundle().(flam.IClosableBundle).Close(e.app))
		_, err := q.Enqueue(context.Background(), "kind", nil)
		testutil.Error(t, err, jobs.ErrClosed, "unable to enqueue job: job queue closed",
			map[string]any{"id": "main", "kind": "kind"})
	})

	t.Run("should fail to close without a configuration", func(t *testing.T) {
		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(jobs.NewBundle()))
		if err := jobs.NewBundle().(flam.IClosableBundle).Close(a); err == nil {
			t.Fatal("Close() did not fail")
		}
	})

	t.Run("should refuse the configurations of another driver", func(t *testing.T) {
		e := newEnv(t, options{queue: container.Bag{factory.ConfigFieldDriver: "other"}})
		_, err := e.queues(t).Get("main")
		if !errors.Is(err, factory.ErrNoCreator) {
			t.Fatalf("Get() = %v, want ErrNoCreator", err)
		}
	})
}
