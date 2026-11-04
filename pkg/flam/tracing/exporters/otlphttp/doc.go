// Package otlphttp is the tracing exporter driver sending the spans to an
// OpenTelemetry collector, or to any backend receiving the OTLP protocol,
// over HTTP, with
// [go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp].
//
// An exporter is configured by its endpoint, the host and the port of the
// collector, the path of its URL, whether it is reached over plain HTTP,
// the headers sent along, such as for an authentication, the timeout of an
// export and the compression of the spans, such as:
//
//	flam:
//	  tracing:
//	    exporters:
//	      collector:
//	        driver: flam.tracing.exporters.driver.otlphttp
//	        endpoint: collector:4318
//	        insecure: true
//	        headers: {Authorization: Bearer secret}
//	        compression: gzip
//	    export: [collector]
//
// The fields left out take their default, read from the configuration,
// and the endpoint and the headers left empty keep the ones of the
// environment variables of OpenTelemetry, such as
// OTEL_EXPORTER_OTLP_ENDPOINT. An export failing, such as when the
// collector cannot be reached, is retried with a backoff within the
// timeout, then reported to the error handler of OpenTelemetry, and its
// spans are dropped: it never fails the application.
package otlphttp
