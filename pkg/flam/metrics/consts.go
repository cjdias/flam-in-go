package metrics

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.metrics"

// Configuration paths and container group of the collectors.
const (
	// CollectorGroup is the container group the application and the
	// collector bundles provide their [prometheus.Collector] values in,
	// registered on the registry when it is created.
	//
	// [prometheus.Collector]: https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#Collector
	CollectorGroup = "flam.metrics.collectors"

	// GoCollectorConfigPath is the configuration path of a bool telling
	// whether the registry holds the collector of the Go runtime metrics,
	// the go_* metrics. It defaults to true.
	GoCollectorConfigPath = "flam.metrics.collectors.go"

	// ProcessCollectorConfigPath is the configuration path of a bool
	// telling whether the registry holds the collector of the process
	// metrics, the process_* metrics, as far as the platform reports them.
	// It defaults to true.
	ProcessCollectorConfigPath = "flam.metrics.collectors.process"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldCollector holds the type of a collector, such as
	// "*prometheus.CounterVec".
	ErrorFieldCollector = "collector"
)

// Defaults of the registry configuration.
const (
	defaultGoCollector      = true
	defaultProcessCollector = true
)
