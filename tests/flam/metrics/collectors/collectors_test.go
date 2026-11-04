package collectors

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	promtestutil "github.com/prometheus/client_golang/prometheus/testutil"
	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/database"
	"github.com/cjdias/flam-in-go/pkg/flam/metrics/collectors/redis"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

var errBoom = errors.New("boom")

// connector is a database connector that never connects.
type connector struct{}

func (connector) Connect(context.Context) (driver.Conn, error) { return nil, errBoom }

func (connector) Driver() driver.Driver { return nil }

// dbConnection is a database connection of the pool, or failing with err
// to give it.
type dbConnection struct {
	flam.IDatabaseConnection

	db  *sql.DB
	err error
}

func (c *dbConnection) DB() (*sql.DB, error) { return c.db, c.err }

// dbFactory is a database connection factory of the connections, failing
// with err to give one.
type dbFactory struct {
	flam.IDatabaseConnectionFactory

	connections map[string]flam.IDatabaseConnection
	available   []string
	err         error
	got         []string
}

func (f *dbFactory) Available() []string { return f.available }

func (f *dbFactory) Get(id string) (flam.IDatabaseConnection, error) {
	f.got = append(f.got, id)
	if f.err != nil {
		return nil, f.err
	}
	return f.connections[id], nil
}

// redisConnection is a Redis connection of the pool statistics.
type redisConnection struct {
	flam.IRedisConnection

	stats *goredis.PoolStats
}

func (c *redisConnection) PoolStats() *goredis.PoolStats { return c.stats }

// redisFactory is a Redis connection factory of the connections, failing
// with err to give one.
type redisFactory struct {
	flam.IRedisConnectionFactory

	connections map[string]flam.IRedisConnection
	available   []string
	err         error
	got         []string
}

func (f *redisFactory) Available() []string { return f.available }

func (f *redisFactory) Get(id string) (flam.IRedisConnection, error) {
	f.got = append(f.got, id)
	if f.err != nil {
		return nil, f.err
	}
	return f.connections[id], nil
}

