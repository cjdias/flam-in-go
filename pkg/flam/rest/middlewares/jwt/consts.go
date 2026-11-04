package jwt

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.jwt"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "jwt"

	// Driver is the driver name of the jwt middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.jwt"
)

// Fields of a jwt middleware configuration.
const (
	// ConfigFieldAlgorithms is the field holding the non-empty list of the
	// names of the signing algorithms accepted, such as "HS256" or
	// "RS256", a []string or a []any of strings, all fitting the key.
	ConfigFieldAlgorithms = "algorithms"

	// ConfigFieldSecret is the field holding the shared secret of the HMAC
	// algorithms, a string at least as long as the hash of the strongest
	// of them: 32, 48 or 64 bytes for HS256, HS384 or HS512.
	ConfigFieldSecret = "secret"

	// ConfigFieldPublicKey is the field holding the PEM public key of the
	// RSA, ECDSA or Ed25519 algorithms, a string holding a "PUBLIC KEY",
	// an "RSA PUBLIC KEY" or a "CERTIFICATE" block.
	ConfigFieldPublicKey = "public_key"

	// ConfigFieldPublicKeyFile is the field holding the path of the file
	// of the PEM public key, read as [ConfigFieldPublicKey] when the
	// middleware is created.
	ConfigFieldPublicKeyFile = "public_key_file"

	// ConfigFieldJWKSURL is the field holding the https URL of the JSON
	// Web Key Set of the RSA, ECDSA and Ed25519 public keys, fetched when
	// first needed, and again every [ConfigFieldJWKSRefresh] or when a
	// token names a key it does not hold.
	ConfigFieldJWKSURL = "jwks_url"

	// ConfigFieldIssuerURL is the field holding the https URL of the
	// OpenID Connect issuer, whose discovery document, at
	// "{issuer_url}/.well-known/openid-configuration", gives the URL of
	// the key set, read as [ConfigFieldJWKSURL]. It is also the default of
	// [ConfigFieldIssuer].
	ConfigFieldIssuerURL = "issuer_url"

	// ConfigFieldJWKSRefresh is the field holding the time the fetched key
	// set is used before being fetched again, a time.Duration or a string
	// such as "1h", over 0.
	ConfigFieldJWKSRefresh = "jwks_refresh"

	// ConfigFieldJWKSMinRefresh is the field holding the minimum time
	// between two fetches of the key set, bounding the fetches the tokens
	// naming unknown keys cause, a time.Duration or a string such as "1m",
	// over 0.
	ConfigFieldJWKSMinRefresh = "jwks_min_refresh"

	// ConfigFieldJWKSTimeout is the field holding the time a fetch of the
	// discovery document and of the key set may take, a time.Duration or
	// a string such as "10s", over 0.
	ConfigFieldJWKSTimeout = "jwks_timeout"

	// ConfigFieldJWKSClient is the field holding the id of the
	// [github.com/cjdias/flam-in-go/pkg/flam.IClient] fetching the key
	// set, taken from the client factory of the container, a string,
	// empty for a plain HTTP client.
	ConfigFieldJWKSClient = "jwks_client"

	// ConfigFieldIssuer is the field holding the issuer the "iss" claim of
	// the tokens must hold, a string, not checked when empty, unless
	// [ConfigFieldIssuerURL] is set.
	ConfigFieldIssuer = "issuer"

	// ConfigFieldAudience is the field holding the audience the "aud"
	// claim of the tokens must hold, a string, not checked when empty.
	ConfigFieldAudience = "audience"

	// ConfigFieldLeeway is the field holding the leeway of the time
	// claims, a time.Duration or a string such as "30s", 0 or more.
	ConfigFieldLeeway = "leeway"

	// ConfigFieldOptional is the field holding the bool telling whether a
	// request without a bearer token is served, without claims.
	ConfigFieldOptional = "optional"

	// ConfigFieldRequireExpiration is the field holding the bool telling
	// whether the tokens must hold an "exp" claim.
	ConfigFieldRequireExpiration = "require_expiration"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration. The secret, the public keys and the URLs have
// none, as they belong to each middleware.
const (
	// DefaultAlgorithmsConfigPath is the path of the default of
	// [ConfigFieldAlgorithms], "HS256" unless changed.
	DefaultAlgorithmsConfigPath = "flam.rest.defaults.middlewares.jwt.algorithms"

	// DefaultIssuerConfigPath is the path of the default of
	// [ConfigFieldIssuer], empty unless changed.
	DefaultIssuerConfigPath = "flam.rest.defaults.middlewares.jwt.issuer"

	// DefaultAudienceConfigPath is the path of the default of
	// [ConfigFieldAudience], empty unless changed.
	DefaultAudienceConfigPath = "flam.rest.defaults.middlewares.jwt.audience"

	// DefaultLeewayConfigPath is the path of the default of
	// [ConfigFieldLeeway], "0s" unless changed.
	DefaultLeewayConfigPath = "flam.rest.defaults.middlewares.jwt.leeway"

	// DefaultOptionalConfigPath is the path of the default of
	// [ConfigFieldOptional], false unless changed.
	DefaultOptionalConfigPath = "flam.rest.defaults.middlewares.jwt.optional"

	// DefaultRequireExpirationConfigPath is the path of the default of
	// [ConfigFieldRequireExpiration], true unless changed.
	DefaultRequireExpirationConfigPath = "flam.rest.defaults.middlewares.jwt.require_expiration"

	// DefaultJWKSRefreshConfigPath is the path of the default of
	// [ConfigFieldJWKSRefresh], "1h" unless changed.
	DefaultJWKSRefreshConfigPath = "flam.rest.defaults.middlewares.jwt.jwks_refresh"

	// DefaultJWKSMinRefreshConfigPath is the path of the default of
	// [ConfigFieldJWKSMinRefresh], "1m" unless changed.
	DefaultJWKSMinRefreshConfigPath = "flam.rest.defaults.middlewares.jwt.jwks_min_refresh"

	// DefaultJWKSTimeoutConfigPath is the path of the default of
	// [ConfigFieldJWKSTimeout], "10s" unless changed.
	DefaultJWKSTimeoutConfigPath = "flam.rest.defaults.middlewares.jwt.jwks_timeout"

	// DefaultJWKSClientConfigPath is the path of the default of
	// [ConfigFieldJWKSClient], empty unless changed.
	DefaultJWKSClientConfigPath = "flam.rest.defaults.middlewares.jwt.jwks_client"
)

// JWKSMaxSize is the maximum size, in bytes, of the discovery document and
// of the key set, a larger one failing the fetch.
const JWKSMaxSize = 1 << 20

// Values of the WWW-Authenticate header of the 401 answers, as RFC 6750
// defines them.
const (
	// ChallengeMissing is the challenge answering a request without a
	// bearer token.
	ChallengeMissing = "Bearer"

	// ChallengeInvalid is the challenge answering a request with a token
	// failing verification.
	ChallengeInvalid = `Bearer error="invalid_token"`
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultIssuer            = ""
	defaultAudience          = ""
	defaultLeeway            = time.Duration(0)
	defaultOptional          = false
	defaultRequireExpiration = true
	defaultJWKSRefresh       = time.Hour
	defaultJWKSMinRefresh    = time.Minute
	defaultJWKSTimeout       = 10 * time.Second
	defaultJWKSClient        = ""
)

// defaultAlgorithms returns the default of [ConfigFieldAlgorithms], a new
// list each time, so that the configuration never shares it.
func defaultAlgorithms() []string {
	return []string{"HS256"}
}
