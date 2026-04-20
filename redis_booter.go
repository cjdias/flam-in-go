package flam

import (
	"sync"

	"github.com/alicebob/miniredis/v2"
)

type redisBooter struct {
	mu     sync.Mutex
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
	booter.mu.Lock()
	defer booter.mu.Unlock()

	if booter.mini != nil {
		booter.mini.Close()
		booter.mini = nil
	}

	return nil
}

func (booter *redisBooter) Boot() error {
	booter.mu.Lock()
	defer booter.mu.Unlock()

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
			// Error ignored - config update is best effort, shouldn't block boot
			_ = booter.config.Set(PathRedisConnections+"."+id+".host", addr)
		}
	}

	return nil
}
