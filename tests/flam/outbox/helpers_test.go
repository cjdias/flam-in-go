package outbox

import (
	"context"
	"maps"
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
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/outbox"
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

// published is a message published with a fake broker.
type published struct {
	topic   string
	payload string
	headers map[string]string
	err     bool
}

// fakeBroker is a flam.IBroker recording the published messages, and
// failing them while fail returns an error.
type fakeBroker struct {
	flam.IBroker

	fail func(topic string, payload string) error

	mu       sync.Mutex
	messages []published
}

func (b *fakeBroker) Id() string { return "fake" }

func (b *fakeBroker) Close() error { return nil }

func (b *fakeBroker) Publish(_ context.Context, topic string, payload []byte, headers map[string]string) error {
	var err error
	if b.fail != nil {
		err = b.fail(topic, string(payload))
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.messages = append(b.messages, published{topic: topic, payload: string(payload), headers: maps.Clone(headers), err: err != nil})
	return err
}

// published returns the messages published so far.
func (b *fakeBroker) published() []published {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]published(nil), b.messages...)
}

// env is an application with the outbox feature over a SQLite file, a
// mini redis broker "events", a fake broker "fake", a fake clock and a
// logger.
type env struct {
	app    *testutil.App
	clock  *testutil.Clock
	logger *logger
	conn   flam.IDatabaseConnection
	fake   *fakeBroker
}

// options are the options of newEnv.
type options struct {
	noLogger bool
	relay    container.Bag // the fields of the relay "main", none when nil
	values   map[string]any
	bundles  []flam.IBundle // registered in place of the default ones
}

// newEnv returns the env of the options.
func newEnv(t *testing.T, o options) *env {
	t.Helper()
	bundles := o.bundles
	if bundles == nil {
		bundles = []flam.IBundle{
			config.NewBundle(), database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(),
			redis.NewBundle(), mini.NewBundle(), broker.NewBundle(), brokers.NewBundle(), outbox.NewBundle(),
		}
	}
	e := &env{app: testutil.NewApp(), logger: &logger{}, clock: testutil.NewClock(ts), fake: &fakeBroker{}}
	testutil.NoError(t, e.app.Provide(func() flam.IClock { return e.clock }))
	if !o.noLogger {
		testutil.NoError(t, e.app.Provide(func() flam.ILogger { return e.logger }))
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
		outbox.DefaultConnectionConfigPath:      "main",
		outbox.DefaultBrokerConfigPath:          "fake",
	}
	for path, value := range defaults {
		testutil.NoError(t, bag.Set(path, value))
	}
	if o.relay != nil {
		testutil.NoError(t, bag.Set(outbox.RelayConfigPath+".main", o.relay))
	}
	for path, value := range o.values {
		testutil.NoError(t, bag.Set(path, value))
	}

	for _, b := range bundles {
		testutil.NoError(t, e.app.RegisterBundle(b))
	}
	testutil.NoError(t, e.app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	_ = e.app.Invoke(func(f flam.IBrokerFactory) error { return f.Store("fake", e.fake) })
	t.Cleanup(func() {
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

// relay returns the relay process of the id of the env.
func (e *env) relay(t *testing.T, id string) flam.IWatchdogProcess {
	t.Helper()
	got, err := processes(e.app)
	testutil.NoError(t, err)
	for _, p := range got {
		if p.Id() == id {
			return p
		}
	}
	t.Fatalf("no process %q", id)
	return nil
}

// add adds the messages of the topic and payloads in a transaction.
func (e *env) add(t *testing.T, topic string, payloads ...string) {
	t.Helper()
	testutil.NoError(t, e.conn.Transaction(func(tx *gorm.DB) error {
		for _, payload := range payloads {
			if err := outbox.Add(tx, topic, []byte(payload), nil); err != nil {
				return err
			}
		}
		return nil
	}))
}

// rows returns the rows of the outbox table, in id order.
func (e *env) rows(t *testing.T) []outbox.Row {
	t.Helper()
	var rows []outbox.Row
	testutil.NoError(t, e.conn.Order("id").Find(&rows).Error)
	return rows
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

// advance waits until the clock has a timer, then advances it.
func (e *env) advance(t *testing.T, d time.Duration) {
	t.Helper()
	eventually(t, "timers", func() bool { return e.clock.Timers() >= 1 })
	e.clock.Advance(d)
}

// logged waits for the entry whose message holds the text, and returns
// it.
func (e *env) logged(t *testing.T, text string) entry {
	t.Helper()
	eventually(t, "logged "+text, func() bool { _, ok := e.logger.find(text); return ok })
	got, _ := e.logger.find(text)
	return got
}

// inject registers the gorm callback, before the statements of the kind
// ("query", "update" or "delete"), of the connection.
func inject(t *testing.T, conn flam.IDatabaseConnection, kind string, name string, fn func(db *gorm.DB)) {
	t.Helper()
	callbacks := conn.Gorm().Callback()
	var err error
	switch kind {
	case "query":
		err = callbacks.Query().Before("gorm:query").Register(name, fn)
	case "update":
		err = callbacks.Update().Before("gorm:update").Register(name, fn)
	default:
		err = callbacks.Delete().Before("gorm:delete").Register(name, fn)
	}
	testutil.NoError(t, err)
}

// updates returns the values of the update statement, nil for another
// one.
func updates(db *gorm.DB) map[string]any {
	values, _ := db.Statement.Dest.(map[string]any)
	return values
}
