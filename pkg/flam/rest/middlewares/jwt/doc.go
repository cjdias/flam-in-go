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
// algorithm, or an Ed25519 key. The public keys may also be those of a
// JSON Web Key Set, fetched from the https URL of [ConfigFieldJWKSURL], or
// from the one the OpenID Connect discovery document of the issuer of
// [ConfigFieldIssuerURL] gives, which must name the issuer URL as its
// issuer, the issuer URL being then the default of [ConfigFieldIssuer]. Exactly one of the five fields is set, and every
// algorithm must fit its key, which the creator checks: an unsigned token
// ("none") is never accepted, nor a token signed with an algorithm of the
// other family, closing the algorithm confusion attacks.
//
// The key set is fetched when a token first needs it, never at creation,
// with a plain HTTP client, or with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClient] of
// [ConfigFieldJWKSClient], each fetch bounded by [ConfigFieldJWKSTimeout]
// and its answer by [JWKSMaxSize]. Its RSA, ECDSA P-256, P-384 and P-521,
// and Ed25519 keys are kept, unless their "use" member is other than
// "sig", or their id is shared, and each verifies only the tokens naming
// it by their "kid" header, of an algorithm fitting it, the one of its
// "alg" member when set: only the public key algorithms are accepted with
// a key set. The set is fetched again once [ConfigFieldJWKSRefresh]
// elapsed, measured with the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, or when
// a token names a key it does not hold, such as after a key rotation, at
// most once per [ConfigFieldJWKSMinRefresh] however many tokens name
// unknown keys. A failed fetch keeps the keys of the last successful one.
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
