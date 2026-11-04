package mini

import (
	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/redis"
	"github.com/cjdias/flam-in-go/pkg/flam/redis/connections/internal/base"
)

// newCreator returns the [flam.IRedisConnectionCreator] of the mini
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

// Create starts a server at the address of the fields, failing at the
// first invalid one, and returns the connection to it. The server
// requires the username and password of the connection, when it has a
// password.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRedisConnection, error) {
	id := bag.String(factory.ConfigFieldId)
	r := base.NewReader(c.config, bag)

	address := r.NonEmpty(ConfigFieldAddress, DefaultAddressConfigPath, defaultAddress)
	options := r.Options("", "", "")
	if field := r.Invalid(); field != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, field)
	}

	server := miniredis.NewMiniRedis()
	switch {
	case options.Password == "":
	case options.Username == "":
		server.RequireAuth(options.Password)
	default:
		server.RequireUserAuth(options.Username, options.Password)
	}
	if err := server.StartAddr(address); err != nil {
		return nil, newErrStart(creatorCreateMessage, id, address, err)
	}

	options.Addr = server.Addr()
	conn, _ := redis.NewConnection(id, goredis.NewClient(options)) // a client is never nil
	return &connection{IRedisConnection: conn, server: server}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create mini redis connection"
