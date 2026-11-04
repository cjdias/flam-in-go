package otlp

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.ILogStreamCreator] of the otlp streams,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILogStreamCreator {
	return &creator{
		config: args.Config,
	}
}

// creator is the [flam.ILogStreamCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns a stream exporting its entries to the collector, and
// exporting those left, then shutting its exporter down, when closed. The
// collector is only reached when the first batch of records is exported.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ILogStream, error) {
	id := bag.String(factory.ConfigFieldId)

	level, ok := base.Level(base.Field(c.config, bag, ConfigFieldLevel, DefaultLevelConfigPath, defaultLevel))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldLevel)
	}
	channels, ok := base.Channels(bag.Get(ConfigFieldChannels))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldChannels)
	}
	endpoint, ok := base.NonEmpty(base.Field(c.config, bag, ConfigFieldEndpoint, DefaultEndpointConfigPath, defaultEndpoint))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldEndpoint)
	}
	insecure, ok := base.Field(c.config, bag, ConfigFieldInsecure, DefaultInsecureConfigPath, false).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldInsecure)
	}
	headers, ok := creatorHeaders(base.Field(c.config, bag, ConfigFieldHeaders, DefaultHeadersConfigPath, nil))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldHeaders)
	}
	timeout, ok := settings.Duration(base.Field(c.config, bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok || timeout == 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	serviceName, ok := base.Field(c.config, bag, ConfigFieldServiceName, DefaultServiceNameConfigPath, "").(string)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldServiceName)
	}

	options := []otlploghttp.Option{
		otlploghttp.WithEndpoint(endpoint),
		otlploghttp.WithTimeout(timeout),
		otlploghttp.WithHeaders(headers),
	}
	if insecure {
		options = append(options, otlploghttp.WithInsecure())
	}
	exporter, err := otlploghttp.New(context.Background(), options...)
	if err != nil {
		return nil, newErrExporter(creatorCreateMessage, id, endpoint, err)
	}
	providerOptions := []sdklog.LoggerProviderOption{sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter))}
	if serviceName != "" {
		// merging a schemaless resource never fails
		res, _ := resource.Merge(resource.Default(),
			resource.NewSchemaless(attribute.String(creatorServiceNameKey, serviceName)))
		providerOptions = append(providerOptions, sdklog.WithResource(res))
	}
	provider := sdklog.NewLoggerProvider(providerOptions...)

	w := &writer{provider: provider, logger: provider.Logger(ScopeName), timeout: timeout}
	return base.New(id, level, channels, recorder{w: w}, w, w), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create otlp log stream"

// creatorServiceNameKey is the resource attribute of the service name, as
// the OpenTelemetry semantic conventions name it.
const creatorServiceNameKey = "service.name"

// creatorHeaders returns the headers held by the value, a bag or a map of
// string values, or nil, which holds none, or false for any other value.
func creatorHeaders(
	val any,
) (map[string]string, bool) {
	headers := map[string]string{}
	switch v := val.(type) {
	case nil:
		return headers, true
	case map[string]string:
		return v, true
	case flam.IBag:
		for key, item := range v.All() {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			headers[key] = s
		}
		return headers, true
	case map[string]any:
		for key, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			headers[key] = s
		}
		return headers, true
	}
	return nil, false
}
