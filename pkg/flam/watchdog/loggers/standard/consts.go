package standard

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.watchdog.loggers.standard"

const (
	// Id is the id of the default logger adaptor configured by the bundle
	// returned from [NewBundle], and the default of the logger adaptor of
	// the processes.
	Id = "standard"

	// Driver is the driver name of the standard logger adaptors, held by
	// the [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.watchdog.loggers.driver.standard"
)

// Fields of a standard logger adaptor configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevelStart is the field holding the level of the entries
	// reporting the start of a process, a whole number, a [flam.LogLevel]
	// included, or a level name such as "info", as [flam.LogLevel] From
	// reads it.
	ConfigFieldLevelStart = "level_start"

	// ConfigFieldLevelError is the field holding the level of the entries
	// reporting the failure of a process, read as [ConfigFieldLevelStart].
	ConfigFieldLevelError = "level_error"

	// ConfigFieldLevelDone is the field holding the level of the entries
	// reporting the end of a process, read as [ConfigFieldLevelStart].
	ConfigFieldLevelDone = "level_done"
)

// Configuration paths of the values taken by the fields missing from an
// adaptor configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.watchdog" unless changed.
	DefaultChannelConfigPath = "flam.watchdog.defaults.loggers.standard.channel"

	// DefaultLevelStartConfigPath is the path of the default of
	// [ConfigFieldLevelStart], "info" unless changed.
	DefaultLevelStartConfigPath = "flam.watchdog.defaults.loggers.standard.level_start"

	// DefaultLevelErrorConfigPath is the path of the default of
	// [ConfigFieldLevelError], "error" unless changed.
	DefaultLevelErrorConfigPath = "flam.watchdog.defaults.loggers.standard.level_error"

	// DefaultLevelDoneConfigPath is the path of the default of
	// [ConfigFieldLevelDone], "info" unless changed.
	DefaultLevelDoneConfigPath = "flam.watchdog.defaults.loggers.standard.level_done"
)

// Context fields of the log entries.
const (
	// ContextFieldProcess holds the id of the process.
	ContextFieldProcess = "process"

	// ContextFieldError holds the error of the process.
	ContextFieldError = "error"

	// ContextFieldPanic holds the value the process panicked with, for an
	// error of a panic, as the log serializers keep the message of an
	// error only.
	ContextFieldPanic = "panic"

	// ContextFieldStack holds the stack of the process when it panicked,
	// for an error of a panic.
	ContextFieldStack = "stack"
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
	defaultChannel    = "flam.watchdog"
	defaultLevelStart = flam.LogInfo
	defaultLevelError = flam.LogError
	defaultLevelDone  = flam.LogInfo
)
