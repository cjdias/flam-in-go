package flam

import (
	"fmt"

	"github.com/redis/go-redis/v9"
)

type defaultRedisConnectionCreator struct{}

func newDefaultRedisConnectionCreator() RedisConnectionCreator {
	return &defaultRedisConnectionCreator{}
}

func (defaultRedisConnectionCreator) Accept(
	config Bag,
) bool {
	return config.String("driver") == RedisConnectionDriverDefault
}

func (defaultRedisConnectionCreator) Create(
	config Bag,
) (RedisConnection, error) {
	host := config.String("host", DefaultRedisHost)
	port := config.Int("port", DefaultRedisPort)

	switch {
	case host == "":
		return nil, newErrInvalidResourceConfig("defaultRedisConnection", "host", config)
	case port == 0:
		return nil, newErrInvalidResourceConfig("defaultRedisConnection", "port", config)
	}

	return redis.NewClient(&redis.Options{
		Network:               config.String("network"),
		Addr:                  fmt.Sprintf("%s:%d", host, port),
		ClientName:            config.String("client_name"),
		Protocol:              config.Int("protocol"),
		Username:              config.String("username"),
		Password:              config.String("password", DefaultRedisPassword),
		DB:                    config.Int("db", DefaultRedisDatabase),
		MaxRetries:            config.Int("max_retries"),
		MinRetryBackoff:       config.Duration("min_retry_backoff"),
		MaxRetryBackoff:       config.Duration("max_retry_backoff"),
		DialTimeout:           config.Duration("dial_timeout"),
		ReadTimeout:           config.Duration("read_timeout"),
		WriteTimeout:          config.Duration("write_timeout"),
		ContextTimeoutEnabled: config.Bool("context_timeout_enabled"),
		ReadBufferSize:        config.Int("read_buffer_size"),
		WriteBufferSize:       config.Int("write_buffer_size"),
		PoolFIFO:              config.Bool("pool_fifo"),
		PoolSize:              config.Int("pool_size"),
		PoolTimeout:           config.Duration("pool_timeout"),
		MinIdleConns:          config.Int("min_idle_conns"),
		MaxIdleConns:          config.Int("max_idle_conns"),
		MaxActiveConns:        config.Int("max_active_conns"),
		ConnMaxIdleTime:       config.Duration("conn_max_idle_time"),
		ConnMaxLifetime:       config.Duration("conn_max_lifetime"),
		DisableIdentity:       config.Bool("disable_identity"),
		IdentitySuffix:        config.String("identity_suffix"),
		UnstableResp3:         config.Bool("unstable_resp3"),
	}), nil
}
