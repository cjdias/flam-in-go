package redis

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/lock/lockers/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the connection factory and the clock being optional so that
// the locker factory can be created without the redis and clock bundles,
// failing to create the redis lockers only.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Connections flam.IRedisConnectionFactory `optional:"true"`
	Clock       flam.IClock                  `optional:"true"`
}

// newCreator returns the [flam.ILockerCreator] of the redis lockers,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILockerCreator {
	return &creator{config: args.Config, connections: args.Connections, clock: args.Clock}
}

// creator is the [flam.ILockerCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	connections flam.IRedisConnectionFactory
	clock       flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the locker of the fields, failing at the first invalid
// one, over the redis connection of its connection field, taken from the
// connection factory, which keeps owning it.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ILocker, error) {
	id := bag.String(factory.ConfigFieldId)

	connection, ok := settings.Field(c.config, bag, ConfigFieldConnection, DefaultConnectionConfigPath, defaultConnection).(string)
	if !ok || connection == "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, ConfigFieldConnection)
	}
	prefix, ok := settings.Field(c.config, bag, ConfigFieldPrefix, DefaultPrefixConfigPath, defaultPrefix).(string)
	if !ok {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, ConfigFieldPrefix)
	}
	options, field := base.ReadOptions(c.config, bag)
	if field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}
	if c.clock == nil {
		return nil, base.NewErr(ErrClock, creatorCreateMessage, id, nil)
	}

	var conn flam.IRedisConnection
	var err error
	if c.connections == nil {
		err = errNoFactory
	} else {
		conn, err = c.connections.Get(connection)
	}
	if err != nil {
		return nil, base.NewErr(ErrConnection, creatorCreateMessage, id, err).
			With(ErrorFieldConnection, connection)
	}
	return base.NewLocker(id, &backend{conn: conn, prefix: prefix}, c.clock, options), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create redis locker"
