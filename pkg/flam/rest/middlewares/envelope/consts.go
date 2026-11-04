package envelope

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.envelope"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "envelope"

	// Driver is the driver name of the envelope middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.envelope"
)

// Fields of a envelope middleware configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevel is the field holding the level of the entries, a
	// whole number, a [flam.LogLevel] included, or a level name such as
	// "error", as [flam.LogLevel] From reads it.
	ConfigFieldLevel = "level"

	// ConfigFieldExposeErrors is the field holding whether the message of
	// an error value of a panic is sent to the client, a bool: when false,
	// [ErrInternal] stands for it.
	ConfigFieldExposeErrors = "expose_errors"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.rest" unless changed.
	DefaultChannelConfigPath = "flam.rest.defaults.middlewares.envelope.channel"

	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "error" unless changed.
	DefaultLevelConfigPath = "flam.rest.defaults.middlewares.envelope.level"

	// DefaultExposeErrorsConfigPath is the path of the default of
	// [ConfigFieldExposeErrors], false unless changed.
	DefaultExposeErrorsConfigPath = "flam.rest.defaults.middlewares.envelope.expose_errors"
)

// Context fields of the log entries.
const (
	// ContextFieldEndpoint holds the id of the endpoint.
	ContextFieldEndpoint = "endpoint"

	// ContextFieldPattern holds the pattern of the endpoint.
	ContextFieldPattern = "pattern"

	// ContextFieldPanic holds the value the handler panicked with.
	ContextFieldPanic = "panic"

	// ContextFieldStack holds the stack of the handler when it panicked.
	ContextFieldStack = "stack"
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
	defaultChannel      = "flam.rest"
	defaultLevel        = flam.LogError
	defaultExposeErrors = false
)
