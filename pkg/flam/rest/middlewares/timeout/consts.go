package timeout

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.timeout"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "timeout"

	// Driver is the driver name of the timeout middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.timeout"
)

// Fields of a timeout middleware configuration.
const (
	// ConfigFieldTimeout is the field holding the time a handler may take
	// to serve a request, a [time.Duration] or a string such as "30s", of
	// more than 0.
	ConfigFieldTimeout = "timeout"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], "30s" unless changed.
	DefaultTimeoutConfigPath = "flam.rest.defaults.middlewares.timeout.timeout"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultTimeout = 30 * time.Second
)
