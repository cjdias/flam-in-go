package jobs

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/dig"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/broker"
	"github.com/cjdias/flam-in-go/pkg/flam/broker/brokers"
	brokerredis "github.com/cjdias/flam-in-go/pkg/flam/broker/brokers/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/clock"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/jobs"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/mini"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// ts is the time of the fake clocks.
var ts = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// entry is an entry signaled to a logger.
type entry struct {
	level   flam.LogLevel
	channel string
	message string
	ctx     map[string]any
}

// logger is a flam.ILogger recording the signaled entries.
type logger struct {
	flam.ILogger

	mu      sync.Mutex
	entries []entry
}

func (l *logger) Signal(level flam.LogLevel, channel, message string, ctx ...flam.IBag) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry{level: level, channel: channel, message: message, ctx: map[string]any(ctx[0].(container.Bag))})
}

// find returns the first entry whose message holds the text, and whether
// there is one.
func (l *logger) find(text string) (entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if strings.Contains(e.message, text) {
			return e, true
		}
	}
	return entry{}, false
}

// count returns the number of entries.
func (l *logger) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.entries)
}

// handler is a flam.IJobHandler calling a function.
type handler struct {
	kind string
	fn   func(ctx context.Context, job flam.IJob) error
}

func (h *handler) Kind() string { return h.kind }

func (h *handler) Handle(ctx context.Context, job flam.IJob) error { return h.fn(ctx, job) }

// env is an application with the jobs feature over a SQLite file, a
// mini redis broker "events", a fake clock unless real, and a logger.
type env struct {
	app    *testutil.App
	clock  *testutil.Clock
	logger *logger
	conn   flam.IDatabaseConnection
}

// options are the options of newEnv.
type options struct {
	real     bool          // reads the system clock rather than a fake
	noLogger bool          // provides no logger
	queue    container.Bag // the fields of the queue "main", none when nil
	values   map[string]any
	handlers []flam.IJobHandler
	bundles  []flam.IBundle // registered in place of the default ones
	setups   []func(a *testutil.App)
}

