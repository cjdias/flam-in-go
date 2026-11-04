// Package lockers provides the bundle registering every locker driver
// bundle, to be registered along with the
// [github.com/cjdias/flam-in-go/pkg/flam/lock] bundle, while its sub
// packages hold the locker drivers: redis, locking in a Redis server
// shared by the instances of an application, and memory, locking in the
// process.
package lockers
