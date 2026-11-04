package tracing

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.tracing"

// Configuration paths and container group of the tracer provider and its
// exporters.
const (
	// EnabledConfigPath is the configuration path of a bool telling
	// whether the tracing is enabled. When false, the bundle provides a
	// no-op tracer provider, whose spans record and export nothing, and
	// creates no exporter. It defaults to true.
	EnabledConfigPath = "flam.tracing.enabled"

	// GlobalConfigPath is the configuration path of a bool telling whether
	// the bundle, when the application boots, sets its tracer provider and
	// its propagator as the global ones of OpenTelemetry, returned by
	// otel.GetTracerProvider and otel.GetTextMapPropagator, for the
	// instrumentations reading them. It defaults to true.
	GlobalConfigPath = "flam.tracing.global"

	// ServiceNameConfigPath is the configuration path of the name of the
	// service, the "service.name" attribute of the resource of the spans,
	// a string. Unless set, or empty, the resource keeps the name
	// OpenTelemetry gives it, from the OTEL_SERVICE_NAME environment
	// variable, or else from the name of the executable.
	ServiceNameConfigPath = "flam.tracing.service.name"

	// ServiceVersionConfigPath is the configuration path of the version of
	// the service, the "service.version" attribute of the resource of the
	// spans, a string, left out unless set, or empty.
	ServiceVersionConfigPath = "flam.tracing.service.version"

	// ResourceConfigPath is the configuration path of the bag of the
	// attributes of the resource of the spans, along with the service
	// name and version, which take precedence over the ones of the bag.
	// Its values are strings, bools, ints, int64s or float64s, and the
	// nested bags are flattened, the keys of a nested bag being prefixed
	// with the key of the bag and a dot, as a dotted key, such as
	// "deployment.environment.name", is stored nested in the configuration.
	ResourceConfigPath = "flam.tracing.resource"

	// SamplerConfigPath is the configuration path of the sampler of the
	// spans: [SamplerAlways], the default, [SamplerNever] or
	// [SamplerRatio].
	SamplerConfigPath = "flam.tracing.sampler.type"

	// SamplerRatioConfigPath is the configuration path of the ratio of the
	// traces the [SamplerRatio] sampler samples, a float64, or an int, from
	// 0 to 1. It defaults to 1.
	SamplerRatioConfigPath = "flam.tracing.sampler.ratio"

	// SamplerParentBasedConfigPath is the configuration path of a bool
	// telling whether the spans having a parent are sampled when their
	// parent is, the sampler only deciding for the root spans, so that a
	// trace is sampled whole. It defaults to true.
	SamplerParentBasedConfigPath = "flam.tracing.sampler.parent_based"

	// ExporterConfigPath is the configuration path holding one exporter
	// configuration per exporter id, as read by the exporter factory.
	ExporterConfigPath = "flam.tracing.exporters"

	// ExporterCreatorGroup is the container group the exporter driver
	// bundles provide their [flam.ITracingExporterCreator] in, collected
	// by the exporter factory.
	ExporterCreatorGroup = "flam.tracing.exporters.creator"

	// ExportConfigPath is the configuration path of the list of the ids of
	// the exporters, in the exporter factory, the tracer provider sends its
	// spans to, a []string or a []any of strings. It defaults to none: the
	// spans are then recorded, for the instrumentations reading them, but
	// exported nowhere.
	ExportConfigPath = "flam.tracing.export"

	// BatchConfigPath is the configuration path of a bool telling whether
	// the spans are sent to the exporters in batches, in the background,
	// rather than one at a time, as each ends. It defaults to true.
	BatchConfigPath = "flam.tracing.batch"

	// ShutdownTimeoutConfigPath is the configuration path of the longest
	// time the shutdown of the tracer provider, when the application
	// closes, takes to export the spans left, a non-negative duration,
	// such as "5s", 0 waiting as long as it takes. It defaults to 5s.
	ShutdownTimeoutConfigPath = "flam.tracing.shutdown_timeout"
)

// Samplers held by [SamplerConfigPath].
const (
	// SamplerAlways samples every trace.
	SamplerAlways = "always"

	// SamplerNever samples no trace.
	SamplerNever = "never"

	// SamplerRatio samples the ratio of the traces of
	// [SamplerRatioConfigPath], deciding from their trace id.
	SamplerRatio = "ratio"
)

// Context fields of the [*flam.Error] values returned by the package.
const (
	// ErrorFieldID holds the id of an exporter.
	ErrorFieldID = "id"

	// ErrorFieldField holds the configuration path of an invalid value.
	ErrorFieldField = "field"
)

// Defaults of the configuration of the tracer provider.
const (
	defaultEnabled         = true
	defaultGlobal          = true
	defaultSampler         = SamplerAlways
	defaultSamplerRatio    = 1.0
	defaultParentBased     = true
	defaultBatch           = true
	defaultShutdownTimeout = 5 * time.Second
)
