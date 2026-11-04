// Package redis is the locker driver of Redis: its lockers hold their
// locks in the Redis server of a connection of the
// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory], named
// by its id in their connection field, or its default, so that the
// lockers of the instances of an application sharing a server exclude
// each other.
//
// The lock of a key is the Redis key of the prefix of the locker followed
// by the key, set to the token of the acquisition with SET NX PX, its ttl
// being of a millisecond resolution. Its release and refresh run Lua
// scripts deleting or extending it with PEXPIRE when it still holds the
// token, so that a lock lost and acquired by another holder is never
// released or extended by this one. The acquisition also increments, in
// the same script, the fence of the key, held by the Redis key of the lock
// followed by [FenceSuffix], with no expiration, so that the fences keep
// increasing for as long as the server keeps its data.
//
// A locker reads the time of its waits and of its automatic refreshes, and
// measures the ttl of its locks, on the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, the
// server expiring the keys on its own clock. Closing a locker leaves its
// connection open, as the connection factory owns it.
//
// Redis replicates asynchronously: a lock acquired on a primary failing
// before its replicas got it may be acquired again on the promoted
// replica. Use the fences where that matters.
package redis
