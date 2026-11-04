package standard

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.client.breakers.standard"

const (
	// Id is the id of the default circuit breaker configured by the bundle
	// returned from [NewBundle].
	Id = "standard"

	// Driver is the driver name of the standard circuit breakers, held by
	// the [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.client.breakers.driver.standard"
)

// Fields of a standard circuit breaker configuration.
const (
	// ConfigFieldFailureThreshold is the field holding the number of
	// consecutive failures opening the circuit, a positive whole number.
	ConfigFieldFailureThreshold = "failure_threshold"

	// ConfigFieldOpenTimeout is the field holding the time the circuit
	// stays open before becoming half open, a positive duration such as
	// "30s".
	ConfigFieldOpenTimeout = "open_timeout"

	// ConfigFieldHalfOpenRequests is the field holding the number of trial
	// requests let through by a half open circuit, all of which must
	// succeed to close it, a positive whole number.
	ConfigFieldHalfOpenRequests = "half_open_requests"
)

// Configuration paths of the values taken by the fields missing from a
// circuit breaker configuration.
const (
	// DefaultFailureThresholdConfigPath is the path of the default of
	// [ConfigFieldFailureThreshold], 5 unless set.
	DefaultFailureThresholdConfigPath = "flam.client.defaults.breakers.standard.failure_threshold"

	// DefaultOpenTimeoutConfigPath is the path of the default of
	// [ConfigFieldOpenTimeout], "30s" unless set.
	DefaultOpenTimeoutConfigPath = "flam.client.defaults.breakers.standard.open_timeout"

	// DefaultHalfOpenRequestsConfigPath is the path of the default of
	// [ConfigFieldHalfOpenRequests], 10 unless set.
	DefaultHalfOpenRequestsConfigPath = "flam.client.defaults.breakers.standard.half_open_requests"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the circuit breaker.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a circuit breaker configuration.
const (
	defaultFailureThreshold = 5
	defaultOpenTimeout      = 30 * time.Second
	defaultHalfOpenRequests = 10
)
