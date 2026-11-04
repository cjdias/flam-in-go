// Package standard is the redis connection driver of a Redis server
// reached over TCP or a unix socket, at the address of its network, host
// and port fields, with the go-redis options of the fields shared by the
// drivers, documented in [github.com/cjdias/flam-in-go/pkg/flam/redis].
// Creating a connection does not reach the server: an unreachable one
// fails the commands instead.
package standard
