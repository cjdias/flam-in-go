package standard

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.loggers.standard"

const (
	// Id is the id of the default logger adaptor configured by the bundle
	// returned from [NewBundle], and the default of the logger adaptor of
	// the servers.
	Id = "standard"

	// Driver is the driver name of the standard logger adaptors, held by
	// the [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.loggers.driver.standard"
)

// Fields of a standard logger adaptor configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevelStart is the field holding the level of the entries
	// reporting the start of a server, a whole number, a [flam.LogLevel]
	// included, or a level name such as "info", as [flam.LogLevel] From
	// reads it.
	ConfigFieldLevelStart = "level_start"

	// ConfigFieldLevelError is the field holding the level of the entries
	// reporting the failure of a server, read as [ConfigFieldLevelStart].
	ConfigFieldLevelError = "level_error"

	// ConfigFieldLevelShutdown is the field holding the level of the
	// entries reporting the shutdown of a server, read as
	// [ConfigFieldLevelStart].
	ConfigFieldLevelShutdown = "level_shutdown"

	// ConfigFieldLevelDone is the field holding the level of the entries
	// reporting the end of a server, read as [ConfigFieldLevelStart].
	ConfigFieldLevelDone = "level_done"
)

// Configuration paths of the values taken by the fields missing from an
// adaptor configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.rest" unless changed.
	DefaultChannelConfigPath = "flam.rest.defaults.loggers.standard.channel"

	// DefaultLevelStartConfigPath is the path of the default of
	// [ConfigFieldLevelStart], "info" unless changed.
	DefaultLevelStartConfigPath = "flam.rest.defaults.loggers.standard.level_start"

	// DefaultLevelErrorConfigPath is the path of the default of
	// [ConfigFieldLevelError], "error" unless changed.
	DefaultLevelErrorConfigPath = "flam.rest.defaults.loggers.standard.level_error"

	// DefaultLevelShutdownConfigPath is the path of the default of
	// [ConfigFieldLevelShutdown], "info" unless changed.
	DefaultLevelShutdownConfigPath = "flam.rest.defaults.loggers.standard.level_shutdown"

	// DefaultLevelDoneConfigPath is the path of the default of
	// [ConfigFieldLevelDone], "info" unless changed.
	DefaultLevelDoneConfigPath = "flam.rest.defaults.loggers.standard.level_done"
)

// Context fields of the log entries.
const (
	// ContextFieldServer holds the id of the server.
	ContextFieldServer = "server"

	// ContextFieldAddress holds the address the server listens on, for a
	// start.
	ContextFieldAddress = "address"

	// ContextFieldError holds the error of the server, for a failure.
	ContextFieldError = "error"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the adaptor.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of an adaptor configuration.
const (
	defaultChannel       = "flam.rest"
	defaultLevelStart    = flam.LogInfo
	defaultLevelError    = flam.LogError
	defaultLevelShutdown = flam.LogInfo
	defaultLevelDone     = flam.LogInfo
)
