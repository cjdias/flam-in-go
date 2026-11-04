package file

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.streams.file"

// Driver is the driver name of the file streams, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.log.streams.driver.file"

// Fields of a file stream configuration.
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

	// ConfigFieldDisk is the field holding the id of the disk of the file.
	ConfigFieldDisk = "disk"

	// ConfigFieldPath is the required field holding the path of the file
	// on its disk.
	ConfigFieldPath = "path"
)

// Configuration paths of the values taken by the fields missing from a
// stream configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.log.defaults.streams.file.level"

	// DefaultSerializerConfigPath is the path of the default of
	// [ConfigFieldSerializer], the serializer of
	// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json.Id]
	// unless changed.
	DefaultSerializerConfigPath = "flam.log.defaults.streams.file.serializer"

	// DefaultDiskConfigPath is the path of the default of
	// [ConfigFieldDisk], the disk of
	// [github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local.Id] unless
	// changed.
	DefaultDiskConfigPath = "flam.log.defaults.streams.file.disk"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the stream.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldSerializer holds the id of the serializer of the stream.
	ErrorFieldSerializer = "serializer"

	// ErrorFieldDisk holds the id of the disk of the file.
	ErrorFieldDisk = "disk"

	// ErrorFieldPath holds the path of the file.
	ErrorFieldPath = "path"
)

// Defaults of the fields of a stream configuration.
const (
	defaultLevel      = flam.LogInfo
	defaultSerializer = json.Id
	defaultDisk       = local.Id
)
