package broker

import (
	"context"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/health"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container, the broker factory only when provided.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Brokers flam.IBrokerFactory `optional:"true"`
}

// newCreator returns the [flam.IHealthCheckCreator] of the broker checks,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IHealthCheckCreator {
	return &creator{config: args.Config, brokers: args.Brokers}
}

// creator is the [flam.IHealthCheckCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config  flam.IConfig
	brokers flam.IBrokerFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the check of the configuration, failing with
// ErrInvalidField for an invalid field, ErrNoFactory without broker
// factory, and ErrUnknownBroker when the factory has no broker of the id.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IHealthCheck, error) {
	id := bag.String(factory.ConfigFieldId)

	kind, ok := health.ParseKind(settings.Field(c.config, bag, health.ConfigFieldKind, health.DefaultKindConfigPath, defaultKind))
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, health.ConfigFieldKind)
	}
	topic, ok := bag.Get(ConfigFieldTopic, defaultTopic).(string)
	if !ok {
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, ConfigFieldTopic)
	}
	brokerId, ok := settings.Field(c.config, bag, ConfigFieldBroker, "", id).(string)
	switch {
	case !ok || brokerId == "":
		return nil, newErr(ErrInvalidField, creatorCreateMessage, id, nil).
			With(ErrorFieldField, ConfigFieldBroker)
	case c.brokers == nil:
		return nil, newErr(ErrNoFactory, creatorCreateMessage, id, nil)
	case !c.brokers.Has(brokerId):
		return nil, newErr(ErrUnknownBroker, creatorCreateMessage, id, nil).
			With(ErrorFieldBroker, brokerId)
	}

	return health.NewCheck(id, kind, func(ctx context.Context) error {
		return c.check(ctx, id, brokerId, topic)
	})
}

// Messages of the errors returned by the creator and the checks.
const (
	creatorCreateMessage = "unable to create broker health check"
	checkMessage         = "broker health check failed"
)

// check takes the broker from the factory, failing with ErrBroker when it
// cannot, then pings it when it implements flam.IHealthPinger, failing
// with ErrPing when its backend does not answer, or else publishes an
// empty message on the topic, unless empty, failing with ErrPublish when
// it cannot.
func (c *creator) check(
	ctx context.Context,
	id string,
	brokerId string,
	topic string,
) error {
	b, err := c.brokers.Get(brokerId)
	if err != nil {
		return newErr(ErrBroker, checkMessage, id, err).
			With(ErrorFieldBroker, brokerId)
	}
	if pinger, ok := b.(flam.IHealthPinger); ok {
		if err := pinger.Ping(ctx); err != nil {
			return newErr(ErrPing, checkMessage, id, err).
				With(ErrorFieldBroker, brokerId)
		}
		return nil
	}
	if topic == "" {
		return nil
	}
	if err := b.Publish(ctx, topic, []byte{}, nil); err != nil {
		return newErr(ErrPublish, checkMessage, id, err).
			With(ErrorFieldBroker, brokerId).
			With(ErrorFieldTopic, topic)
	}
	return nil
}
