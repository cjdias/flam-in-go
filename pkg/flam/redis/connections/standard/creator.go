package standard

import (
	"net"
	"strconv"

	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/internal/base"
)

// newCreator returns the [flam.IRedisConnectionCreator] of the standard
// connections, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IRedisConnectionCreator {
	return &creator{config: config}
}

// creator is the [flam.IRedisConnectionCreator] of the configurations
// with the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the connection to the server of the fields, failing at
// the first invalid one. It does not reach the server.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRedisConnection, error) {
	id := bag.String(factory.ConfigFieldId)
	r := base.NewReader(c.config, bag)

	network := r.NonEmpty(ConfigFieldNetwork, DefaultNetworkConfigPath, defaultNetwork)
	host := r.NonEmpty(ConfigFieldHost, DefaultHostConfigPath, defaultHost)
	port := r.Port(ConfigFieldPort, DefaultPortConfigPath, defaultPort)
	options := r.Options(DefaultUsernameConfigPath, DefaultPasswordConfigPath, DefaultDatabaseConfigPath)
	if field := r.Invalid(); field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}

	options.Network = network
	options.Addr = host
	if network != "unix" {
		options.Addr = net.JoinHostPort(host, strconv.Itoa(port))
	}
	return redis.NewConnection(id, goredis.NewClient(options))
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard redis connection"
