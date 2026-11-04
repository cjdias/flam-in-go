// Package etag is the REST middleware driver tagging the responses of the
// endpoints with an entity tag, the ETag header, and answering the
// conditional requests whose tag matches with a 304, so that the clients
// and the caches holding the response need not fetch it again.
//
// Only the 200 responses to the GET and HEAD requests are tagged: their
// body is buffered, up to [ConfigFieldMaxSize] bytes, a MiB by default,
// until the handler returns, then tagged with the first [TagLength] bytes
// of its SHA-256 hash, as quoted hexadecimal digits, prefixed by "W/" as
// a weak tag unless [ConfigFieldStrong] is true, unless the handler set
// the ETag header itself. A longer response, or one the handler
// flushes, as when streaming it, is sent as it is, untagged, as is the
// response to a HEAD request whose handler wrote no body. The responses of
// another status, and the responses to the requests of another method,
// are sent as they are, unbuffered. A 200 response whose handler set the
// ETag header before writing its status is not buffered either.
//
// When the If-None-Match header of the request lists the tag, or "*", by
// the weak comparison RFC 9110 sets for that header, the request is
// answered with a 304 without body, the headers describing the body,
// Content-Type, Content-Length, Content-Encoding and Last-Modified,
// removed, as [net/http.ServeContent] does.
//
// The writer given to the handler is a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], recording
// the status and the size of the body written by the handler, while the
// middlewares listed before this one see the response as sent, such as
// the 304. A handler panicking leaves the response unsent, when still
// buffered, for the middlewares listed before to answer.
//
// A strong tag must tell apart the responses whose bodies differ byte for
// byte, as RFC 9110 sets: the compressed response from the uncompressed
// one. The middleware tags the body it sees, so that, listed after the
// compress middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/compress], it
// would give the same tag to the compressed and uncompressed responses,
// which only a weak tag may share: its tags are weak by default, fitting
// either order, the weak comparison of the If-None-Match header matching
// them as it does the strong ones. Set [ConfigFieldStrong] only when the
// middleware is listed before the compress middleware, so that it tags
// the compressed body, the compress middleware adding Accept-Encoding to
// the Vary header of the responses, so that the caches keep their
// representations apart.
//
// The middleware passes the requests of a streaming endpoint, one whose
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint] Streaming field is
// set, such as one sending Server-Sent Events or taking its connection
// over for a WebSocket, through as they are, neither buffered nor tagged.
package etag
