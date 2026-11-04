package flam

import (
	"context"
	"io"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// IMongoConnection is a connection to a MongoDB deployment, a
// [mongo.Client] of [go.mongodb.org/mongo-driver/v2] along with the
// database its consumers work in by default. It is held by an
// [IMongoConnectionFactory], which closes it, disconnecting its client,
// when removing it.
type IMongoConnection interface {
	IResource
	io.Closer

	// Client returns the client of the connection, shared by every
	// consumer of the connection.
	Client() *mongo.Client

	// Database returns the default database of the connection, the one
	// its configuration names.
	Database() *mongo.Database

	// Ping checks that the deployment can be reached, with the read
	// preference of the client, until the server selection timeout of the
	// client or the end of ctx.
	Ping(ctx context.Context) error
}

// IMongoConnectionCreator creates the MongoDB connections of a driver, for
// the [IMongoConnectionFactory].
type IMongoConnectionCreator = IResourceCreator[IMongoConnection]

// IMongoConnectionFactory creates and holds the MongoDB connections, one
// per id.
type IMongoConnectionFactory = IFactory[IMongoConnection]
