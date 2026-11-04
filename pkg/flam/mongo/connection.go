package mongo

import (
	"context"

	gomongo "go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewConnection returns the [flam.IMongoConnection] of the id over the
// client, working by default in the database of the name, for the
// connection creators. Closing the connection disconnects the client. It
// fails with ErrNilClient when client is nil and ErrEmptyDatabase when
// database is empty.
func NewConnection(
	id string,
	client *gomongo.Client,
	database string,
) (flam.IMongoConnection, error) {
	if client == nil {
		return nil, newErrNew(ErrNilClient, id)
	}
	if database == "" {
		return nil, newErrNew(ErrEmptyDatabase, id)
	}
	return &connection{id: id, client: client, database: client.Database(database)}, nil
}

// connection is the [flam.IMongoConnection] returned by [NewConnection].
type connection struct {
	id       string
	client   *gomongo.Client
	database *gomongo.Database
}

func (c *connection) Id() string {
	return c.id
}

func (c *connection) Client() *gomongo.Client {
	return c.client
}

func (c *connection) Database() *gomongo.Database {
	return c.database
}

// Ping fails with the error of the client, wrapped with the ErrorFieldID
// context field.
func (c *connection) Ping(
	ctx context.Context,
) error {
	return c.wrap(connectionPingMessage, c.client.Ping(ctx, nil))
}

// Close disconnects the client, waiting for its connections in use to be
// returned to its pool. It fails with the error of the client, such as
// [gomongo.ErrClientDisconnected] once disconnected, wrapped with the
// ErrorFieldID context field.
func (c *connection) Close() error {
	return c.wrap(connectionCloseMessage, c.client.Disconnect(context.Background()))
}

// wrap returns the error of the client, wrapped with the message and the
// ErrorFieldID context field, or nil when there is none.
func (c *connection) wrap(
	message string,
	err error,
) error {
	if err == nil {
		return nil
	}
	return newErrConnection(message, c.id, err)
}

// Messages of the errors of the connections.
const (
	connectionNewMessage   = "unable to create mongo connection"
	connectionPingMessage  = "unable to ping mongo connection"
	connectionCloseMessage = "unable to close mongo connection"
)
