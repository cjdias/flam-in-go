package flam

import "github.com/alicebob/miniredis/v2"

type redisBooter struct {
	config Config
	mini   *miniredis.Miniredis
}

func newRedisBooter(
	config Config,
) *redisBooter {
	return &redisBooter{
		config: config}
}

func (booter *redisBooter) Close() error {
	if booter.mini != nil {
		booter.mini.Close()
		booter.mini = nil
	}

	return nil
}

func (booter *redisBooter) Boot() error {
	if !booter.config.Bool(PathRedisMiniBoot) {
		return nil
	}

	booter.mini = miniredis.NewMiniRedis()
	if e := booter.mini.Start(); e != nil {
		return e
	}

	addr := booter.mini.Addr()

	for id := range booter.config.Bag(PathRedisConnections) {
		if booter.config.String(PathRedisConnections+"."+id+".driver") == RedisConnectionDriverMini {
			_ = booter.config.Set(PathRedisConnections+"."+id+".host", addr)
		}
	}

	return nil
}
