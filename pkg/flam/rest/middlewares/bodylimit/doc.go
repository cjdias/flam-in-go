// Package bodylimit is the REST middleware driver limiting the size of the
// bodies of the requests served by the endpoints, so that a client cannot
// exhaust the memory of the server by sending a huge body.
//
// A request announcing, by its Content-Length header, a body longer than
// the maximum of the middleware, [ConfigFieldMaxBytes], is answered at
// once, without calling the handler, with a 413 envelope, written by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.WriteEnvelope], holding
// [ErrTooLarge]. The body of any other request is wrapped by
// [net/http.MaxBytesReader], failing the reads past the maximum, such as
// of a chunked body, with a [*net/http.MaxBytesError], held by the error
// [github.com/cjdias/flam-in-go/pkg/flam/rest.DecodeJSON] returns.
//
// List the middleware after the log middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log], so that
// its 413 answers are logged, the log middleware reading no more than its
// own maximum body size, and before the middlewares and the handlers
// reading the bodies whole.
package bodylimit
