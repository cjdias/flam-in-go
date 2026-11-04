package invalidating

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache.adaptors.invalidating"

// Driver is the driver name of the invalidating adaptors, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.cache.adaptors.driver.invalidating"

// Fields of an invalidating adaptor configuration, which, unlike the
// other adaptor drivers, takes no serializer nor key generator: those of
// the adaptor it wraps.
const (
	// ConfigFieldAdaptor is the field holding the id of the adaptor
	// wrapped, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.ICacheAdaptorFactory], one
	// implementing
	// [github.com/cjdias/flam-in-go/pkg/flam.ICacheKeyedAdaptor], other
	// than the invalidating adaptor itself.
	ConfigFieldAdaptor = "adaptor"

	// ConfigFieldBroker is the field holding the id of the broker
	// carrying the keys removed, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.IBrokerFactory].
	ConfigFieldBroker = "broker"

	// ConfigFieldTopic is the field holding the topic the keys removed are
	// published on, a non-empty string, [TopicPrefix] followed by the id
	// of the adaptor unless set.
	ConfigFieldTopic = "topic"

	// ConfigFieldPublishSet is the field holding whether the keys of the
	// objects stored are published too, a bool, [DefaultPublishSet]
	// unless set.
	ConfigFieldPublishSet = "publish_set"
)

// Configuration paths of the values taken by the fields missing from an
// adaptor configuration.
const (
	// DefaultAdaptorConfigPath is the path of the default of
	// [ConfigFieldAdaptor], unset unless configured.
	DefaultAdaptorConfigPath = "flam.cache.defaults.adaptors.invalidating.adaptor"

	// DefaultBrokerConfigPath is the path of the default of
	// [ConfigFieldBroker], unset unless configured.
	DefaultBrokerConfigPath = "flam.cache.defaults.adaptors.invalidating.broker"

	// DefaultTopicConfigPath is the path of the default of
	// [ConfigFieldTopic], unset unless configured.
	DefaultTopicConfigPath = "flam.cache.defaults.adaptors.invalidating.topic"

	// DefaultPublishSetConfigPath is the path of the default of
	// [ConfigFieldPublishSet], unset unless configured.
	DefaultPublishSetConfigPath = "flam.cache.defaults.adaptors.invalidating.publish_set"
)

// Built-in defaults of the fields missing from an adaptor configuration
// and from the configuration at their default paths.
const (
	// TopicPrefix is the prefix of the built-in default of
	// [ConfigFieldTopic], followed by the id of the adaptor, so that the
	// adaptors of one id share their topic across the processes, and the
	// adaptors of different ids do not.
	TopicPrefix = "flam.cache.invalidations."

	// DefaultPublishSet is the built-in default of
	// [ConfigFieldPublishSet].
	DefaultPublishSet = true
)

// HeaderInstance is the header of the messages an adaptor publishes,
// holding the random id of the adaptor, so that it ignores its own
// messages.
const HeaderInstance = "flam-cache-instance"

// Context fields of the [*flam.Error] values returned by the creator and
// the adaptors, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/cache].
const (
	// ErrorFieldID holds the id of the adaptor.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldAdaptor holds the id of the adaptor wrapped.
	ErrorFieldAdaptor = "adaptor"

	// ErrorFieldBroker holds the id of the broker.
	ErrorFieldBroker = "broker"

	// ErrorFieldTopic holds the topic of the adaptor.
	ErrorFieldTopic = "topic"
)
