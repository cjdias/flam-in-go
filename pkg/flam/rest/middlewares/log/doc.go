// Package log is the REST middleware driver logging the requests served by
// the endpoints to the
// [github.com/cjdias/flam-in-go/pkg/flam.ILogger] of the container, on
// the channel of the middleware, with two entries per request.
//
// The first entry is signaled when the request is received, at the
// request level, with the bag describing the request at
// [ContextFieldRequest]: its method, path, endpoint pattern, id and
// service id, remote address, headers and query parameters, its id, when
// its context holds one, and its body, up to the maximum body size. Reading the body leaves it whole for the
// handler. The body of a request expecting a "100-continue" is not read,
// as reading it would have the client send it at once, before the
// handler may reject it.
//
// The second entry is signaled once the request is served, even when the
// handler panicked, whose panic then goes on, at the error level for a
// response of a 5xx status or a handler that panicked, and at the
// response level otherwise. It holds the request bag again, the bag
// describing the response at [ContextFieldResponse], its status, size and
// headers, and its body, up to the maximum body size, when the status is
// not the one of the endpoint, the body being kept only then, the duration measured with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container at
// [ContextFieldDuration], and [ContextFieldPanicked] for a handler that
// panicked.
//
// The values of the headers listed by [ConfigFieldRedactHeaders], and of
// the query parameters listed by [ConfigFieldRedactParams], are logged as
// [RedactedValue], the bodies are left out with a maximum body size of 0,
// and the JSON bodies are also logged decoded with [ConfigFieldParseJSON],
// as the bodies, headers and query parameters may hold private data.
//
// The id of the request is the one stored in its context by the request
// id middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/requestid], read
// by [github.com/cjdias/flam-in-go/pkg/flam/rest.RequestIdOf], at
// [ContextFieldRequestId]: list the request id middleware before the log
// middleware, as the middlewares are applied in the order the server
// lists them, the first one being the outermost, so that the requests
// reach the log middleware holding their id.
package log
