package redis

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.lock.lockers.redis"

// Driver is the driver name of the redis lockers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.lock.lockers.driver.redis"

// Fields of a redis locker configuration, along with the fields shared by
// the locker drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/lock].
const (
	// ConfigFieldConnection is the field holding the id of the redis
	// connection of the locker, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory].
	ConfigFieldConnection = "connection"

	// ConfigFieldPrefix is the field holding the prefix of the Redis keys
	// of the locks, which may be empty.
	ConfigFieldPrefix = "prefix"
)

// Configuration paths of the values taken by the fields missing from a
// locker configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], "default" unless set.
	DefaultConnectionConfigPath = "flam.lock.defaults.lockers.redis.connection"

	// DefaultPrefixConfigPath is the path of the default of
	// [ConfigFieldPrefix], "lock:" unless set.
	DefaultPrefixConfigPath = "flam.lock.defaults.lockers.redis.prefix"
)

// FenceSuffix is the suffix of the Redis key holding the fence of a lock,
// after the key of the lock.
const FenceSuffix = ":fence"

// Context fields of the [*flam.Error] values returned by the creator,
// along with those of [github.com/cjdias/flam-in-go/pkg/flam/lock].
const (
	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the redis connection.
	ErrorFieldConnection = "connection"
)

// Defaults of the fields of a locker configuration.
const (
	defaultConnection = "default"
	defaultPrefix     = "lock:"
)
