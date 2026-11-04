// Package mongo is the mongo feature: the bundle returned from [NewBundle]
// provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IMongoConnectionFactory] of an
// application, which creates the MongoDB connections described by the
// configuration at [ConnectionConfigPath] with the creators provided in
// the [ConnectionCreatorGroup] container group, such as:
//
//	flam:
//	  mongo:
//	    connections:
//	      main:
//	        driver: flam.mongo.connections.driver.standard
//	        uri: mongodb://db-1,db-2/?replicaSet=rs0
//	        database: app
//
// A connection is a [go.mongodb.org/mongo-driver/v2/mongo.Client] along
// with its default database: creating it does not wait for the deployment,
// which the client monitors in the background, so that an unreachable one
// fails the operations, or Ping, instead. [NewConnection] wraps a client
// created elsewhere.
//
// The package has no health check nor metrics of its own. A health check
// is a check calling the connection Ping, with the deadline of the check,
// registered with the health feature; the metrics of the commands are
// those of a [go.mongodb.org/mongo-driver/v2/event.CommandMonitor] and of
// the pool those of a
// [go.mongodb.org/mongo-driver/v2/event.PoolMonitor], set on the options
// of the client by a driver, or by a client created elsewhere and given to
// NewConnection.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/mongo/connections]. They import
// this package for its paths and groups, so this package never imports
// them.
package mongo
