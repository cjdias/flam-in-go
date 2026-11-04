// Package redis provides, to the Prometheus registry of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics], the collectors of the
// statistics of the pools of the Redis connections of the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory] of the
// container, the go_redis_pool_* metrics, such as
// go_redis_pool_idle_connections, labelled by the id of their connection
// at [LabelConnection], as the go_sql_* metrics of the database
// connections are.
//
// The connections are those listed at [ConnectionsConfigPath], or else
// every connection configured in the factory, taken from it, and so
// opened, when the registry is created:
//
//	flam:
//	  metrics:
//	    collectors:
//	      redis:
//	        connections: [cache]
package redis
