// Package secure is the REST middleware driver setting the security
// headers of the responses of the endpoints, before their handlers run,
// so that a handler may still change or remove them.
//
// The defaults suit an API: X-Content-Type-Options "nosniff", so that the
// browsers never guess another content type, X-Frame-Options "DENY", so
// that no page frames the responses, Referrer-Policy "no-referrer", and
// Strict-Transport-Security of a year, with includeSubDomains, so that the
// browsers only call the host over TLS from then on. The
// Content-Security-Policy, Permissions-Policy, Cross-Origin-Opener-Policy,
// Cross-Origin-Embedder-Policy and Cross-Origin-Resource-Policy headers
// are left out unless configured. Each header is set from its field, an
// empty value leaving it out.
//
// The Strict-Transport-Security header is only set on the responses to the
// requests received over TLS, as the browsers ignore it otherwise, unless
// [ConfigFieldForceHSTS] is set, such as behind a proxy ending the TLS
// connections, whose requests reach the server without TLS. A
// [ConfigFieldHSTSMaxAge] of 0 leaves it out, while one with the preload
// directive, [ConfigFieldHSTSPreload], should be of a year or more, with
// the includeSubDomains directive, to be accepted by the preload lists.
//
// List the middleware early among the middlewares of a server, so that the
// answers of the middlewares listed after it, such as the envelope
// middleware answering the panics, hold its headers too.
package secure
