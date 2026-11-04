// Package exporters registers every tracing exporter driver of flam with
// the bundle returned from [NewBundle]: the
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters/otlphttp]
// driver, sending the spans to an OpenTelemetry collector over HTTP, and
// the [github.com/cjdias/flam-in-go/pkg/flam/tracing/exporters/none]
// driver, discarding them. Each driver also configures an exporter of its
// own name, to list at
// [github.com/cjdias/flam-in-go/pkg/flam/tracing.ExportConfigPath].
package exporters
