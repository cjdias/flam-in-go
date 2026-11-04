// Package redis is the redis feature: the bundle returned from [NewBundle]
// provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory] of an
// application, which creates the Redis connections described by the
// configuration at [ConnectionConfigPath] with the creators provided in
// the [ConnectionCreatorGroup] container group, such as:
//
//	flam:
//	  redis:
//	    connections:
//	      main: {driver: flam.redis.connections.driver.standard, host: cache, database: 1}
//	      test: {driver: flam.redis.connections.driver.mini}
//
// A connection is a [github.com/redis/go-redis/v9.Client]: it connects
// lazily, on its first command, so that creating it does not reach the
// server. [NewConnection] wraps a client created elsewhere.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/redis/connections], reading the
// fields shared by the drivers, such as [ConfigFieldDatabase] and
// [ConfigFieldPoolSize], along with their own. They import this package for
// its paths, groups and fields, so this package never imports them.
package redis
