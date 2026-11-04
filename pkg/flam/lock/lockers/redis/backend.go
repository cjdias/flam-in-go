package redis

import (
	"context"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// backend holds the locks of a redis locker in the server of its
// connection, under the keys of its prefix.
type backend struct {
	conn   flam.IRedisConnection
	prefix string
}

// Acquire sets the key of the lock to the token with SET NX PX, and, when
// set, increments the key of its fence, in one script.
func (b *backend) Acquire(
	ctx context.Context,
	key string,
	token string,
	ttl time.Duration,
) (int64, bool, error) {
	k := b.prefix + key
	fence, err := acquireScript.Run(ctx, b.conn, []string{k, k + FenceSuffix}, token, ttl.Milliseconds()).Int64()
	if err != nil {
		return 0, false, err
	}
	return fence, fence > 0, nil
}

// Refresh extends the key of the lock with PEXPIRE when it holds the
// token, in one script.
func (b *backend) Refresh(
	ctx context.Context,
	key string,
	token string,
	ttl time.Duration,
) (bool, error) {
	n, err := refreshScript.Run(ctx, b.conn, []string{b.prefix + key}, token, ttl.Milliseconds()).Int64()
	return n == 1, err
}

// Release deletes the key of the lock when it holds the token, in one
// script.
func (b *backend) Release(
	ctx context.Context,
	key string,
	token string,
) (bool, error) {
	n, err := releaseScript.Run(ctx, b.conn, []string{b.prefix + key}, token).Int64()
	return n == 1, err
}

// The scripts of the backend, run with EVALSHA, loaded by EVAL when the
// server lacks them.
var (
	// acquireScript sets KEYS[1] to the token ARGV[1] for ARGV[2]
	// milliseconds unless set, returning then the increment of the fence
	// KEYS[2], or else 0.
	acquireScript = goredis.NewScript(`
if redis.call('SET', KEYS[1], ARGV[1], 'NX', 'PX', ARGV[2]) then
	return redis.call('INCR', KEYS[2])
end
return 0`)

	// refreshScript extends KEYS[1] to expire in ARGV[2] milliseconds
	// when it holds the token ARGV[1], returning 1, or else 0.
	refreshScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0`)

	// releaseScript deletes KEYS[1] when it holds the token ARGV[1],
	// returning 1, or else 0.
	releaseScript = goredis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
	return redis.call('DEL', KEYS[1])
end
return 0`)
)
