package tracing

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// exporterFactoryArgs are the dependencies of the exporter factory, taken
// from the container. The group tag repeats ExporterCreatorGroup, as a
// struct tag cannot use a constant.
type exporterFactoryArgs struct {
	dig.In

	Config   flam.IConfig
	Creators []flam.ITracingExporterCreator `group:"flam.tracing.exporters.creator"`
}

// newExporterFactory returns the exporter factory creating the exporters
// configured at ExporterConfigPath with the creators of the
// ExporterCreatorGroup group.
func newExporterFactory(
	args exporterFactoryArgs,
) (flam.ITracingExporterFactory, error) {
	return factory.New(args.Config.Bag(ExporterConfigPath), args.Creators...)
}

// newPropagator returns the propagator of the W3C trace context and
// baggage.
func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
}

// providerArgs are the dependencies of the tracer provider, taken from the
// container.
type providerArgs struct {
	dig.In

	Config    flam.IConfig
	Exporters flam.ITracingExporterFactory
}

// newProvider returns the no-op tracer provider when the tracing is
// disabled, or else the OpenTelemetry SDK tracer provider of the sampler
// and the resource of the configuration, sending its spans to the
// exporters of ExportConfigPath, taken from the exporter factory, in
// batches or one at a time. The exporters are all taken before the
// provider is created, so that a failure leaves no span processor
// running.
func newProvider(
	args providerArgs,
) (trace.TracerProvider, error) {
	config := args.Config
	enabled, ok := config.Get(EnabledConfigPath, defaultEnabled).(bool)
	if !ok {
		return nil, newErrField(providerNewMessage, EnabledConfigPath)
	}
	if !enabled {
		return noop.NewTracerProvider(), nil
	}

	sampler, err := providerSampler(config)
	if err != nil {
		return nil, err
	}
	resource, err := providerResource(config)
	if err != nil {
		return nil, err
	}
	batch, ok := config.Get(BatchConfigPath, defaultBatch).(bool)
	if !ok {
		return nil, newErrField(providerNewMessage, BatchConfigPath)
	}
	ids, ok := settings.Strings(config.Get(ExportConfigPath))
	if !ok {
		return nil, newErrField(providerNewMessage, ExportConfigPath)
	}
	exporters := make([]flam.ITracingExporter, 0, len(ids))
	for _, id := range ids {
		exporter, err := args.Exporters.Get(id)
		if err != nil {
			return nil, newErrJoined(ErrExporter, providerNewMessage, err).
				With(ErrorFieldID, id)
		}
		exporters = append(exporters, exporter)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sampler),
		sdktrace.WithResource(resource),
	}
	for _, exporter := range exporters {
		if batch {
			opts = append(opts, sdktrace.WithBatcher(exporter))
			continue
		}
		opts = append(opts, sdktrace.WithSyncer(exporter))
	}
	return sdktrace.NewTracerProvider(opts...), nil
}

// providerNewMessage is the message of the errors returned by
// newProvider.
const providerNewMessage = "unable to create tracer provider"

// providerSampler returns the sampler of the configuration, made parent
// based when configured so.
func providerSampler(
	config flam.IConfig,
) (sdktrace.Sampler, error) {
	var sampler sdktrace.Sampler
	switch config.Get(SamplerConfigPath, defaultSampler) {
	case SamplerAlways:
		sampler = sdktrace.AlwaysSample()
	case SamplerNever:
		sampler = sdktrace.NeverSample()
	case SamplerRatio:
		ratio, ok := providerRatio(config.Get(SamplerRatioConfigPath, defaultSamplerRatio))
		if !ok {
			return nil, newErrField(providerNewMessage, SamplerRatioConfigPath)
		}
		sampler = sdktrace.TraceIDRatioBased(ratio)
	default:
		return nil, newErrField(providerNewMessage, SamplerConfigPath)
	}

	parentBased, ok := config.Get(SamplerParentBasedConfigPath, defaultParentBased).(bool)
	if !ok {
		return nil, newErrField(providerNewMessage, SamplerParentBasedConfigPath)
	}
	if parentBased {
		sampler = sdktrace.ParentBased(sampler)
	}
	return sampler, nil
}

// providerRatio returns the ratio of the value, a float64 or an int, and
// whether it is one, from 0 to 1.
func providerRatio(
	value any,
) (float64, bool) {
	ratio := -1.0
	switch v := value.(type) {
	case float64:
		ratio = v
	case int:
		ratio = float64(v)
	}
	return ratio, ratio >= 0 && ratio <= 1
}

// providerResource returns the default resource of OpenTelemetry, merged
// with the attributes of the configuration, the service name and version
// taking precedence over the ones of the bag of ResourceConfigPath.
func providerResource(
	config flam.IConfig,
) (*sdkresource.Resource, error) {
	var attributes []attribute.KeyValue
	if config.Get(ResourceConfigPath) != nil {
		bag := config.Bag(ResourceConfigPath)
		if bag == nil {
			return nil, newErrField(providerNewMessage, ResourceConfigPath)
		}
		var ok bool
		if attributes, ok = providerAttributes(bag, ""); !ok {
			return nil, newErrField(providerNewMessage, ResourceConfigPath)
		}
	}
	for path, attr := range map[string]func(string) attribute.KeyValue{
		ServiceNameConfigPath:    semconv.ServiceName,
		ServiceVersionConfigPath: semconv.ServiceVersion,
	} {
		value, ok := config.Get(path, "").(string)
		if !ok {
			return nil, newErrField(providerNewMessage, path)
		}
		if value != "" {
			attributes = append(attributes, attr(value))
		}
	}

	// both resources are of the schema of the SDK, so that the merge,
	// failing only on a schema conflict, cannot fail
	resource, _ := sdkresource.Merge(
		sdkresource.Default(),
		sdkresource.NewWithAttributes(semconv.SchemaURL, attributes...),
	)
	return resource, nil
}

// providerAttributes returns the attributes of the values of the bag, of
// the keys of the bag prefixed with the prefix, flattening its nested
// bags, and whether every value is of a type an attribute can hold.
func providerAttributes(
	bag flam.IBag,
	prefix string,
) ([]attribute.KeyValue, bool) {
	var attributes []attribute.KeyValue
	for key, value := range bag.All() {
		name := prefix + key
		switch v := value.(type) {
		case string:
			attributes = append(attributes, attribute.String(name, v))
		case bool:
			attributes = append(attributes, attribute.Bool(name, v))
		case int:
			attributes = append(attributes, attribute.Int(name, v))
		case int64:
			attributes = append(attributes, attribute.Int64(name, v))
		case float64:
			attributes = append(attributes, attribute.Float64(name, v))
		default:
			nested := bag.Bag(key)
			if nested == nil {
				return nil, false
			}
			inner, ok := providerAttributes(nested, name+".")
			if !ok {
				return nil, false
			}
			attributes = append(attributes, inner...)
		}
	}
	return attributes, true
}
