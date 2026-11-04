// Package mini is the redis connection driver of an in-memory Redis
// server, [github.com/alicebob/miniredis/v2], for tests and local runs:
// each connection starts its own server, listening on the address of its
// address field, and closing the connection stops it, losing its data.
// Two connections, of different ids or created with Generate, never share
// data. [Server] returns the server of a connection.
//
// The connection reads the go-redis options of the fields shared by the
// drivers, documented in [github.com/cjdias/flam-in-go/pkg/flam/redis],
// and the server requires its username and password, when it has a
// password. miniredis implements most, not all, Redis commands: the
// others fail.
package mini
