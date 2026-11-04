package redis

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/cache"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The connection factory is optional, so that the adaptor
// factory can be created without the redis bundle, failing to create the
// redis adaptors only.
type creatorArgs struct {
	dig.In

	Config        flam.IConfig
	Serializers   flam.ICacheSerializerFactory
	KeyGenerators flam.ICacheKeyGeneratorFactory
	Connections   flam.IRedisConnectionFactory `optional:"true"`
}

// newCreator returns the [flam.ICacheAdaptorCreator] of the redis
// adaptors, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ICacheAdaptorCreator {
	return &creator{
		config:        args.Config,
		serializers:   args.Serializers,
		keyGenerators: args.KeyGenerators,
		connections:   args.Connections,
	}
}

// creator is the [flam.ICacheAdaptorCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config        flam.IConfig
	serializers   flam.ICacheSerializerFactory
	keyGenerators flam.ICacheKeyGeneratorFactory
	connections   flam.IRedisConnectionFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the adaptor with the serializer, key generator and
// connection of the ids of the configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ICacheAdaptor, error) {
	id := bag.String(factory.ConfigFieldId)

	serializerID, ok := c.field(bag, cache.ConfigFieldSerializer, cache.DefaultSerializerConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, cache.ConfigFieldSerializer)
	}
	keyGeneratorID, ok := c.field(bag, cache.ConfigFieldKeyGenerator, cache.DefaultKeyGeneratorConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, cache.ConfigFieldKeyGenerator)
	}
	connectionID, ok := c.field(bag, ConfigFieldConnection, DefaultConnectionConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldConnection)
	}

	serializer, err := c.serializers.Get(serializerID)
	if err != nil {
		return nil, newErrJoined(ErrSerializer, creatorCreateMessage, id, ErrorFieldSerializer, serializerID, err)
	}
	keyGenerator, err := c.keyGenerators.Get(keyGeneratorID)
	if err != nil {
		return nil, newErrJoined(ErrKeyGenerator, creatorCreateMessage, id, ErrorFieldKeyGenerator, keyGeneratorID, err)
	}
	if c.connections == nil {
		return nil, newErrJoined(ErrConnection, creatorCreateMessage, id, ErrorFieldConnection, connectionID, errNoConnections)
	}
	connection, err := c.connections.Get(connectionID)
	if err != nil {
		return nil, newErrJoined(ErrConnection, creatorCreateMessage, id, ErrorFieldConnection, connectionID, err)
	}
	return NewAdaptor(id, keyGenerator, serializer, connection)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create redis cache adaptor"

// field returns the id held by the field of the adaptor configuration, or
// else by the configuration at the default path, and false when it is not
// a non-empty string.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
) (string, bool) {
	val := bag.Get(field)
	if val == nil {
		val = c.config.Get(defaultPath)
	}
	s, ok := val.(string)
	return s, ok && s != ""
}
