// Package timeout is the REST middleware driver bounding the time taken by
// the handlers of the endpoints, as [net/http.TimeoutHandler] does, but
// answering with an envelope.
//
// The handler runs in its own goroutine, with a request whose context
// expires once the timeout of the middleware, [ConfigFieldTimeout], has
// passed, writing its response in a buffer. Once the handler returns, the
// response buffered is written as it is. When the context is done before,
// as the timeout passed, or the client went away, the response buffered
// is dropped, and the request is answered at once with a 503 envelope,
// written by [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope],
// holding [ErrTimeout], while the writes of the handler, still running,
// fail from then on with [net/http.ErrHandlerTimeout]: the handler should
// stop once the context of its request is done.
//
// The writer given to the handler is a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], so that
// the middlewares listed after this one record the status and the size of
// the response. As every response is buffered whole, the streaming
// responses are only sent once the handler returns, their flushes doing
// nothing, and the informational statuses, such as 103, are dropped.
//
// The middleware passes the requests of a streaming endpoint, one whose
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint] Streaming field is
// set, such as one sending Server-Sent Events with
// [github.com/cjdias/flam-in-go/pkg/flam/rest.NewEventStream], or taking
// its connection over for a WebSocket, through as they are: their
// handlers are neither bounded by the timeout nor buffered, and should
// end once the context of their request is done.
//
// A panic of the handler before the context is done goes on in the
// goroutine serving the request, so that the middlewares listed before
// this one, such as the envelope middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/envelope], can
// answer it, while a later one is dropped, as the request is answered.
package timeout
