package mongo

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.mongo"

// Configuration path and container group of the connections.
const (
	// ConnectionConfigPath is the configuration path holding one connection
	// configuration per connection id, as read by the connection factory.
	ConnectionConfigPath = "flam.mongo.connections"

	// ConnectionCreatorGroup is the container group the connection driver
	// bundles provide their [flam.IMongoConnectionCreator] in, collected by
	// the connection factory.
	ConnectionCreatorGroup = "flam.mongo.connections.creator"
)

// Context fields of the [*flam.Error] values returned by the mongo
// package.
const (
	// ErrorFieldID holds the id of a connection.
	ErrorFieldID = "id"
)
