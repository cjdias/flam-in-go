package rotatingfile

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.streams.rotating_file"

// Driver is the driver name of the rotating file streams, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.log.streams.driver.rotating_file"

// Fields of a rotating file stream configuration.
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

	// ConfigFieldPath is the required field holding the path of the files
	// on their disk, holding once [DatePlaceholder], replaced by the date
	// of the entries written to the file.
	ConfigFieldPath = "path"
)

const (
	// DatePlaceholder is the placeholder of the path of the files, replaced
	// by the date, in the [DateFormat] layout.
	DatePlaceholder = "%s"

	// DateFormat is the layout, as for time.Time Format, of the date
	// replacing [DatePlaceholder] in the path of the files.
	DateFormat = "2006-01-02"
)

// Configuration paths of the values taken by the fields missing from a
// stream configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.log.defaults.streams.rotating_file.level"

	// DefaultSerializerConfigPath is the path of the default of
	// [ConfigFieldSerializer], the serializer of
	// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers/json.Id]
	// unless changed.
	DefaultSerializerConfigPath = "flam.log.defaults.streams.rotating_file.serializer"

	// DefaultDiskConfigPath is the path of the default of
	// [ConfigFieldDisk], the disk of
	// [github.com/cjdias/flam-in-go/pkg/flam/disk/disks/local.Id] unless
	// changed.
	DefaultDiskConfigPath = "flam.log.defaults.streams.rotating_file.disk"
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

	// ErrorFieldPath holds the path of a file, its date replaced.
	ErrorFieldPath = "path"
)

// Defaults of the fields of a stream configuration.
const (
	defaultLevel      = flam.LogInfo
	defaultSerializer = json.Id
	defaultDisk       = local.Id
)
