package invalidating

import (
	"sync"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container. The broker factory is optional, so that the adaptor factory
// can be created without the broker bundle, failing to create the
// invalidating adaptors only.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Brokers flam.IBrokerFactory `optional:"true"`
}

// newCreator returns the [flam.ICacheAdaptorWrapperCreator] of the
// invalidating adaptors, reading the defaults of the missing fields from
// the configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ICacheAdaptorCreator {
	return &creator{
		config:  args.Config,
		brokers: args.Brokers,
	}
}

// creator is the [flam.ICacheAdaptorWrapperCreator] of the configurations
// with the [Driver] driver, taking the wrapped adaptors from the factory
// handed to it.
type creator struct {
	config  flam.IConfig
	brokers flam.IBrokerFactory

	mu       sync.Mutex // guards the field below
	adaptors flam.ICacheAdaptorFactory
}

// compile time check that *creator implements ICacheAdaptorWrapperCreator
var _ flam.ICacheAdaptorWrapperCreator = (*creator)(nil)

func (c *creator) UseFactory(
	adaptors flam.ICacheAdaptorFactory,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adaptors = adaptors
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the adaptor wrapping the adaptor of the id of the
// configuration, or of its default, with the broker, topic and publish
// set of the configuration, or of their defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ICacheAdaptor, error) {
	id := bag.String(factory.ConfigFieldId)

	adaptorID, ok := c.id(bag, ConfigFieldAdaptor, DefaultAdaptorConfigPath)
	if !ok || adaptorID == id {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldAdaptor)
	}
	brokerID, ok := c.id(bag, ConfigFieldBroker, DefaultBrokerConfigPath)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldBroker)
	}
	topic, ok := settings.Field(c.config, bag, ConfigFieldTopic, DefaultTopicConfigPath, TopicPrefix+id).(string)
	if !ok || topic == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTopic)
	}
	publishSet, ok := settings.Field(c.config, bag, ConfigFieldPublishSet, DefaultPublishSetConfigPath, DefaultPublishSet).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldPublishSet)
	}

	c.mu.Lock()
	adaptors := c.adaptors
	c.mu.Unlock()
	if adaptors == nil {
		return nil, newErrJoined(ErrAdaptor, creatorCreateMessage, id, ErrorFieldAdaptor, adaptorID, errNoFactory)
	}
	wrapped, err := adaptors.Get(adaptorID)
	if err != nil {
		return nil, newErrJoined(ErrAdaptor, creatorCreateMessage, id, ErrorFieldAdaptor, adaptorID, err)
	}
	if c.brokers == nil {
		return nil, newErrJoined(ErrBroker, creatorCreateMessage, id, ErrorFieldBroker, brokerID, errNoBrokers)
	}
	broker, err := c.brokers.Get(brokerID)
	if err != nil {
		return nil, newErrJoined(ErrBroker, creatorCreateMessage, id, ErrorFieldBroker, brokerID, err)
	}
	return NewAdaptor(id, wrapped, broker, topic, publishSet)
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create invalidating cache adaptor"

// id returns the id held by the field of the adaptor configuration, or
// else by the configuration at the default path, and false when it is not
// a non-empty string.
func (c *creator) id(
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
