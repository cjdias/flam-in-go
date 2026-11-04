package otlphttp

import (
	"context"

	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.ITracingExporterCreator] of the otlphttp
// exporters, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ITracingExporterCreator {
	return &creator{config: args.Config}
}

// creator is the [flam.ITracingExporterCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the started exporter of the endpoint, the path, the
// security, the headers, the timeout and the compression of the
// configuration, or of their defaults. Starting it connects to nothing:
// the collector is only reached by the exports.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ITracingExporter, error) {
	id := bag.String(factory.ConfigFieldId)

	texts := map[string]string{}
	for field, def := range map[string]struct {
		path  string
		value string
	}{
		ConfigFieldEndpoint: {DefaultEndpointConfigPath, defaultEndpoint},
		ConfigFieldPath:     {DefaultPathConfigPath, defaultPath},
	} {
		value, ok := c.field(bag, field, def.path, def.value).(string)
		if !ok {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
		}
		texts[field] = value
	}
	insecure, ok := c.field(bag, ConfigFieldInsecure, DefaultInsecureConfigPath, defaultInsecure).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldInsecure)
	}
	headers, ok := creatorHeaders(c.field(bag, ConfigFieldHeaders, DefaultHeadersConfigPath, nil))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldHeaders)
	}
	timeout, ok := settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok || timeout == 0 {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldTimeout)
	}
	name, _ := c.field(bag, ConfigFieldCompression, DefaultCompressionConfigPath, defaultCompression).(string)
	compression, ok := creatorCompressions[name]
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldCompression)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithTimeout(timeout),
		otlptracehttp.WithCompression(compression),
	}
	if endpoint := texts[ConfigFieldEndpoint]; endpoint != "" {
		opts = append(opts, otlptracehttp.WithEndpoint(endpoint))
	}
	if path := texts[ConfigFieldPath]; path != "" {
		opts = append(opts, otlptracehttp.WithURLPath(path))
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(headers))
	}
	exp, err := otlptracehttp.New(context.Background(), opts...)
	if err != nil {
		return nil, newErrJoined(ErrExporter, creatorCreateMessage, id, err)
	}
	return &exporter{SpanExporter: exp, id: id}, nil
}

// field returns the value of the field of the exporter configuration, or
// else the one stored in the configuration at the default path, or else
// the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorHeaders returns the headers of the value, a bag of strings, nil
// being no header, and whether the value is one.
func creatorHeaders(
	value any,
) (map[string]string, bool) {
	var bag container.Bag
	switch v := value.(type) {
	case nil:
		return nil, true
	case container.Bag:
		bag = v
	case map[string]any:
		bag = v
	default:
		return nil, false
	}
	headers := make(map[string]string, len(bag))
	for name, value := range bag {
		s, ok := value.(string)
		if !ok {
			return nil, false
		}
		headers[name] = s
	}
	return headers, true
}

// creatorCompressions are the compressions of the exporters, by their
// name held by ConfigFieldCompression.
var creatorCompressions = map[string]otlptracehttp.Compression{
	CompressionNone: otlptracehttp.NoCompression,
	CompressionGzip: otlptracehttp.GzipCompression,
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create otlphttp tracing exporter"

// exporter is the otlphttp [flam.ITracingExporter], exporting the spans
// with the OpenTelemetry exporter it embeds.
type exporter struct {
	sdktrace.SpanExporter

	id string
}

// compile time check that *exporter implements ITracingExporter
var _ flam.ITracingExporter = (*exporter)(nil)

func (e *exporter) Id() string {
	return e.id
}
