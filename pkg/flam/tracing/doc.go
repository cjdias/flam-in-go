// Package tracing is the tracing feature, over OpenTelemetry: the bundle
// returned from [NewBundle] provides the
// [go.opentelemetry.io/otel/trace.TracerProvider] of an application,
// whose spans are sent to the exporters held by the
// [github.com/cjdias/flam-in-go/pkg/flam.ITracingExporterFactory],
// created from the configuration at [ExporterConfigPath] with the
// creators provided in the [ExporterCreatorGroup] container group, and
// the [go.opentelemetry.io/otel/propagation.TextMapPropagator] of the W3C
// trace context and baggage.
//
// The tracer provider is configured under "flam.tracing", such as:
//
//	flam:
//	  tracing:
//	    service: {name: api, version: 1.2.0}
//	    resource:
//	      deployment.environment.name: production
//	    sampler: {type: ratio, ratio: 0.25}
//	    exporters:
//	      collector:
//	        driver: flam.tracing.exporters.driver.otlphttp
//	        endpoint: collector:4318
//	        insecure: true
//	    export: [collector]
//
// The sampler, [SamplerAlways] unless set, decides whether a trace is
// sampled, recorded and exported, when it starts: a span having a parent
// is sampled when its parent is, unless [SamplerParentBasedConfigPath]
// holds false, so that the traces of the requests received from another
// service follow the decision of that service. Setting
// [EnabledConfigPath] to false disables the tracing: the provider is then
// the no-op one, whose spans cost close to nothing.
//
// Booting the application sets the provider and the propagator as the
// global ones of OpenTelemetry, for the instrumentations reading them,
// unless [GlobalConfigPath] holds false, and closing it shuts the
// provider down, exporting the spans left.
//
// The application, its libraries and the instrumentations of flam start
// their spans with a tracer of the provider:
//
//	err := app.Invoke(func(provider trace.TracerProvider) {
//		tracer := provider.Tracer("example.com/orders")
//		ctx, span := tracer.Start(ctx, "checkout")
//		defer span.End()
//		...
//	})
//
// The instrumentations of flam are bundles of their own, each requiring
// this one: the REST middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware], tracing the
// requests served by the REST servers, the gorm plugin of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/database], tracing the
// statements of the database connections, and the go-redis hook of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/redis], tracing the
// commands of the redis connections.
package tracing
