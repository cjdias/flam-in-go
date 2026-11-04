package flam

import (
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// ITracingExporter is an OpenTelemetry span exporter, held by an
// [ITracingExporterFactory] under its id, to which the tracer provider of
// the tracing feature sends the spans it ends.
//
// The tracer provider owns the exporters it sends its spans to, and shuts
// them down when it is shut down: the factory does not, as it only closes
// the resources implementing [io.Closer], which an exporter needs not.
type ITracingExporter interface {
	sdktrace.SpanExporter
	IResource
}

// ITracingExporterCreator creates the tracing exporters of a driver, for
// the [ITracingExporterFactory].
type ITracingExporterCreator = IResourceCreator[ITracingExporter]

// ITracingExporterFactory creates and holds the tracing exporters, one per
// id.
type ITracingExporterFactory = IFactory[ITracingExporter]
