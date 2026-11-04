package redis

import (
	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewConnection returns the [flam.IRedisConnection] of the id over the
// client, for the connection creators. Closing the connection closes the
// client. It fails with ErrNilClient when client is nil.
func NewConnection(
	id string,
	client *goredis.Client,
) (flam.IRedisConnection, error) {
	if client == nil {
		return nil, newErrNil(ErrNilClient, connectionNewMessage, id)
	}
	return &connection{Client: client, id: id}, nil
}

// connection is the [flam.IRedisConnection] returned by [NewConnection].
type connection struct {
	*goredis.Client

	id string
}

func (c *connection) Id() string {
	return c.id
}

// connectionNewMessage is the message of the errors returned by
// NewConnection.
const connectionNewMessage = "unable to create redis connection"
