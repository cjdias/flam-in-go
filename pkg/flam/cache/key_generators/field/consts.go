package field

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache.key_generators.field"

const (
	// Id is the id of the default key generator configured by the bundle
	// returned from [NewBundle].
	Id = "field"

	// Driver is the driver name of the field key generators, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.cache.key_generators.driver.field"
)

// Fields of a field key generator configuration.
const (
	// ConfigFieldField is the field holding the name of the struct field
	// of the objects whose value is their key, which may be promoted from
	// an embedded struct, "Id" unless set, as for
	// [github.com/cjdias/flam-in-go/pkg/flam.Record].
	ConfigFieldField = "field"

	// ConfigFieldPrefix is the field holding the prefix of the keys, such
	// as "app:", none unless set.
	ConfigFieldPrefix = "prefix"

	// ConfigFieldType is the field holding whether the keys hold the type
	// of the objects, named by its package and name, such as
	// "models.User", so that objects of different types share no key,
	// unless both their packages and names are the same, a bool, true
	// unless set. The objects of an unnamed type, such as an anonymous
	// struct, cannot then be keyed.
	ConfigFieldType = "type"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the key generator.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a key generator configuration.
const (
	defaultField  = "Id"
	defaultPrefix = ""
	defaultType   = true
)
