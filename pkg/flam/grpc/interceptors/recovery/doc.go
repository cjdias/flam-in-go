// Package recovery is the gRPC interceptor driver recovering the panics of
// the handlers of the calls, which would otherwise end the process: the
// call is answered with an Internal status of the generic [Message],
// telling the client nothing of the panic, while the panic is signaled to
// the [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container, on
// the channel of the interceptor, with the method of the call, the value
// of the panic and the stack of the handler.
//
// The interceptors of a server are applied in the order it lists them, the
// first one being the outermost: list the recovery interceptor last, so
// that the others see the Internal status of the panics, or first, so that
// they see the panics themselves, as the log, tracing and metrics
// interceptors of the sibling packages record a panic going through them.
package recovery