// newDB returns a pool that never connects, of at most max open
// connections.
func newDB(t *testing.T, max int) *sql.DB {
	t.Helper()
	db := sql.OpenDB(connector{})
	db.SetMaxOpenConns(max)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// gather returns the gatherer of an app with the config, the metrics and
// the collector bundles, the provider when not nil, and the configuration
// holding the values, or the error of its creation.
func gather(t *testing.T, b flam.IBundle, provider any, values map[string]any) (prometheus.Gatherer, error) {
	t.Helper()
	app := testutil.NewApp()
	if provider != nil {
		testutil.NoError(t, app.Provide(provider))
	}
	for _, bundle := range []flam.IBundle{config.NewBundle(), metrics.NewBundle(), b} {
		testutil.NoError(t, app.RegisterBundle(bundle))
	}
	bag := container.Bag{}
	testutil.NoError(t, bag.Set(metrics.GoCollectorConfigPath, false))
	testutil.NoError(t, bag.Set(metrics.ProcessCollectorConfigPath, false))
	for path, value := range values {
		testutil.NoError(t, bag.Set(path, value))
	}
	testutil.NoError(t, app.Invoke(func(c flam.IConfig) error { return c.AddDefaults(bag) }))
	var gatherer prometheus.Gatherer
	err := app.Invoke(func(g prometheus.Gatherer) { gatherer = g })
	return gatherer, err
}

// flamError returns the *flam.Error of the error of the container,
// failing the test when it holds none.
func flamError(t *testing.T, err error) *flam.Error {
	t.Helper()
	var e *flam.Error
	if !errors.As(err, &e) {
		t.Fatalf("error = %v, want a *flam.Error", err)
	}
	return e
}

// count returns the number of metrics of the names gathered.
func count(t *testing.T, gatherer prometheus.Gatherer, names ...string) int {
	t.Helper()
	n, err := promtestutil.GatherAndCount(gatherer, names...)
	testutil.NoError(t, err)
	return n
}

func Test_Declarations(t *testing.T) {
	for name, tc := range map[string][2]string{
		"database.ConnectionsConfigPath": {database.ConnectionsConfigPath, "flam.metrics.collectors.database.connections"},
		"database.ErrorFieldID":          {database.ErrorFieldID, "id"},
		"database.ErrorFieldPath":        {database.ErrorFieldPath, "path"},
		"redis.ConnectionsConfigPath":    {redis.ConnectionsConfigPath, "flam.metrics.collectors.redis.connections"},
		"redis.ErrorFieldID":             {redis.ErrorFieldID, "id"},
		"redis.ErrorFieldPath":           {redis.ErrorFieldPath, "path"},
		"redis.LabelConnection":          {redis.LabelConnection, "db_name"},
		"redis.MetricHits":               {redis.MetricHits, "go_redis_pool_hits_total"},
		"redis.MetricMisses":             {redis.MetricMisses, "go_redis_pool_misses_total"},
		"redis.MetricTimeouts":           {redis.MetricTimeouts, "go_redis_pool_timeouts_total"},
		"redis.MetricWaits":              {redis.MetricWaits, "go_redis_pool_waits_total"},
		"redis.MetricWaitDuration":       {redis.MetricWaitDuration, "go_redis_pool_wait_duration_seconds_total"},
		"redis.MetricUnusable":           {redis.MetricUnusable, "go_redis_pool_unusable_total"},
		"redis.MetricStaleConnections":   {redis.MetricStaleConnections, "go_redis_pool_stale_connections_total"},
		"redis.MetricConnections":        {redis.MetricConnections, "go_redis_pool_connections"},
		"redis.MetricIdleConnections":    {redis.MetricIdleConnections, "go_redis_pool_idle_connections"},
		"redis.MetricPendingRequests":    {redis.MetricPendingRequests, "go_redis_pool_pending_requests"},
	} {
		testutil.Equal(t, name, tc[0], tc[1])
	}
	for name, err := range map[string]error{
		"database.ErrInvalidConnections": database.ErrInvalidConnections,
		"database.ErrMissingFactory":     database.ErrMissingFactory,
		"database.ErrConnection":         database.ErrConnection,
		"redis.ErrInvalidConnections":    redis.ErrInvalidConnections,
		"redis.ErrMissingFactory":        redis.ErrMissingFactory,
		"redis.ErrConnection":            redis.ErrConnection,
	} {
		if err == nil {
			t.Errorf("%s = nil", name)
		}
	}
}

func Test_Bundles(t *testing.T) {
	for id, b := range map[string]flam.IBundle{
		"flam.metrics.collectors.database": database.NewBundle(),
		"flam.metrics.collectors.redis":    redis.NewBundle(),
	} {
		t.Run("should register the collectors provider of "+id, func(t *testing.T) {
			testutil.Equal(t, "Id()", b.Id(), id)
			app := testutil.NewApp()
			testutil.NoError(t, app.RegisterBundle(b))
			testutil.Equal(t, "calls", app.Calls(), []string{"bundle:" + id, "provider/1"})
		})
	}
}

func Test_Database(t *testing.T) {
	t.Run("should collect the statistics of every configured connection", func(t *testing.T) {
		f := &dbFactory{
			connections: map[string]flam.IDatabaseConnection{
				"a": &dbConnection{db: newDB(t, 3)},
				"b": &dbConnection{db: newDB(t, 5)},
			},
			available: []string{"a", "b"},
		}
		gatherer, err := gather(t, database.NewBundle(), func() flam.IDatabaseConnectionFactory { return f }, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "got", f.got, []string{"a", "b"})
		testutil.NoError(t, promtestutil.GatherAndCompare(gatherer, strings.NewReader(`
# HELP go_sql_max_open_connections Maximum number of open connections to the database.
# TYPE go_sql_max_open_connections gauge
go_sql_max_open_connections{db_name="a"} 3
go_sql_max_open_connections{db_name="b"} 5
`), "go_sql_max_open_connections"))
		testutil.Equal(t, "open connections", count(t, gatherer, "go_sql_open_connections"), 2)
	})

	t.Run("should collect the statistics of the listed connections only", func(t *testing.T) {
		f := &dbFactory{
			connections: map[string]flam.IDatabaseConnection{"b": &dbConnection{db: newDB(t, 5)}},
			available:   []string{"a", "b"},
		}
		gatherer, err := gather(t, database.NewBundle(), func() flam.IDatabaseConnectionFactory { return f },
			map[string]any{database.ConnectionsConfigPath: []any{"b"}})
		testutil.NoError(t, err)
		testutil.Equal(t, "got", f.got, []string{"b"})
		testutil.Equal(t, "max open connections", count(t, gatherer, "go_sql_max_open_connections"), 1)
	})

	t.Run("should collect nothing without factory nor listed connections", func(t *testing.T) {
		gatherer, err := gather(t, database.NewBundle(), nil, nil)
		testutil.NoError(t, err)
		families, err := gatherer.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 0)
	})

	t.Run("should fail on an invalid list of connections", func(t *testing.T) {
		_, err := gather(t, database.NewBundle(), nil, map[string]any{database.ConnectionsConfigPath: "main"})
		testutil.Error(t, flamError(t, err), database.ErrInvalidConnections,
			"unable to create metrics database collectors: invalid metrics database connections",
			map[string]any{database.ErrorFieldPath: database.ConnectionsConfigPath})
	})

	t.Run("should fail on listed connections without factory", func(t *testing.T) {
		_, err := gather(t, database.NewBundle(), nil, map[string]any{database.ConnectionsConfigPath: []string{"main"}})
		testutil.Error(t, flamError(t, err), database.ErrMissingFactory,
			"unable to create metrics database collectors: database connection factory not provided by the container",
			map[string]any{})
	})

	t.Run("should fail on a connection that cannot be taken", func(t *testing.T) {
		f := &dbFactory{available: []string{"main"}, err: errBoom}
		_, err := gather(t, database.NewBundle(), func() flam.IDatabaseConnectionFactory { return f }, nil)
		e := flamError(t, err)
		testutil.Equal(t, "boom", errors.Is(e, errBoom), true)
		testutil.Error(t, e, database.ErrConnection,
			"unable to create metrics database collectors: unable to get metrics database connection: boom",
			map[string]any{database.ErrorFieldID: "main"})
	})

	t.Run("should fail on a connection whose pool cannot be taken", func(t *testing.T) {
		f := &dbFactory{
			connections: map[string]flam.IDatabaseConnection{"main": &dbConnection{err: errBoom}},
			available:   []string{"main"},
		}
		_, err := gather(t, database.NewBundle(), func() flam.IDatabaseConnectionFactory { return f }, nil)
		e := flamError(t, err)
		testutil.Equal(t, "boom", errors.Is(e, errBoom), true)
		testutil.Error(t, e, database.ErrConnection,
			"unable to create metrics database collectors: unable to get metrics database connection: boom",
			map[string]any{database.ErrorFieldID: "main"})
	})
}

