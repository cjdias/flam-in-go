package health

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health"

// Configuration paths and container groups of the checks and of the
// registry.
const (
	// CheckConfigPath is the configuration path holding one check
	// configuration per check id, as read by the check factory.
	CheckConfigPath = "flam.health.checks"

	// CheckCreatorGroup is the container group the check driver bundles
	// provide their [flam.IHealthCheckCreator] in, collected by the check
	// factory.
	CheckCreatorGroup = "flam.health.checks.creator"

	// CheckGroup is the container group the application provides its
	// [flam.IHealthCheck] values in, added to the registry when the
	// application boots.
	CheckGroup = "flam.health.checks"

	// TimeoutConfigPath is the configuration path of the time a probe
	// waits for each of its checks, a duration such as "2s", after which
	// the check fails, 0 waiting as long as the context of the probe
	// allows. It defaults to 2s.
	TimeoutConfigPath = "flam.health.timeout"

	// CacheConfigPath is the configuration path of the time the result of
	// a check is reused by the probes, a duration such as "1s", 0 running
	// the check for every probe. It defaults to 0.
	CacheConfigPath = "flam.health.cache"
)

// Fields of a check configuration shared by the drivers of
// [github.com/cjdias/flam-in-go/pkg/flam/health/checks].
const (
	// ConfigFieldKind is the field holding the probes the check counts
	// for, read by [ParseKind]: [KindLiveness], [KindReadiness] or
	// [KindBoth], each driver telling its default.
	ConfigFieldKind = "kind"
)

// Configuration paths of the values taken by the fields missing from a
// check configuration, whatever its driver.
const (
	// DefaultKindConfigPath is the path of the default of
	// [ConfigFieldKind], unset unless set by the application: the checks
	// then take the default of their driver.
	DefaultKindConfigPath = "flam.health.defaults.checks.kind"
)

// Names of the kinds of the checks in the configuration, read by
// [ParseKind].
const (
	// KindLiveness names [flam.HealthLiveness].
	KindLiveness = "liveness"

	// KindReadiness names [flam.HealthReadiness].
	KindReadiness = "readiness"

	// KindBoth names [flam.HealthBoth].
	KindBoth = "both"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of a check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the path of an invalid configuration value.
	ErrorFieldField = "field"

	// ErrorFieldKind holds the invalid kind of a check.
	ErrorFieldKind = "kind"

	// ErrorFieldPanic holds the value a check panicked with.
	ErrorFieldPanic = "panic"
)

// Defaults of the registry.
const (
	defaultTimeout = 2 * time.Second
	defaultCache   = time.Duration(0)
)
