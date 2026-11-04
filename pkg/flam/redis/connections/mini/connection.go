package mini

import (
	"github.com/alicebob/miniredis/v2"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Server returns the in-memory server of a connection created by the mini
// driver, to inspect or change its data or fast forward its time in tests,
// and false for any other connection.
func Server(
	conn flam.IRedisConnection,
) (*miniredis.Miniredis, bool) {
	c, ok := conn.(*connection)
	if !ok {
		return nil, false
	}
	return c.server, true
}

// connection is the [flam.IRedisConnection] of the mini driver, owning
// its server.
type connection struct {
	flam.IRedisConnection

	server *miniredis.Miniredis
}

// Close closes the client, then the server, returning the error of the
// client.
func (c *connection) Close() error {
	err := c.IRedisConnection.Close()
	c.server.Close()
	return err
}
