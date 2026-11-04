// Package apikey is the REST middleware driver authenticating the
// requests by the API key of their header, [ConfigFieldHeader],
// "X-Api-Key" by default, or of their query parameter,
// [ConfigFieldQuery], unset by default, the header being read first.
//
// The keys accepted, [ConfigFieldKeys], are either a list, or a map of the
// name of each principal to its key, or to the list of its keys, so that a
// key can be rotated: the principal of the key of a request is then given
// to the other middlewares and to the handler in its context, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf], such as for a
// rate limiter to limit each client. The names are the map keys, rather
// than the API keys themselves, as the configuration paths are split on
// their dots.
//
// The keys are held as their SHA-256 hashes, the hash of the key of a
// request being compared with all of them in constant time, so that
// neither the time taken nor the memory tell how much of a key matched,
// or which one did.
//
// A request without a key is answered with a 401 envelope of
// [ErrMissingKey], unless the middleware is optional, by
// [ConfigFieldOptional], in which case it is served without principal. A
// request with a key matching none is answered with a 401 envelope of
// [ErrInvalidKey], even for an optional middleware. Both answers hold the
// WWW-Authenticate header [Challenge].
//
// A key sent in the query is likely logged, such as by the access logs of
// the proxies: prefer the header, and redact the query parameter from the
// logs of the application.
package apikey
