// Package middlewares provides the bundle registering every REST
// middleware driver bundle, to be registered along with the
// [github.com/cjdias/flam-in-go/pkg/flam/rest] bundle, while its sub
// packages hold the drivers.
//
// The middlewares of a server are applied in the order it lists them, the
// first one being the outermost. A sensible order of the drivers of the
// requestid, cors, log, envelope, bodylimit and timeout sub packages is
// that one: requestid first, so that every other one, and the log entries,
// see the id of the request; cors before the middlewares answering the
// requests themselves, so that their answers hold the CORS headers and the
// preflight requests are answered before them; log before envelope, so
// that it logs the answers to the panics; and timeout last, so that it
// bounds the handler only.
//
// The authentication drivers of the jwt and apikey sub packages set the
// principal of the requests, by which the ratelimit one tells the
// clients, falling back to their IP: list them before ratelimit, so that
// it limits each client rather than each address. The authz driver
// authorizes the requests by their claims and principal: list it after
// them too.
//
// The secure driver sets the security headers of the responses: list it
// early, so that the answers of the middlewares after it hold them too.
// The etag and compress drivers buffer the responses: list etag before
// compress, so that it tags the compressed bodies, as its strong tags
// require, its default weak ones fitting either order, and compress before
// log, so that the bodies logged are readable. The idempotency driver
// replays the responses to the repeated requests: list it after the
// authentication drivers, as it scopes the keys by principal, and after
// bodylimit, as it reads the bodies whole.
package middlewares