func Test_Redis(t *testing.T) {
	t.Run("should collect the statistics of every configured connection", func(t *testing.T) {
		f := &redisFactory{
			connections: map[string]flam.IRedisConnection{
				"cache": &redisConnection{stats: &goredis.PoolStats{
					Hits: 1, Misses: 2, Timeouts: 3, WaitCount: 4, Unusable: 5,
					WaitDurationNs: int64(1500 * time.Millisecond),
					TotalConns:     6, IdleConns: 7, StaleConns: 8, PendingRequests: 9,
				}},
				"empty": &redisConnection{},
			},
			available: []string{"cache", "empty"},
		}
		gatherer, err := gather(t, redis.NewBundle(), func() flam.IRedisConnectionFactory { return f }, nil)
		testutil.NoError(t, err)
		testutil.Equal(t, "got", f.got, []string{"cache", "empty"})
		testutil.NoError(t, promtestutil.GatherAndCompare(gatherer, strings.NewReader(`
# HELP go_redis_pool_connections The number of connections of the pool.
# TYPE go_redis_pool_connections gauge
go_redis_pool_connections{db_name="cache"} 6
# HELP go_redis_pool_hits_total The number of times a free connection was found in the pool.
# TYPE go_redis_pool_hits_total counter
go_redis_pool_hits_total{db_name="cache"} 1
# HELP go_redis_pool_idle_connections The number of idle connections of the pool.
# TYPE go_redis_pool_idle_connections gauge
go_redis_pool_idle_connections{db_name="cache"} 7
# HELP go_redis_pool_misses_total The number of times a free connection was not found in the pool.
# TYPE go_redis_pool_misses_total counter
go_redis_pool_misses_total{db_name="cache"} 2
# HELP go_redis_pool_pending_requests The number of requests waiting for a connection.
# TYPE go_redis_pool_pending_requests gauge
go_redis_pool_pending_requests{db_name="cache"} 9
# HELP go_redis_pool_stale_connections_total The number of stale connections removed from the pool.
# TYPE go_redis_pool_stale_connections_total counter
go_redis_pool_stale_connections_total{db_name="cache"} 8
# HELP go_redis_pool_timeouts_total The number of times waiting for a connection timed out.
# TYPE go_redis_pool_timeouts_total counter
go_redis_pool_timeouts_total{db_name="cache"} 3
# HELP go_redis_pool_unusable_total The number of times a connection was found unusable.
# TYPE go_redis_pool_unusable_total counter
go_redis_pool_unusable_total{db_name="cache"} 5
# HELP go_redis_pool_wait_duration_seconds_total The total time spent waiting for a connection, in seconds.
# TYPE go_redis_pool_wait_duration_seconds_total counter
go_redis_pool_wait_duration_seconds_total{db_name="cache"} 1.5
# HELP go_redis_pool_waits_total The number of times a connection was waited for.
# TYPE go_redis_pool_waits_total counter
go_redis_pool_waits_total{db_name="cache"} 4
`)))
	})

	t.Run("should collect the statistics of the listed connections only", func(t *testing.T) {
		f := &redisFactory{
			connections: map[string]flam.IRedisConnection{"b": &redisConnection{stats: &goredis.PoolStats{Hits: 2}}},
			available:   []string{"a", "b"},
		}
		gatherer, err := gather(t, redis.NewBundle(), func() flam.IRedisConnectionFactory { return f },
			map[string]any{redis.ConnectionsConfigPath: []string{"b"}})
		testutil.NoError(t, err)
		testutil.Equal(t, "got", f.got, []string{"b"})
		testutil.Equal(t, "hits", count(t, gatherer, redis.MetricHits), 1)
	})

	t.Run("should collect nothing without factory nor listed connections", func(t *testing.T) {
		gatherer, err := gather(t, redis.NewBundle(), nil, nil)
		testutil.NoError(t, err)
		families, err := gatherer.Gather()
		testutil.NoError(t, err)
		testutil.Equal(t, "families", len(families), 0)
	})

	t.Run("should fail on an invalid list of connections", func(t *testing.T) {
		_, err := gather(t, redis.NewBundle(), nil, map[string]any{redis.ConnectionsConfigPath: []any{1}})
		testutil.Error(t, flamError(t, err), redis.ErrInvalidConnections,
			"unable to create metrics redis collectors: invalid metrics redis connections",
			map[string]any{redis.ErrorFieldPath: redis.ConnectionsConfigPath})
	})

	t.Run("should fail on listed connections without factory", func(t *testing.T) {
		_, err := gather(t, redis.NewBundle(), nil, map[string]any{redis.ConnectionsConfigPath: []string{"cache"}})
		testutil.Error(t, flamError(t, err), redis.ErrMissingFactory,
			"unable to create metrics redis collectors: redis connection factory not provided by the container",
			map[string]any{})
	})

	t.Run("should fail on a connection that cannot be taken", func(t *testing.T) {
		f := &redisFactory{available: []string{"cache"}, err: errBoom}
		_, err := gather(t, redis.NewBundle(), func() flam.IRedisConnectionFactory { return f }, nil)
		e := flamError(t, err)
		testutil.Equal(t, "boom", errors.Is(e, errBoom), true)
		testutil.Error(t, e, redis.ErrConnection,
			"unable to create metrics redis collectors: unable to get metrics redis connection: boom",
			map[string]any{redis.ErrorFieldID: "cache"})
	})
}
