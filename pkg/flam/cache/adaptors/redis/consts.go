package redis

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache.adaptors.redis"

// Driver is the driver name of the redis adaptors, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.cache.adaptors.driver.redis"

// Fields of a redis adaptor configuration, along with the fields shared by
// the adaptor drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/cache].
const (
	// ConfigFieldConnection is the field holding the id of the connection
	// of the adaptor, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory].
	ConfigFieldConnection = "connection"
)

// Configuration paths of the values taken by the fields missing from an
// adaptor configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], unset unless configured.
	DefaultConnectionConfigPath = "flam.cache.defaults.adaptors.redis.connection"
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

	// ErrorFieldConnection holds the id of the connection of the adaptor.
	ErrorFieldConnection = "connection"
)
