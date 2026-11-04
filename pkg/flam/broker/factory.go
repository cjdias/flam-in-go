package broker

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// brokerFactoryArgs are the dependencies of the broker factory, taken
// from the container. The group tag repeats BrokerCreatorGroup, as a
// struct tag cannot use a constant.
type brokerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IBrokerCreator `group:"flam.broker.brokers.creator"`
}

// newBrokerFactory returns the broker factory creating the brokers
// configured at BrokerConfigPath with the creators of the
// BrokerCreatorGroup group.
func newBrokerFactory(
	args brokerFactoryArgs,
) (flam.IBrokerFactory, error) {
	return factory.New(args.Config.Bag(BrokerConfigPath), args.Creators...)
}
