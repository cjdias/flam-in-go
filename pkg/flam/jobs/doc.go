// Package jobs is the background job feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IJobQueueFactory] of an
// application, which creates the queues described by the configuration at
// [QueueConfigPath], storing their jobs in the [Table] table of a database
// connection of [github.com/cjdias/flam-in-go/pkg/flam/database], such as:
//
//	flam:
//	  jobs:
//	    queues:
//	      default: {connection: main, concurrency: 4}
//	      mails:   {connection: main, poll: 5s, max_attempts: 10, broker: events}
//
// The application enqueues a job by kind, with a payload encoded as JSON,
// and options such as [WithDelay], [WithMaxAttempts], [WithKey] or
// [WithTx], the latter enqueuing it with the transaction of the caller:
//
//	id, err := queue.Enqueue(ctx, "mail.welcome", Welcome{UserId: 7}, jobs.WithDelay(time.Minute))
//
// A job enqueued with a transaction is not visible to the workers before
// the transaction commits, so that Enqueue does not notify them: the
// caller calls the Notify method of the queue once it commits, the
// workers finding the job at their next poll otherwise:
//
//	err := conn.Transaction(func(tx *gorm.DB) error {
//		_, err := queue.Enqueue(ctx, "mail.welcome", Welcome{UserId: 7}, jobs.WithTx(tx))
//		return err
//	})
//	if err == nil {
//		queue.Notify(ctx)
//	}
//
// and provides a [github.com/cjdias/flam-in-go/pkg/flam.IJobHandler] per
// kind in the [HandlerGroup] container group.
//
// The worker of a queue, its Worker method, is a process of the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], provided in its
// process group under the id "jobs-" followed by the queue id, unless set
// at [ConfigFieldProcess]. While it runs, it claims the jobs whose run
// time is reached, in run time order, up to [ConfigFieldConcurrency] at
// once, and polls again once idle, every [ConfigFieldPoll] or when a job
// is enqueued by the queue, or by another process when the queue has a
// [ConfigFieldBroker]: Enqueue, or Notify, then publishes a wakeup
// notification on its [ConfigFieldTopic], which the worker subscribes to. The notifications
// only shorten the wait, as the polls find the jobs anyway.
//
// A worker claims a job by setting its state to [StateRunning] with a new
// claim token and a lease of [ConfigFieldLease], in a conditional update
// failing when another worker claimed it first, the selection being made
// with "FOR UPDATE SKIP LOCKED" on PostgreSQL and MySQL, as set at
// [ConfigFieldLocking]. A worker crashing leaves its jobs running until
// their lease expires, after which another worker claims them again, the
// crashed attempt counting as one: a job may thus run more than once, its
// handler having to be idempotent, and the lease must exceed the longest
// run of a job. With SQLite, which serializes the writes, set a busy
// timeout on the connection, such as the "_busy_timeout" parameter.
//
// A job whose handler returns nil is [StateDone], kept for
// [ConfigFieldRetention], along with its deduplication key, and deleted
// afterwards, every [ConfigFieldCleanup]. A job whose handler fails, or
// panics, or of a kind no handler handles, runs again after a backoff of
// [ConfigFieldBackoff], doubled at each attempt up to
// [ConfigFieldMaxBackoff], until it ran its maximum number of attempts,
// after which it is [StateDead], kept with its last error until the
// application deletes it. When the worker stops, it claims no more jobs,
// waits for the ones under way, and makes their context done once
// [ConfigFieldStopTimeout] elapses.
//
// The table is created or updated by the queues when created, with gorm
// AutoMigrate, unless [ConfigFieldMigrate] is false, in which case the
// application creates it, with [Migrate] or the migration of
// [NewMigration]. The failures of the workers are logged on [LogChannel]
// when the container provides a
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger].
package jobs
