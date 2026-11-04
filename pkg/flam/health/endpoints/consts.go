package endpoints

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.endpoints"

// Configuration paths of the endpoints.
const (
	// GroupConfigPath is the configuration path of the group of the
	// servers the endpoints are registered on, a non-empty string,
	// "health" unless set.
	GroupConfigPath = "flam.health.endpoints.group"

	// LivenessConfigPath is the configuration path of the path of the
	// endpoint of the liveness probe, a non-empty string, "/health/live"
	// unless set.
	LivenessConfigPath = "flam.health.endpoints.liveness"

	// ReadinessConfigPath is the configuration path of the path of the
	// endpoint of the readiness probe, a non-empty string,
	// "/health/ready" unless set.
	ReadinessConfigPath = "flam.health.endpoints.readiness"
)

// Ids of the endpoints, in the errors they answer with.
const (
	// LivenessEndpointId is the id of the endpoint of the liveness probe.
	LivenessEndpointId = 1

	// ReadinessEndpointId is the id of the endpoint of the readiness
	// probe.
	ReadinessEndpointId = 2
)

// ContextFieldReport is the field of the context of the envelope error
// of a failed probe holding its report, and so the extension of the
// problem it is written as.
const ContextFieldReport = "report"

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldField holds the path of an invalid configuration value.
	ErrorFieldField = "field"
)

// Defaults of the configuration of the endpoints.
const (
	defaultGroup     = "health"
	defaultLiveness  = "/health/live"
	defaultReadiness = "/health/ready"
)
