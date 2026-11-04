package cache

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache"

// Configuration paths and container groups of the serializers, key
// generators and adaptors.
const (
	// SerializerConfigPath is the configuration path holding one
	// serializer configuration per serializer id, as read by the
	// serializer factory.
	SerializerConfigPath = "flam.cache.serializers"

	// SerializerCreatorGroup is the container group the serializer driver
	// bundles provide their [flam.ICacheSerializerCreator] in, collected
	// by the serializer factory.
	SerializerCreatorGroup = "flam.cache.serializers.creator"

	// KeyGeneratorConfigPath is the configuration path holding one key
	// generator configuration per key generator id, as read by the key
	// generator factory.
	KeyGeneratorConfigPath = "flam.cache.key_generators"

	// KeyGeneratorCreatorGroup is the container group the key generator
	// driver bundles provide their [flam.ICacheKeyGeneratorCreator] in,
	// collected by the key generator factory.
	KeyGeneratorCreatorGroup = "flam.cache.key_generators.creator"

	// AdaptorConfigPath is the configuration path holding one adaptor
	// configuration per adaptor id, as read by the adaptor factory.
	AdaptorConfigPath = "flam.cache.adaptors"

	// AdaptorCreatorGroup is the container group the adaptor driver
	// bundles provide their [flam.ICacheAdaptorCreator] in, collected by
	// the adaptor factory.
	AdaptorCreatorGroup = "flam.cache.adaptors.creator"
)

// Fields of an adaptor configuration shared by the adaptor drivers.
const (
	// ConfigFieldSerializer is the field holding the id of the serializer
	// of the adaptor, in the serializer factory.
	ConfigFieldSerializer = "serializer"

	// ConfigFieldKeyGenerator is the field holding the id of the key
	// generator of the adaptor, in the key generator factory.
	ConfigFieldKeyGenerator = "key_generator"
)

// Configuration paths of the values taken by the shared fields missing
// from an adaptor configuration.
const (
	// DefaultSerializerConfigPath is the path of the default of
	// [ConfigFieldSerializer], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/cache/serializers/json] to
	// its serializer.
	DefaultSerializerConfigPath = "flam.cache.defaults.adaptors.serializer"

	// DefaultKeyGeneratorConfigPath is the path of the default of
	// [ConfigFieldKeyGenerator], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field]
	// to its key generator.
	DefaultKeyGeneratorConfigPath = "flam.cache.defaults.adaptors.key_generator"
)

// Context fields of the [*flam.Error] values returned by the cache
// drivers.
const (
	// ErrorFieldID holds the id of a serializer, key generator or
	// adaptor.
	ErrorFieldID = "id"

	// ErrorFieldType holds the type of the object of an error, as
	// formatted by fmt %T.
	ErrorFieldType = "type"

	// ErrorFieldKey holds the key of the object of an error.
	ErrorFieldKey = "key"
)
