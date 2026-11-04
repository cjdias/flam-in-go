// Package log is the gRPC interceptor driver logging the calls served by
// a server to the [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the
// container, on the channel of the interceptor, with one entry per call,
// unary or streaming, signaled once it is served, even when its handler
// panicked, whose panic then goes on.
//
// The entry holds the full method of the call at [ContextFieldMethod], the
// name of the code of its status at [ContextFieldCode], such as "OK" or
// "NotFound", the duration measured with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container at
// [ContextFieldDuration], the address of the client at [ContextFieldPeer],
// and its error at [ContextFieldError]. It is signaled at the error level
// for a server error, as
// [github.com/cjdias/flam-in-go/pkg/flam/grpc.IsServerError] tells, and
// for a handler that panicked, whose entry holds [CodePanic] and
// [ContextFieldPanicked], and at the level of the interceptor otherwise,
// such as for a NotFound status, which tells a failure of the call rather
// than of the server.
package log
