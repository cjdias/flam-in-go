// Package envelope is the REST middleware driver answering the panics of
// the handlers of the endpoints with envelopes, written by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope], so that a
// handler can panic with the envelope, or the envelope error, of its
// response as a control flow:
//
//   - a [github.com/cjdias/flam-in-go/pkg/flam/envelope.Envelope] is
//     answered as it is;
//   - a [github.com/cjdias/flam-in-go/pkg/flam/envelope.Error] is answered
//     with an envelope holding it;
//   - any other error is answered with a 500 envelope holding its message
//     when the middleware exposes the errors, or [ErrInternal] otherwise,
//     the default, so that the details of the failure never reach the
//     client unless intended;
//   - any other value is answered with a 500 envelope holding
//     [ErrInternal].
//
// The panics of the last two kinds, unexpected, are reported to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container, when
// provided, as an entry signaled on the channel of the middleware, at its
// level, with the endpoint id and pattern, the value of the panic and the
// stack in its context, at [ContextFieldEndpoint], [ContextFieldPattern],
// [ContextFieldPanic] and [ContextFieldStack].
//
// A panic with [net/http.ErrAbortHandler], or wrapping it, is panicked
// again, unreported, to abort the response as intended, and so is any
// panic raised once the response was written, after being reported, as a
// response cannot be written in place of a started one: the server then
// aborts the connection.
package envelope
