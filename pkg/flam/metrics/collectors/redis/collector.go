package redis

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// newCollector returns the collector of the statistics of the pool of the
// connection, labelled by the id.
func newCollector(
	id string,
	connection flam.IRedisConnection,
) prometheus.Collector {
	labels := prometheus.Labels{LabelConnection: id}
	stats := make([]collectorStat, len(collectorStats))
	for i, stat := range collectorStats {
		stats[i] = collectorStat{
			desc:      prometheus.NewDesc(stat.name, stat.help, nil, labels),
			valueType: stat.valueType,
			value:     stat.value,
		}
	}
	return &collector{connection: connection, stats: stats}
}

// collector is the [prometheus.Collector] of the statistics of the pool
// of a connection.
type collector struct {
	connection flam.IRedisConnection
	stats      []collectorStat
}

// compile time check that *collector implements Collector
var _ prometheus.Collector = (*collector)(nil)

// Describe sends the descriptions of the metrics.
func (c *collector) Describe(
	ch chan<- *prometheus.Desc,
) {
	for _, stat := range c.stats {
		ch <- stat.desc
	}
}

// Collect sends the metrics of the statistics of the pool, none when the
// connection reports none.
func (c *collector) Collect(
	ch chan<- prometheus.Metric,
) {
	stats := c.connection.PoolStats()
	if stats == nil {
		return
	}
	for _, stat := range c.stats {
		ch <- prometheus.MustNewConstMetric(stat.desc, stat.valueType, stat.value(stats))
	}
}

// collectorStat is a metric of the statistics of a pool: its description,
// its type, and the reading of its value from the statistics.
type collectorStat struct {
	desc      *prometheus.Desc
	valueType prometheus.ValueType
	value     func(*goredis.PoolStats) float64
}

// collectorStats are the metrics of the statistics of a pool, described
// by their names and their help texts.
var collectorStats = []struct {
	name      string
	help      string
	valueType prometheus.ValueType
	value     func(*goredis.PoolStats) float64
}{
	{MetricHits, "The number of times a free connection was found in the pool.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.Hits) }},
	{MetricMisses, "The number of times a free connection was not found in the pool.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.Misses) }},
	{MetricTimeouts, "The number of times waiting for a connection timed out.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.Timeouts) }},
	{MetricWaits, "The number of times a connection was waited for.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.WaitCount) }},
	{MetricWaitDuration, "The total time spent waiting for a connection, in seconds.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return time.Duration(s.WaitDurationNs).Seconds() }},
	{MetricUnusable, "The number of times a connection was found unusable.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.Unusable) }},
	{MetricStaleConnections, "The number of stale connections removed from the pool.", prometheus.CounterValue,
		func(s *goredis.PoolStats) float64 { return float64(s.StaleConns) }},
	{MetricConnections, "The number of connections of the pool.", prometheus.GaugeValue,
		func(s *goredis.PoolStats) float64 { return float64(s.TotalConns) }},
	{MetricIdleConnections, "The number of idle connections of the pool.", prometheus.GaugeValue,
		func(s *goredis.PoolStats) float64 { return float64(s.IdleConns) }},
	{MetricPendingRequests, "The number of requests waiting for a connection.", prometheus.GaugeValue,
		func(s *goredis.PoolStats) float64 { return float64(s.PendingRequests) }},
}
