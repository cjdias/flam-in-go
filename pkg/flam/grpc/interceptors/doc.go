// Package interceptors provides the bundle registering every gRPC
// interceptor driver bundle, to be registered along with the
// [github.com/cjdias/flam-in-go/pkg/flam/grpc] bundle, while its sub
// packages hold the drivers.
//
// The interceptors of a server are applied in the order it lists them, the
// first one being the outermost. A sensible order of the drivers of the
// tracing, metrics, log and recovery sub packages is that one: tracing
// first, so that its span covers the others, and the log entries may be
// told by the trace of their context; recovery last, so that the others
// see the Internal status answering a panic rather than the panic. The
// tracing and metrics drivers need the bundles of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing] and
// [github.com/cjdias/flam-in-go/pkg/flam/metrics] to create their
// interceptors.
package interceptors
