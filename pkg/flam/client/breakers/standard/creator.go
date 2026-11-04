package standard

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.IClientCircuitBreakerCreator] of the
// standard circuit breakers, reading the defaults of the missing fields
// from the configuration, and the time from the clock. It is the provider
// registered by the bundle returned from [NewBundle].
func newCreator(
	config flam.IConfig,
	clock flam.IClock,
) flam.IClientCircuitBreakerCreator {
	return &creator{
		config: config,
		clock:  clock,
	}
}

// creator is the [flam.IClientCircuitBreakerCreator] of the
// configurations with the [Driver] driver.
type creator struct {
	config flam.IConfig
	clock  flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the closed circuit breaker of the configuration.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IClientCircuitBreaker, error) {
	id := bag.String(factory.ConfigFieldId)

	failureThreshold, ok := c.field(bag, ConfigFieldFailureThreshold, DefaultFailureThresholdConfigPath, defaultFailureThreshold).(int)
	if !ok || failureThreshold <= 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldFailureThreshold)
	}
	openTimeout, ok := settings.Duration(c.field(bag, ConfigFieldOpenTimeout, DefaultOpenTimeoutConfigPath, defaultOpenTimeout))
	if !ok || openTimeout == 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldOpenTimeout)
	}
	halfOpenRequests, ok := c.field(bag, ConfigFieldHalfOpenRequests, DefaultHalfOpenRequestsConfigPath, defaultHalfOpenRequests).(int)
	if !ok || halfOpenRequests <= 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldHalfOpenRequests)
	}

	return &breaker{
		id:               id,
		clock:            c.clock,
		failureThreshold: failureThreshold,
		openTimeout:      openTimeout,
		halfOpenRequests: halfOpenRequests,
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create standard client circuit breaker"

// field returns the value of the field of the breaker configuration, or
// else the one stored in the configuration at the default path, or else
// the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}
