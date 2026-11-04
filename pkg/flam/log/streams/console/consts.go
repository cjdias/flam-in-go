package console

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.streams.console"

// Driver is the driver name of the console streams, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.log.streams.driver.console"

// Fields of a console stream configuration.
const (
	// ConfigFieldLevel is the field holding the least severe level the
	// stream keeps, a whole number, a [flam.LogLevel] included, or a level
	// name such as "info", as [flam.LogLevel] From reads it.
	ConfigFieldLevel = "level"

	// ConfigFieldChannels is the field holding the list of the channels
	// the stream listens to, "*" listening to all of them. It defaults to
	// none: the stream then keeps only the broadcast entries.
	ConfigFieldChannels = "channels"

	// ConfigFieldSerializer is the field holding the id of the serializer
	// of the stream.
	ConfigFieldSerializer = "serializer"

	// ConfigFieldOutput is the field holding the output of the stream,
	// [OutputStdout] or [OutputStderr], whatever its case.
	ConfigFieldOutput = "output"
)

// Outputs of a console stream.
const (
	// OutputStdout is the output writing to the standard output.
	OutputStdout = "stdout"

	// OutputStderr is the output writing to the standard error.
	OutputStderr = "stderr"
)

// Configuration paths of the values taken by the fields missing from a
// stream configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.log.defaults.streams.console.level"

	// DefaultSerializerConfigPath is the path of the default of
	// [ConfigFieldSerializer], the serializer of
	// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify.Id]
	// unless changed.
	DefaultSerializerConfigPath = "flam.log.defaults.streams.console.serializer"

	// DefaultOutputConfigPath is the path of the default of
	// [ConfigFieldOutput], [OutputStdout] unless changed.
	DefaultOutputConfigPath = "flam.log.defaults.streams.console.output"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the stream.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldSerializer holds the id of the serializer of the stream.
	ErrorFieldSerializer = "serializer"
)

// Defaults of the fields of a stream configuration.
const (
	defaultLevel      = flam.LogInfo
	defaultSerializer = stringify.Id
	defaultOutput     = OutputStdout
)
