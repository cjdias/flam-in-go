package client

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.client"

// Configuration paths and container groups of the clients and the circuit
// breakers.
const (
	// ClientConfigPath is the configuration path holding one client
	// configuration per client id, as read by the client factory.
	ClientConfigPath = "flam.client.clients"

	// ClientCreatorGroup is the container group the client driver bundles
	// provide their [flam.IClientCreator] in, collected by the client
	// factory.
	ClientCreatorGroup = "flam.client.clients.creator"

	// BreakerConfigPath is the configuration path holding one circuit
	// breaker configuration per breaker id, as read by the circuit breaker
	// factory.
	BreakerConfigPath = "flam.client.breakers"

	// BreakerCreatorGroup is the container group the circuit breaker
	// driver bundles provide their [flam.IClientCircuitBreakerCreator] in,
	// collected by the circuit breaker factory.
	BreakerCreatorGroup = "flam.client.breakers.creator"
)

// Configuration paths of the values taken by the fields missing from a
// client configuration, whatever its driver.
const (
	// DefaultBreakerConfigPath is the path of the default id of the
	// circuit breaker of the clients, unset unless set by the application:
	// the clients then have no circuit breaker.
	DefaultBreakerConfigPath = "flam.client.defaults.clients.breaker"
)
