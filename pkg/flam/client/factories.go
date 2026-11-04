package client

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// clientFactoryArgs are the dependencies of the client factory, taken
// from the container. The group tag repeats ClientCreatorGroup, as a
// struct tag cannot use a constant.
type clientFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IClientCreator `group:"flam.client.clients.creator"`
}

// newClientFactory returns the client factory creating the clients
// configured at ClientConfigPath with the creators of the
// ClientCreatorGroup group.
func newClientFactory(
	args clientFactoryArgs,
) (flam.IClientFactory, error) {
	return factory.New(args.Config.Bag(ClientConfigPath), args.Creators...)
}

// breakerFactoryArgs are the dependencies of the circuit breaker factory,
// taken from the container. The group tag repeats BreakerCreatorGroup.
type breakerFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.IClientCircuitBreakerCreator `group:"flam.client.breakers.creator"`
}

// newBreakerFactory returns the circuit breaker factory creating the
// breakers configured at BreakerConfigPath with the creators of the
// BreakerCreatorGroup group.
func newBreakerFactory(
	args breakerFactoryArgs,
) (flam.IClientCircuitBreakerFactory, error) {
	return factory.New(args.Config.Bag(BreakerConfigPath), args.Creators...)
}
