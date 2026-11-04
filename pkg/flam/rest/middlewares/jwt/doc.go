// Package jwt is the REST middleware driver authenticating the requests
// by the JSON Web Token of their "Authorization: Bearer <token>" header,
// verified with [github.com/golang-jwt/jwt/v5].
//
// A token is accepted only when signed with one of the algorithms of the
// middleware, [ConfigFieldAlgorithms], with its key: the HMAC algorithms
// (HS256, HS384, HS512) verify with the shared [ConfigFieldSecret], at
// least as long as the hash of the strongest of them, and the public key
// ones (RS*, PS*, ES*, EdDSA) with the PEM public key of
// [ConfigFieldPublicKey] or of the file of [ConfigFieldPublicKeyFile], an
// RSA key of 2048 bits or more, an ECDSA key of the curve of each ES
// algorithm, or an Ed25519 key. Exactly one of the three fields is set,
// and every algorithm must fit its key, which the creator checks: an
// unsigned token ("none") is never accepted, nor a token signed with an
// algorithm of the other family, closing the algorithm confusion attacks.
//
// The token must also be valid in time, with the leeway of
// [ConfigFieldLeeway], measured with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, hold an
// expiration unless [ConfigFieldRequireExpiration] is false, and the issuer
// and the audience of [ConfigFieldIssuer] and [ConfigFieldAudience] when
// set. Its claims are then given to the handler in the context of the
// request, read by [ClaimsOf], along with its subject as the principal,
// read by [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf], for
// the other middlewares, such as a rate limiter.
//
// A request without a bearer token is answered with a 401 envelope of
// [ErrMissingToken], unless the middleware is optional, by
// [ConfigFieldOptional], in which case it is served without claims. A
// request with a token failing verification is answered with a 401
// envelope of [ErrInvalidToken], even for an optional middleware, the
// cause of the failure never reaching the client. Both answers hold a
// WWW-Authenticate header of the Bearer scheme.
package jwt
