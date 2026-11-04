package memory

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache.adaptors.memory"

// Driver is the driver name of the memory adaptors, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.cache.adaptors.driver.memory"

// Fields of a memory adaptor configuration, along with the fields shared
// by the adaptor drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/cache].
const (
	// ConfigFieldMaxEntries is the field holding the maximum number of
	// values the adaptor holds, a whole number between 1 and 2147483647,
	// of any integer or float kind, or a string holding one.
	ConfigFieldMaxEntries = "max_entries"

	// ConfigFieldSweepPeriod is the field holding the period at which the
	// writes sweep the expired values, a time.Duration or a string such as
	// "1m", zero disabling the sweep.
	ConfigFieldSweepPeriod = "sweep_period"
)

// Configuration paths of the values taken by the fields missing from an
// adaptor configuration.
const (
	// DefaultMaxEntriesConfigPath is the path of the default of
	// [ConfigFieldMaxEntries], unset unless configured, then taking
	// [DefaultMaxEntries].
	DefaultMaxEntriesConfigPath = "flam.cache.defaults.adaptors.memory.max_entries"

	// DefaultSweepPeriodConfigPath is the path of the default of
	// [ConfigFieldSweepPeriod], unset unless configured, then taking
	// [DefaultSweepPeriod].
	DefaultSweepPeriodConfigPath = "flam.cache.defaults.adaptors.memory.sweep_period"
)

// Built-in defaults of the fields missing from an adaptor configuration
// and from the configuration at their default paths.
const (
	// DefaultMaxEntries is the built-in default of [ConfigFieldMaxEntries].
	DefaultMaxEntries = 10000

	// DefaultSweepPeriod is the built-in default of
	// [ConfigFieldSweepPeriod].
	DefaultSweepPeriod = time.Minute
)

// Context fields of the [*flam.Error] values returned by the creator and
// the adaptors, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/cache].
const (
	// ErrorFieldID holds the id of the adaptor.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldSerializer holds the id of the serializer of the adaptor.
	ErrorFieldSerializer = "serializer"

	// ErrorFieldKeyGenerator holds the id of the key generator of the
	// adaptor.
	ErrorFieldKeyGenerator = "key_generator"
)
