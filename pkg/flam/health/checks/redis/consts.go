package redis

import (
	"github.com/cjdias/flam-in-go/pkg/flam/health"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.checks.redis"

// Driver is the driver name of the redis checks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.health.checks.driver.redis"

// Fields of a redis check configuration, along with the fields shared
// by the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/health].
const (
	// ConfigFieldConnection is the field holding the id of the connection
	// the check pings, in the connection factory, the check id unless
	// set.
	ConfigFieldConnection = "connection"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the checks.
const (
	// ErrorFieldID holds the id of the check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the connection of the check.
	ErrorFieldConnection = "connection"
)

// Defaults of the fields of a check configuration.
const (
	defaultKind = health.KindReadiness
)
