// Package adaptors provides the bundle registering every cache adaptor
// driver bundle, to be registered along with the
// [github.com/cjdias/flam-in-go/pkg/flam/cache] bundle, while its sub
// packages hold the adaptor drivers: redis, caching in a Redis server,
// memory, caching in the process, and invalidating, wrapping another
// adaptor to remove the values it holds in the other processes through a
// broker.
package adaptors
