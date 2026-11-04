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

	// ConfigFieldIssuer is the field holding the issuer the "iss" claim of
	// the tokens must hold, a string, not checked when empty.
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
// middleware configuration. The secret and the public keys have none, as
// they belong to each middleware.
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
)

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
)

// defaultAlgorithms returns the default of [ConfigFieldAlgorithms], a new
// list each time, so that the configuration never shares it.
func defaultAlgorithms() []string {
	return []string{"HS256"}
}
