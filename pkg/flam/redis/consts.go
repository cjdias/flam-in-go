package redis

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.redis"

// Configuration path and container group of the connections.
const (
	// ConnectionConfigPath is the configuration path holding one connection
	// configuration per connection id, as read by the connection factory.
	ConnectionConfigPath = "flam.redis.connections"

	// ConnectionCreatorGroup is the container group the connection driver
	// bundles provide their [flam.IRedisConnectionCreator] in, collected by
	// the connection factory.
	ConnectionCreatorGroup = "flam.redis.connections.creator"
)

// Fields of a connection configuration shared by the drivers of
// [github.com/cjdias/flam-in-go/pkg/flam/redis/connections], each setting
// the [github.com/redis/go-redis/v9.Options] field of the same name. A
// missing field leaves the go-redis default. A whole number is of any
// integer or float kind, and a duration is a [time.Duration] or a string
// such as "5s": a negative one, such as "-1s", disables what go-redis
// lets disable, such as the read timeout. The numbers, sizes and durations
// go-redis cannot take a negative value of are invalid when negative.
const (
	// ConfigFieldUsername is the field holding the username of the ACL
	// authentication, empty for the default user.
	ConfigFieldUsername = "username"

	// ConfigFieldPassword is the field holding the password, empty for
	// none.
	ConfigFieldPassword = "password" //nolint:gosec // a configuration field name, not a credential

	// ConfigFieldDatabase is the field holding the number of the database
	// to select, 0 or more.
	ConfigFieldDatabase = "database"

	// ConfigFieldProtocol is the field holding the RESP protocol version,
	// 2 or 3.
	ConfigFieldProtocol = "protocol"

	// ConfigFieldClientName is the field holding the name given to the
	// connections with CLIENT SETNAME.
	ConfigFieldClientName = "client_name"

	// ConfigFieldMaxRetries is the field holding the whole number of
	// retries of a failed command, -1 for none, or more.
	ConfigFieldMaxRetries = "max_retries"

	// ConfigFieldMinRetryBackoff is the field holding the minimum duration
	// between retries, negative for none.
	ConfigFieldMinRetryBackoff = "min_retry_backoff"

	// ConfigFieldMaxRetryBackoff is the field holding the maximum duration
	// between retries, negative for none.
	ConfigFieldMaxRetryBackoff = "max_retry_backoff"

	// ConfigFieldDialTimeout is the field holding the duration of the
	// timeout of a new connection, 0 or more.
	ConfigFieldDialTimeout = "dial_timeout"

	// ConfigFieldReadTimeout is the field holding the duration of the
	// timeout of a read, negative for none.
	ConfigFieldReadTimeout = "read_timeout"

	// ConfigFieldWriteTimeout is the field holding the duration of the
	// timeout of a write, negative for none.
	ConfigFieldWriteTimeout = "write_timeout"

	// ConfigFieldContextTimeoutEnabled is the field holding whether the
	// deadline of the context of a command applies to it, a bool.
	ConfigFieldContextTimeoutEnabled = "context_timeout_enabled"

	// ConfigFieldReadBufferSize is the field holding the whole number of
	// bytes of the read buffer of a connection.
	ConfigFieldReadBufferSize = "read_buffer_size"

	// ConfigFieldWriteBufferSize is the field holding the whole number of
	// bytes of the write buffer of a connection.
	ConfigFieldWriteBufferSize = "write_buffer_size"

	// ConfigFieldPoolFIFO is the field holding whether the pool hands out
	// its connections first in, first out, a bool.
	ConfigFieldPoolFIFO = "pool_fifo"

	// ConfigFieldPoolSize is the field holding the whole number of
	// connections of the pool, 0 or more.
	ConfigFieldPoolSize = "pool_size"

	// ConfigFieldPoolTimeout is the field holding the duration a command
	// waits for a connection of a busy pool, 0 or more.
	ConfigFieldPoolTimeout = "pool_timeout"

	// ConfigFieldMinIdleConns is the field holding the whole minimum
	// number of idle connections of the pool.
	ConfigFieldMinIdleConns = "min_idle_conns"

	// ConfigFieldMaxIdleConns is the field holding the whole maximum
	// number of idle connections of the pool.
	ConfigFieldMaxIdleConns = "max_idle_conns"

	// ConfigFieldMaxActiveConns is the field holding the whole maximum
	// number of connections of the pool, 0 for no limit.
	ConfigFieldMaxActiveConns = "max_active_conns"

	// ConfigFieldConnMaxIdleTime is the field holding the duration after
	// which an idle connection is closed, negative for never.
	ConfigFieldConnMaxIdleTime = "conn_max_idle_time"

	// ConfigFieldConnMaxLifetime is the field holding the duration after
	// which a connection is closed, 0 for never.
	ConfigFieldConnMaxLifetime = "conn_max_lifetime"

	// ConfigFieldDisableIdentity is the field holding whether the
	// connections skip CLIENT SETINFO, a bool.
	ConfigFieldDisableIdentity = "disable_identity"

	// ConfigFieldIdentitySuffix is the field holding the suffix of the
	// library name sent with CLIENT SETINFO.
	ConfigFieldIdentitySuffix = "identity_suffix"
)

// Context fields of the [*flam.Error] values returned by the redis
// package.
const (
	// ErrorFieldID holds the id of a connection.
	ErrorFieldID = "id"
)
