package broker

import (
	"github.com/cjdias/flam-in-go/pkg/flam/health"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.checks.broker"

// Driver is the driver name of the broker checks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.health.checks.driver.broker"

// Fields of a broker check configuration, along with the fields shared by
// the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/health].
const (
	// ConfigFieldBroker is the field holding the id of the broker the
	// check checks, in the broker factory, the check id unless set.
	ConfigFieldBroker = "broker"

	// ConfigFieldTopic is the field holding the topic the check publishes
	// an empty message on, when the broker cannot be pinged, as its driver
	// does not implement [github.com/cjdias/flam-in-go/pkg/flam.IHealthPinger],
	// the check then only taking the broker from the factory unless set.
	ConfigFieldTopic = "topic"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the checks.
const (
	// ErrorFieldID holds the id of the check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldBroker holds the id of the broker of the check.
	ErrorFieldBroker = "broker"

	// ErrorFieldTopic holds the topic the check publishes on.
	ErrorFieldTopic = "topic"
)

// Defaults of the fields of a check configuration.
const (
	defaultKind  = health.KindReadiness
	defaultTopic = ""
)