// newEnv returns the env of the options.
func newEnv(t *testing.T, o options) *env {
	t.Helper()
	bundles := o.bundles
	if bundles == nil {
		bundles = []flam.IBundle{
			config.NewBundle(), database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(),
			redis.NewBundle(), mini.NewBundle(), broker.NewBundle(), brokers.NewBundle(), jobs.NewBundle(),
		}
	}
	e := &env{app: testutil.NewApp(), logger: &logger{}}
	if o.real {
		bundles = append(bundles, clock.NewBundle())
	} else {
		e.clock = testutil.NewClock(ts)
		testutil.NoError(t, e.app.Provide(func() flam.IClock { return e.clock }))
	}
	if !o.noLogger {
		testutil.NoError(t, e.app.Provide(func() flam.ILogger { return e.logger }))
	}
	for _, h := range o.handlers {
		testutil.NoError(t, e.app.Provide(func() flam.IJobHandler { return h }, dig.Group(jobs.HandlerGroup)))
	}

	bag := container.Bag{}
	for _, b := range bundles {
		if configurable, ok := b.(flam.IConfigurableBundle); ok {
			testutil.NoError(t, configurable.Config(bag))
		}
	}
	defaults := map[string]any{
		database.DialectConfigPath + ".main": container.Bag{
			factory.ConfigFieldDriver: sqlite.Driver,
			sqlite.ConfigFieldPath:    filepath.Join(t.TempDir(), "app.db"),
			sqlite.ConfigFieldParams:  container.Bag{"_busy_timeout": 10000, "_journal_mode": "WAL"},
		},
		database.ConnectionConfigPath + ".main": container.Bag{database.ConfigFieldDialect: "main"},
		redis.ConnectionConfigPath + ".default": container.Bag{factory.ConfigFieldDriver: mini.Driver},
		broker.BrokerConfigPath + ".events":     container.Bag{factory.ConfigFieldDriver: brokerredis.Driver},
		jobs.DefaultConnectionConfigPath:        "main",
	}
	for path, value := range defaults {
		testutil.NoError(t, bag.Set(path, value))
	}
	if o.queue != nil {
		testutil.NoError(t, bag.Set(jobs.QueueConfigPath+".main", o.queue))
	}
	for path, value := range o.values {
		testutil.NoError(t, bag.Set(path, value))
	}

	for _, b := range bundles {
		testutil.NoError(t, e.app.RegisterBundle(b))
	}
	for _, setup := range o.setups {
		setup(e.app)
	}
	testutil.NoError(t, e.app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	t.Cleanup(func() {
		_ = e.app.Invoke(func(f flam.IJobQueueFactory) error { return f.Close() })
		_ = e.app.Invoke(func(f flam.IBrokerFactory) error { return f.Close() })
		_ = e.app.Invoke(func(f flam.IDatabaseConnectionFactory) error { return f.Close() })
		_ = e.app.Invoke(func(f flam.IRedisConnectionFactory) error { return f.Close() })
	})
	_ = e.app.Invoke(func(f flam.IDatabaseConnectionFactory) (err error) {
		e.conn, err = f.Get("main")
		return err
	})
	return e
}

// queues returns the queue factory of the env.
func (e *env) queues(t *testing.T) flam.IJobQueueFactory {
	t.Helper()
	var f flam.IJobQueueFactory
	testutil.NoError(t, e.app.Invoke(func(provided flam.IJobQueueFactory) { f = provided }))
	return f
}

// queue returns the queue "main" of the env.
func (e *env) queue(t *testing.T) flam.IJobQueue {
	t.Helper()
	q, err := e.queues(t).Get("main")
	testutil.NoError(t, err)
	return q
}

// brokers returns the broker factory of the env.
func (e *env) brokers(t *testing.T) flam.IBrokerFactory {
	t.Helper()
	var f flam.IBrokerFactory
	testutil.NoError(t, e.app.Invoke(func(provided flam.IBrokerFactory) { f = provided }))
	return f
}

// row returns the row of the job id.
func (e *env) row(t *testing.T, id string) jobs.Row {
	t.Helper()
	var row jobs.Row
	testutil.NoError(t, e.conn.Where("id = ?", id).Take(&row).Error)
	return row
}

// rows returns the number of rows of the job table.
func (e *env) rows(t *testing.T) int64 {
	t.Helper()
	var n int64
	testutil.NoError(t, e.conn.Model(&jobs.Row{}).Count(&n).Error)
	return n
}

// enqueue enqueues the job on the queue, failing the test on error.
func enqueue(t *testing.T, q flam.IJobQueue, kind string, payload any, opts ...flam.JobOption) string {
	t.Helper()
	id, err := q.Enqueue(context.Background(), kind, payload, opts...)
	testutil.NoError(t, err)
	return id
}

// run runs the process in a goroutine, returning the channel of the error
// of its Run, and closes it at the end of the test.
func run(t *testing.T, p flam.IWatchdogProcess) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- p.Run(context.Background()) }()
	eventually(t, "running", p.IsRunning)
	t.Cleanup(func() { _ = p.Close() })
	return done
}

// stop closes the process and waits for its Run to return nil.
func stop(t *testing.T, p flam.IWatchdogProcess, done <-chan error) {
	t.Helper()
	testutil.NoError(t, p.Close())
	select {
	case err := <-done:
		testutil.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
	}
}

// eventually fails the test unless the condition holds within 10s.
func eventually(t *testing.T, name string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: condition not met in time", name)
		}
		time.Sleep(time.Millisecond)
	}
}

// advance waits until the clock has the timers, then advances it.
func (e *env) advance(t *testing.T, timers int, d time.Duration) {
	t.Helper()
	eventually(t, "timers", func() bool { return e.clock.Timers() >= timers })
	e.clock.Advance(d)
}

// state returns the condition of the job holding the state.
func (e *env) state(t *testing.T, id string, state string) func() bool {
	return func() bool {
		var row jobs.Row
		err := e.conn.Where("id = ?", id).Take(&row).Error
		return err == nil && row.State == state
	}
}

// inject registers the gorm callback, before the statements of the kind
// ("create", "query", "update", "delete" or "raw"), of the connection.
func inject(t *testing.T, conn flam.IDatabaseConnection, kind string, name string, fn func(db *gorm.DB)) {
	t.Helper()
	callbacks := conn.Gorm().Callback()
	var err error
	switch kind {
	case "create":
		err = callbacks.Create().Before("gorm:create").Register(name, fn)
	case "query":
		err = callbacks.Query().Before("gorm:query").Register(name, fn)
	case "update":
		err = callbacks.Update().Before("gorm:update").Register(name, fn)
	case "delete":
		err = callbacks.Delete().Before("gorm:delete").Register(name, fn)
	default:
		err = callbacks.Raw().Before("gorm:raw").Register(name, fn)
	}
	testutil.NoError(t, err)
}

// updates returns the values of the update statement, nil for another
// one.
func updates(db *gorm.DB) map[string]any {
	values, _ := db.Statement.Dest.(map[string]any)
	return values
}
