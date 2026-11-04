package recovery

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.grpc.interceptors.recovery"

const (
	// Id is the id of the interceptor configured by the bundle returned
	// from [NewBundle].
	Id = "recovery"

	// Driver is the driver name of the recovery interceptors, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.grpc.interceptors.driver.recovery"

	// Message is the message of the status of the calls whose handler
	// panicked, of the [google.golang.org/grpc/codes.Internal] code,
	// telling the client nothing of the panic.
	Message = "internal error"
)

// Fields of a recovery interceptor configuration.
const (
	// ConfigFieldChannel is the field holding the log channel of the
	// entries.
	ConfigFieldChannel = "channel"

	// ConfigFieldLevel is the field holding the level of the entries, a
	// whole number, a [flam.LogLevel] included, or a level name such as
	// "error", as [flam.LogLevel] From reads it.
	ConfigFieldLevel = "level"
)

// Configuration paths of the values taken by the fields missing from an
// interceptor configuration.
const (
	// DefaultChannelConfigPath is the path of the default of
	// [ConfigFieldChannel], "flam.grpc" unless changed.
	DefaultChannelConfigPath = "flam.grpc.defaults.interceptors.recovery.channel"

	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "error" unless changed.
	DefaultLevelConfigPath = "flam.grpc.defaults.interceptors.recovery.level"
)

// Context fields of the log entries.
const (
	// ContextFieldMethod holds the full method of the call, such as
	// "/pkg.Users/Get".
	ContextFieldMethod = "method"

	// ContextFieldPanic holds the value of the panic, as text.
	ContextFieldPanic = "panic"

	// ContextFieldStack holds the stack of the goroutine of the handler
	// that panicked, as text.
	ContextFieldStack = "stack"
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
	defaultChannel = "flam.grpc"
	defaultLevel   = flam.LogError
)
