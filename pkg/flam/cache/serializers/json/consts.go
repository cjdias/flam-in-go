package json

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.cache.serializers.json"

const (
	// Id is the id of the default serializer configured by the bundle
	// returned from [NewBundle].
	Id = "json"

	// Driver is the driver name of the JSON serializers, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.cache.serializers.driver.json"
)
