package redis

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.metrics.collectors.redis"

// ConnectionsConfigPath is the configuration path of the list of the ids
// of the Redis connections whose statistics are collected, a []string or
// a []any of strings, every connection configured in the
// [flam.IRedisConnectionFactory] unless set.
const ConnectionsConfigPath = "flam.metrics.collectors.redis.connections"

// LabelConnection is the label of the metrics holding the id of their
// connection, named as the label of the go_sql_* metrics.
const LabelConnection = "db_name"

// Names of the metrics, each of the statistic of the
// [github.com/redis/go-redis/v9.PoolStats] field of the same name.
const (
	// MetricHits is the counter of the times a free connection was found
	// in the pool.
	MetricHits = "go_redis_pool_hits_total"

	// MetricMisses is the counter of the times no free connection was
	// found in the pool.
	MetricMisses = "go_redis_pool_misses_total"

	// MetricTimeouts is the counter of the times waiting for a connection
	// timed out.
	MetricTimeouts = "go_redis_pool_timeouts_total"

	// MetricWaits is the counter of the times a connection was waited
	// for, of the WaitCount field.
	MetricWaits = "go_redis_pool_waits_total"

	// MetricWaitDuration is the counter of the time spent waiting for a
	// connection, in seconds, of the WaitDurationNs field.
	MetricWaitDuration = "go_redis_pool_wait_duration_seconds_total"

	// MetricUnusable is the counter of the times a connection was found
	// unusable.
	MetricUnusable = "go_redis_pool_unusable_total"

	// MetricStaleConnections is the counter of the stale connections
	// removed from the pool, of the StaleConns field.
	MetricStaleConnections = "go_redis_pool_stale_connections_total"

	// MetricConnections is the gauge of the connections of the pool, of
	// the TotalConns field.
	MetricConnections = "go_redis_pool_connections"

	// MetricIdleConnections is the gauge of the idle connections of the
	// pool, of the IdleConns field.
	MetricIdleConnections = "go_redis_pool_idle_connections"

	// MetricPendingRequests is the gauge of the requests waiting for a
	// connection.
	MetricPendingRequests = "go_redis_pool_pending_requests"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of a Redis connection.
	ErrorFieldID = "id"

	// ErrorFieldPath holds the configuration path of an invalid value.
	ErrorFieldPath = "path"
)
