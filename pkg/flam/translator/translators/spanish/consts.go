package spanish

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.translator.translators.spanish"

const (
	// Id is the id of the default translator configured by the bundle
	// returned from [NewBundle].
	Id = "es"

	// Driver is the driver name of the Spanish translators, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.translator.translators.driver.spanish"
)
