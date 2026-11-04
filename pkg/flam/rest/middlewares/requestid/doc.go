// Package requestid is the REST middleware driver giving an id to each
// request served by the endpoints, so that its log entries, its response
// and the logs of the client can be correlated.
//
// The id is read from the header of the middleware, [ConfigFieldHeader],
// "X-Request-Id" by default, as sent by the client or by a proxy. When the
// header is missing, or its value is invalid, longer than [MaxLength]
// bytes, or holding a byte other than an ASCII letter, a digit or one of
// "-_.:+/=", so that a client cannot inject anything in the logs through
// it, a new id is generated: 128 random bits from [crypto/rand], as 32
// lower case hexadecimal digits.
//
// The id is stored in the context of the request given to the inner
// middlewares and the handler, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.RequestIdOf], and set on
// the header of the response, before the handler runs, so that the
// handler may still change it.
//
// The middlewares are applied in the order the server lists them, the
// first one being the outermost: list the request id middleware before
// the middlewares reading the id, such as the log middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log], which
// logs it in its request entry, as the middlewares listed before it
// receive the request before it holds the id.
package requestid
