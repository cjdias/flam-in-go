package stringify

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.serializers.stringify"

const (
	// Id is the id of the default serializer configured by the bundle
	// returned from [NewBundle].
	Id = "stringify"

	// Driver is the driver name of the stringify serializers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.log.serializers.driver.stringify"

	// ConfigFieldFormat is the field of the serializer configuration
	// holding the layout, as for time.Time Format, of the entry
	// timestamps. It defaults to [DefaultFormat].
	ConfigFieldFormat = "format"

	// DefaultFormat is the default layout of the entry timestamps, with
	// milliseconds and the zone offset.
	DefaultFormat = "2006-01-02T15:04:05.000-0700"
)
