package endpoints

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.metrics.endpoints"

// Configuration paths of the metrics endpoint, read when the register is
// created.
const (
	// GroupConfigPath is the configuration path of the group of the
	// servers serving the endpoint, "metrics" unless set.
	GroupConfigPath = "flam.metrics.endpoint.group"

	// PathConfigPath is the configuration path of the path of the
	// endpoint, served to the GET requests, "/metrics" unless set.
	PathConfigPath = "flam.metrics.endpoint.path"

	// IdConfigPath is the configuration path of the id of the endpoint,
	// a whole number, 0 unless set.
	IdConfigPath = "flam.metrics.endpoint.id"
)

// Defaults of the endpoint configuration.
const (
	defaultGroup = "metrics"
	defaultPath  = "/metrics"
	defaultId    = 0
)
