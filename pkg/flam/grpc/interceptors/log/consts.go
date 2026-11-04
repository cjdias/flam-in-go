package log

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc.interceptors.log"

const (
	// Id is the id of the interceptor configured by the bundle returned
	// from [NewBundle].
	Id = "log"

	// Driver is the driver name of the log interceptors, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.grpc.interceptors.driver.log"

	// CodePanic is the value of [ContextFieldCode] for the calls whose
	// handler panicked.
	CodePanic = "panic"
)

// Fields of a log interceptor configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevel is the field holding the level of the entries of
	// the calls ending without a server error, a whole number, a
	// [flam.LogLevel] included, or a level name such as "info", as
	// [flam.LogLevel] From reads it.
	ConfigFieldLevel = "level"

	// ConfigFieldLevelError is the field holding the level of the entries
	// of the calls ending with a server error, as
	// [github.com/cjdias/flam-in-go/pkg/flam/grpc.IsServerError] tells,
	// and of the handlers that panicked, read as [ConfigFieldLevel].
	ConfigFieldLevelError = "level_error"
)

// Configuration paths of the values taken by the fields missing from an
// interceptor configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.grpc" unless changed.
	DefaultChannelConfigPath = "flam.grpc.defaults.interceptors.log.channel"

	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.grpc.defaults.interceptors.log.level"

	// DefaultLevelErrorConfigPath is the path of the default of
	// [ConfigFieldLevelError], "error" unless changed.
	DefaultLevelErrorConfigPath = "flam.grpc.defaults.interceptors.log.level_error"
)

// Context fields of the log entries.
const (
	// ContextFieldMethod holds the full method of the call, such as
	// "/pkg.Users/Get".
	ContextFieldMethod = "method"

	// ContextFieldCode holds the name of the code of the status of the
	// call, such as "OK" or "NotFound", or [CodePanic].
	ContextFieldCode = "code"

	// ContextFieldDuration holds the time taken to serve the call, as a
	// [time.Duration].
	ContextFieldDuration = "duration"

	// ContextFieldPeer holds the address of the client, left out when the
	// context of the call holds none.
	ContextFieldPeer = "peer"

	// ContextFieldError holds the error of the call, left out when it
	// succeeded or its handler panicked.
	ContextFieldError = "error"

	// ContextFieldPanicked holds true when the handler panicked, and is
	// left out otherwise.
	ContextFieldPanicked = "panicked"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the interceptor.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of an interceptor configuration.
const (
	defaultChannel    = "flam.grpc"
	defaultLevel      = flam.LogInfo
	defaultLevelError = flam.LogError
)
