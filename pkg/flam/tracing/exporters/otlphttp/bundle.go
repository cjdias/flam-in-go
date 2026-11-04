package otlphttp

import (
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/bundle"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/tracing"
)

// NewBundle returns the [flam.IBundle] of the otlphttp exporter driver,
// providing its [flam.ITracingExporterCreator] in the
// [tracing.ExporterCreatorGroup] group. It is also a
// [flam.IConfigurableBundle], configuring by default an exporter of id
// [Id] with this driver, and setting the defaults of the exporter fields:
// list [Id] at [tracing.ExportConfigPath] to send the spans to it.
func NewBundle() flam.IBundle {
	return &bundleImpl{}
}

// bundleImpl is the [flam.IBundle] returned by [NewBundle].
type bundleImpl struct{}

// compile time check that *bundleImpl implements IConfigurableBundle
var _ flam.IConfigurableBundle = (*bundleImpl)(nil)

func (b *bundleImpl) Id() string {
	return bundleId
}

func (b *bundleImpl) Config(
	bag flam.IBag,
) error {
	for path, value := range map[string]any{
		tracing.ExporterConfigPath + "." + Id: container.Bag{factory.ConfigFieldDriver: Driver},
		DefaultEndpointConfigPath:             defaultEndpoint,
		DefaultPathConfigPath:                 defaultPath,
		DefaultInsecureConfigPath:             defaultInsecure,
		DefaultTimeoutConfigPath:              defaultTimeout.String(),
		DefaultCompressionConfigPath:          defaultCompression,
	} {
		if err := bag.Set(path, value); err != nil {
			return err
		}
	}
	return nil
}

func (b *bundleImpl) Register(
	app flam.IApp,
) error {
	return bundle.NewRegisterer(app).
		QueueProvider(newCreator, dig.Group(tracing.ExporterCreatorGroup)).
		Register()
}
