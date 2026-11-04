package none

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.ITracingExporterCreator] of the none
// exporters. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator() flam.ITracingExporterCreator {
	return &creator{}
}

// creator is the [flam.ITracingExporterCreator] of the configurations with
// the [Driver] driver.
type creator struct{}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the exporter discarding the spans, which never fails.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ITracingExporter, error) {
	return &exporter{id: bag.String(factory.ConfigFieldId)}, nil
}

// exporter is the none [flam.ITracingExporter], discarding the spans.
type exporter struct {
	id string
}

// compile time check that *exporter implements ITracingExporter
var _ flam.ITracingExporter = (*exporter)(nil)

func (e *exporter) Id() string {
	return e.id
}

// ExportSpans discards the spans.
func (e *exporter) ExportSpans(
	context.Context,
	[]sdktrace.ReadOnlySpan,
) error {
	return nil
}

// Shutdown does nothing, as the exporter holds nothing.
func (e *exporter) Shutdown(
	context.Context,
) error {
	return nil
}
