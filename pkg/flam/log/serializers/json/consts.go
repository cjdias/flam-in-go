package json

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.log.serializers.json"

const (
	// Id is the id of the default serializer configured by the bundle
	// returned from [NewBundle].
	Id = "json"

	// Driver is the driver name of the JSON serializers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.log.serializers.driver.json"

	// ConfigFieldFormat is the field of the serializer configuration
	// holding the layout, as for time.Time Format, of the entry
	// timestamps. It defaults to [DefaultFormat].
	ConfigFieldFormat = "format"

	// DefaultFormat is the default layout of the entry timestamps, with
	// milliseconds and the zone offset.
	DefaultFormat = "2006-01-02T15:04:05.000-0700"
)

// Fields of the JSON object of an entry, set over the context values of
// the same keys.
const (
	// FieldTime holds the timestamp of the entry, formatted.
	FieldTime = "time"

	// FieldLevel holds the level of the entry, in upper case.
	FieldLevel = "level"

	// FieldMessage holds the message of the entry.
	FieldMessage = "message"

	// FieldContextError holds, in place of the context, the error of
	// encoding/json when the context cannot be encoded.
	FieldContextError = "context_error"
)
