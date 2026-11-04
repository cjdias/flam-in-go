package redis

import (
	"context"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the connection factory only when provided.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Connections flam.IRedisConnectionFactory `optional:"true"`
}

// newCreator returns the [flam.IHealthCheckCreator] of the redis
// checks, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IHealthCheckCreator {
	return &creator{config: args.Config, connections: args.Connections}
}

// creator is the [flam.IHealthCheckCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config      flam.IConfig
	connections flam.IRedisConnectionFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the check of the configuration, failing with
// ErrInvalidField for an invalid field, ErrNoFactory without connection
// factory, and ErrUnknownConnection when the factory has no connection of
// the id.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IHealthCheck, error) {
	id := bag.String(factory.ConfigFieldId)

	kind, ok := health.ParseKind(settings.Field(c.config, bag, health.ConfigFieldKind, health.DefaultKindConfigPath, defaultKind))
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, health.ConfigFieldKind)
	}
	connection, ok := settings.Field(c.config, bag, ConfigFieldConnection, "", id).(string)
	switch {
	case !ok || connection == "":
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, ConfigFieldConnection)
	case c.connections == nil:
		return nil, newErr(ErrNoFactory, creatorCreateMessage, id, nil)
	case !c.connections.Has(connection):
		return nil, newErr(ErrUnknownConnection, creatorCreateMessage, id, nil).
			With(ErrorFieldConnection, connection)
	}

	return health.NewCheck(id, kind, func(ctx context.Context) error {
		return c.ping(ctx, id, connection)
	})
}

// Messages of the errors returned by the creator and the checks.
const (
	creatorCreateMessage = "unable to create redis health check"
	checkMessage         = "redis health check failed"
)

// ping sends a PING with the connection, failing with ErrConnection when
// the connection cannot be taken, and ErrPing when the server does not
// answer.
func (c *creator) ping(
	ctx context.Context,
	id string,
	connection string,
) error {
	conn, err := c.connections.Get(connection)
	if err != nil {
		return newErr(ErrConnection, checkMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	if err := conn.Ping(ctx).Err(); err != nil {
		return newErr(ErrPing, checkMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	return nil
}
