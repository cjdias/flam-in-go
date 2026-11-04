//go:build !windows && !plan9

package syslog

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.streams.syslog"

// Driver is the driver name of the syslog streams, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.log.streams.driver.syslog"

// Fields of a syslog stream configuration.
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

	// ConfigFieldNetwork is the field holding the network of the daemon,
	// such as "udp", "tcp" or "unixgram", as given to [net.Dial], or the
	// empty string for the local daemon.
	ConfigFieldNetwork = "network"

	// ConfigFieldAddress is the field holding the address of the daemon
	// on its network, such as "localhost:514", ignored for the local
	// daemon.
	ConfigFieldAddress = "address"

	// ConfigFieldTag is the field holding the tag of the messages, the
	// empty string standing for the name of the program, os.Args[0].
	ConfigFieldTag = "tag"

	// ConfigFieldFacility is the field holding the facility of the
	// messages, whatever its case: "kern", "user", "mail", "daemon",
	// "auth", "syslog", "lpr", "news", "uucp", "cron", "authpriv", "ftp",
	// or "local0" to "local7".
	ConfigFieldFacility = "facility"
)

// Configuration paths of the values taken by the fields missing from a
// stream configuration.
const (
	// DefaultLevelConfigPath is the path of the default of
	// [ConfigFieldLevel], "info" unless changed.
	DefaultLevelConfigPath = "flam.log.defaults.streams.syslog.level"

	// DefaultSerializerConfigPath is the path of the default of
	// [ConfigFieldSerializer], the serializer of
	// [github.com/cjdias/flam-in-go/pkg/flam/log/serializers/stringify.Id]
	// unless changed.
	DefaultSerializerConfigPath = "flam.log.defaults.streams.syslog.serializer"

	// DefaultNetworkConfigPath is the path of the default of
	// [ConfigFieldNetwork], unset unless configured, then the local
	// daemon.
	DefaultNetworkConfigPath = "flam.log.defaults.streams.syslog.network"

	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], unset unless configured, then empty.
	DefaultAddressConfigPath = "flam.log.defaults.streams.syslog.address"

	// DefaultTagConfigPath is the path of the default of [ConfigFieldTag],
	// unset unless configured, then the name of the program.
	DefaultTagConfigPath = "flam.log.defaults.streams.syslog.tag"

	// DefaultFacilityConfigPath is the path of the default of
	// [ConfigFieldFacility], "user" unless changed.
	DefaultFacilityConfigPath = "flam.log.defaults.streams.syslog.facility"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the stream.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldSerializer holds the id of the serializer of the stream.
	ErrorFieldSerializer = "serializer"

	// ErrorFieldNetwork holds the network of the daemon.
	ErrorFieldNetwork = "network"

	// ErrorFieldAddress holds the address of the daemon.
	ErrorFieldAddress = "address"
)

// Defaults of the fields of a stream configuration.
const (
	defaultLevel      = flam.LogInfo
	defaultSerializer = stringify.Id
	defaultFacility   = "user"
)
