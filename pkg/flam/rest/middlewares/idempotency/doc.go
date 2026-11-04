// Package idempotency is the REST middleware driver making the requests
// of the unsafe methods, [ConfigFieldMethods], "POST" and "PATCH" by
// default, idempotent by their idempotency key, held by the header of the
// middleware, [ConfigFieldHeader], "Idempotency-Key" by default, so that
// a client can retry a request it got no answer to without its effect
// being applied twice.
//
// The first complete response to a request holding a key is stored in
// the cache adaptor of the middleware, [ConfigFieldCache], an id in the
// [github.com/cjdias/flam-in-go/pkg/flam.ICacheAdaptorFactory], for
// [ConfigFieldTTL], a day by default: its status, its headers listed by
// [ConfigFieldHeaders], "Content-Type" and "Location" by default, and its
// body, along with the SHA-256 hash of the body of the request. A request
// repeating the key is answered with the response stored, along with the
// [ReplayedHeader] header set to "true", without calling the handler,
// unless its body is another one, in which case it is answered with a
// 422 envelope of [ErrKeyReused]. The responses of a 5xx status, the ones
// whose body is longer than [ConfigFieldMaxBodySize], a MiB by default,
// and the ones whose handler panics are not stored, so that the request
// can be made again.
//
// The key is scoped by the principal of the request, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf], its method
// and its path, so that two clients, or two endpoints, never share a
// response: list the middleware after the authentication middlewares. The
// records are stored under the SHA-256 hash of the id of the middleware
// and of those, in their Id field, as the default key generator of
// [github.com/cjdias/flam-in-go/pkg/flam/cache/key_generators/field]
// expects.
//
// A request without key is served as it is, unless the key is required,
// by [ConfigFieldRequired], in which case it is answered with a 400
// envelope of [ErrMissingKey]. A key longer than [MaxKeyLength] bytes is
// answered with a 400 envelope of [ErrInvalidKey]. The body of the
// request is read whole, to be hashed, then given back to the handler:
// list the body limit middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/bodylimit]
// before this one, a body longer than its limit being answered with a 413
// envelope of [ErrReadBody]. A failure of the cache adaptor is answered
// with a 503 envelope of [ErrCache], as the request cannot then be made
// idempotent.
//
// While the first request of a key is served, a repeated one is answered
// with a 409 envelope of [ErrInFlight]. Within the process, the keys in
// flight are held by the middleware, so that two requests of a key never
// run at once. Across the processes sharing the cache, a marker is stored
// under the key for [ConfigFieldLockTTL], a minute by default, until the
// response is stored: as the cache adaptors offer no atomic
// set-if-missing, two processes receiving a key at the very same time may
// both serve it, and a handler running longer than the lock ttl lets a
// repeated request be served again. Use the middleware for the retries of
// the clients, rather than as a lock.
//
// The response stored is the one written by the handler and the
// middlewares listed after this one, while the middlewares listed before
// it, such as the log middleware, see the replayed responses as any
// other.
//
// The middleware passes the requests of a streaming endpoint, one whose
// [github.com/cjdias/flam-in-go/pkg/flam.RestEndpoint] Streaming field is
// set, such as one sending Server-Sent Events or taking its connection
// over for a WebSocket, through as they are, their responses being neither stored nor replayed.
package idempotency
