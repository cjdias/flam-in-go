// Package compress is the REST middleware driver compressing the
// responses of the endpoints with the content coding the client prefers,
// by its Accept-Encoding header, among the ones of the middleware,
// [ConfigFieldEncodings]: [EncodingGzip] and [EncodingDeflate] by default,
// gzip being preferred on a tie. The qualities of the header are honored,
// an encoding of quality 0 being refused, and one the header does not
// list taking the quality of "*", when listed. A request without the
// header, or accepting none of the encodings, is served as it is.
//
// Every response holds Accept-Encoding in its Vary header, so that the
// caches tell the compressed responses from the others. The responses to
// the HEAD requests, the ones of status 204 or 304, the ones already
// holding a Content-Encoding header, or a Content-Range one, as a partial
// response cannot be compressed, and the ones whose media type is not one
// of [ConfigFieldContentTypes], "application/json",
// "application/problem+json" and "text/*" by default, are sent as they
// are. A response without Content-Type takes the one the server would
// sniff from the start of its body.
//
// The start of the body is buffered until it reaches [ConfigFieldMinSize]
// bytes, 1024 by default, the response being then compressed, while a
// response whose handler returns before is sent uncompressed, as
// compressing a small body saves too little. A compressed response loses
// its Content-Length and Accept-Ranges headers, which would describe the
// uncompressed body, and gains the Content-Encoding one. When the handler
// flushes the response, as when streaming it, the response buffered is
// sent at once, compressed whatever its size, and every flush flushes
// the compressor too, so that the client receives what was written. The
// compressors, of the [ConfigFieldLevel] level, are reused through a pool
// per encoding.
//
// The writer given to the handler is a
// [github.com/cjdias/flam-in-go/pkg/flam.IRestResponseWriter], recording
// the status and the size of the body, uncompressed, written by the
// handler. As the status and the headers are only sent once the decision
// to compress is taken, the headers the handler changes after writing the
// status are sent too, as long as the decision is pending. A handler
// panicking leaves the response unsent, when still buffered, for the
// middlewares listed before to answer.
//
// The middlewares listed before this one see the compressed body: list
// it before the log middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log], so that
// the body it logs is readable, and after the etag middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/etag], so that
// the compressed and uncompressed responses have different entity tags.
//
// The middleware passes the requests of a streaming endpoint, one whose
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint] Streaming field is
// set, such as one sending Server-Sent Events or taking its connection
// over for a WebSocket, through as they are, neither buffered nor compressed.
package compress
