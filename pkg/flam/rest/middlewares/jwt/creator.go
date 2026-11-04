package jwt

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config  flam.IConfig
	Clock   flam.IClock
	Clients flam.IClientFactory `optional:"true"`
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the jwt
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config:  args.Config,
		clock:   args.Clock,
		clients: args.Clients,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config  flam.IConfig
	clock   flam.IClock
	clients flam.IClientFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the algorithms, the key or the key set,
// the issuer, the audience, the leeway, the optionality and the expiration
// requirement of the configuration, or of their defaults, checking that
// every algorithm fits the key. No key set is fetched before a token needs
// it.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)
	fail := func(field string, cause error) (flam.IRestMiddleware, error) {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field, cause)
	}

	names, ok := settings.Strings(c.field(bag, ConfigFieldAlgorithms, DefaultAlgorithmsConfigPath, defaultAlgorithms()))
	if !ok || len(names) == 0 {
		return fail(ConfigFieldAlgorithms, nil)
	}
	keys, field, err := c.keyring(bag, names)
	if keys == nil {
		return fail(field, err)
	}
	issuer, ok := c.field(bag, ConfigFieldIssuer, DefaultIssuerConfigPath, defaultIssuer).(string)
	if !ok {
		return fail(ConfigFieldIssuer, nil)
	}
	if set, isSet := keys.(*jwks); isSet && issuer == "" {
		issuer = set.issuer
	}
	audience, ok := c.field(bag, ConfigFieldAudience, DefaultAudienceConfigPath, defaultAudience).(string)
	if !ok {
		return fail(ConfigFieldAudience, nil)
	}
	leeway, ok := settings.Duration(c.field(bag, ConfigFieldLeeway, DefaultLeewayConfigPath, defaultLeeway))
	if !ok {
		return fail(ConfigFieldLeeway, nil)
	}
	optional, ok := c.field(bag, ConfigFieldOptional, DefaultOptionalConfigPath, defaultOptional).(bool)
	if !ok {
		return fail(ConfigFieldOptional, nil)
	}
	requireExpiration, ok := c.field(bag, ConfigFieldRequireExpiration, DefaultRequireExpirationConfigPath, defaultRequireExpiration).(bool)
	if !ok {
		return fail(ConfigFieldRequireExpiration, nil)
	}

	options := []gojwt.ParserOption{
		gojwt.WithValidMethods(names),
		gojwt.WithLeeway(leeway),
		gojwt.WithTimeFunc(c.clock.Now),
	}
	if issuer != "" {
		options = append(options, gojwt.WithIssuer(issuer))
	}
	if audience != "" {
		options = append(options, gojwt.WithAudience(audience))
	}
	if requireExpiration {
		options = append(options, gojwt.WithExpirationRequired())
	}
	return &middleware{
		id:       id,
		parser:   gojwt.NewParser(options...),
		keys:     keys,
		optional: optional,
	}, nil
}

// field returns the value of the field of the middleware configuration,
// or else the one stored in the configuration at the default path, or
// else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// keyring returns the keyring of the configuration: the secret as bytes,
// the public key decoded from the PEM of the public key field or of the
// public key file, or the key set of the JWKS URL or of the issuer URL,
// checking that every algorithm of the names fits it. It fails, along with
// the field at fault, when none, or more than one, of the five fields is
// set, or a field is not a string, or the key cannot be read or decoded,
// or a URL is not an absolute https one, or an algorithm does not fit.
func (c *creator) keyring(
	bag flam.IBag,
	names []string,
) (keyring, string, error) {
	fields := []string{ConfigFieldSecret, ConfigFieldPublicKey, ConfigFieldPublicKeyFile, ConfigFieldJWKSURL, ConfigFieldIssuerURL}
	value, set := "", ""
	for _, field := range fields {
		v, ok := c.field(bag, field, "", "").(string)
		if !ok {
			return nil, field, errNotString
		}
		if v == "" {
			continue
		}
		if set != "" {
			return nil, field, errKeyConflict
		}
		value, set = v, field
	}

	switch set {
	case "":
		return nil, ConfigFieldSecret, errNoKey
	case ConfigFieldJWKSURL, ConfigFieldIssuerURL:
		if !creatorHTTPS(value) {
			return nil, set, errNotHTTPS
		}
		for _, name := range names {
			if err := creatorAsymmetric(name); err != nil {
				return nil, ConfigFieldAlgorithms, err
			}
		}
		return c.jwks(bag, set, value)
	}
	key, err := creatorKey(set, value)
	if err != nil {
		return nil, set, err
	}
	for _, name := range names {
		if err := creatorFits(name, key); err != nil {
			return nil, ConfigFieldAlgorithms, err
		}
	}
	return staticKey{value: key}, "", nil
}

// jwks returns the key set of the address of the field, the JWKS URL or the
// issuer URL, fetched with the client of the configuration, and refreshed
// as its durations tell, failing along with the field at fault.
func (c *creator) jwks(
	bag flam.IBag,
	field string,
	address string,
) (keyring, string, error) {
	durations := map[string]time.Duration{}
	for _, d := range []struct {
		field       string
		defaultPath string
		builtIn     time.Duration
	}{
		{ConfigFieldJWKSRefresh, DefaultJWKSRefreshConfigPath, defaultJWKSRefresh},
		{ConfigFieldJWKSMinRefresh, DefaultJWKSMinRefreshConfigPath, defaultJWKSMinRefresh},
		{ConfigFieldJWKSTimeout, DefaultJWKSTimeoutConfigPath, defaultJWKSTimeout},
	} {
		value, ok := settings.Duration(c.field(bag, d.field, d.defaultPath, d.builtIn))
		if !ok || value == 0 {
			return nil, d.field, nil
		}
		durations[d.field] = value
	}
	id, ok := c.field(bag, ConfigFieldJWKSClient, DefaultJWKSClientConfigPath, defaultJWKSClient).(string)
	if !ok {
		return nil, ConfigFieldJWKSClient, nil
	}
	var client jwksClient = &http.Client{}
	if id != "" {
		if c.clients == nil {
			return nil, ConfigFieldJWKSClient, errNoClientFactory
		}
		var err error
		if client, err = c.clients.Get(id); err != nil {
			return nil, ConfigFieldJWKSClient, err
		}
	}

	set := &jwks{
		clock:      c.clock,
		client:     client,
		refresh:    durations[ConfigFieldJWKSRefresh],
		minRefresh: durations[ConfigFieldJWKSMinRefresh],
		timeout:    durations[ConfigFieldJWKSTimeout],
	}
	if field == ConfigFieldJWKSURL {
		set.url = address
	} else {
		set.issuer = address
	}
	return set, "", nil
}

// creatorKey returns the key of the value of the field: the secret as
// bytes, or the public key decoded from the PEM of the public key field
// or of the public key file.
func creatorKey(
	field string,
	value string,
) (any, error) {
	switch field {
	case ConfigFieldSecret:
		return []byte(value), nil
	case ConfigFieldPublicKey:
		return creatorPublicKey([]byte(value))
	}
	data, err := os.ReadFile(value)
	if err != nil {
		return nil, err
	}
	return creatorPublicKey(data)
}

// creatorHTTPS reports whether the value is an absolute https URL, with a
// host, and without user information, query or fragment.
func creatorHTTPS(
	value string,
) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil &&
		u.RawQuery == "" && u.Fragment == "" && !u.ForceQuery
}

// creatorPublicKey returns the RSA, ECDSA or Ed25519 public key of the
// first PEM block of the data: a "CERTIFICATE", an "RSA PUBLIC KEY" or, for
// any other type, a "PUBLIC KEY".
func creatorPublicKey(
	data []byte,
) (any, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errNoPEM
	}
	var key any
	var err error
	switch block.Type {
	case "CERTIFICATE":
		var certificate *x509.Certificate
		if certificate, err = x509.ParseCertificate(block.Bytes); err == nil {
			key = certificate.PublicKey
		}
	case "RSA PUBLIC KEY":
		key, err = x509.ParsePKCS1PublicKey(block.Bytes)
	default:
		key, err = x509.ParsePKIXPublicKey(block.Bytes)
	}
	if err != nil {
		return nil, err
	}
	switch key.(type) {
	case *rsa.PublicKey, *ecdsa.PublicKey, ed25519.PublicKey:
		return key, nil
	}
	return nil, errKeyType
}

// creatorFits checks that the algorithm of the name is a known signing
// algorithm, other than "none", fitting the key: an HMAC one with a secret
// at least as long as its hash, an RSA one with an RSA key of 2048 bits or
// more, an ECDSA one with a key of its curve, or EdDSA with an Ed25519
// key.
func creatorFits(
	name string,
	key any,
) error {
	switch method := gojwt.GetSigningMethod(name).(type) {
	case *gojwt.SigningMethodHMAC:
		if secret, ok := key.([]byte); ok {
			if len(secret) < method.Hash.Size() {
				return errShortSecret
			}
			return nil
		}
	case *gojwt.SigningMethodRSA, *gojwt.SigningMethodRSAPSS:
		if public, ok := key.(*rsa.PublicKey); ok {
			if public.N.BitLen() < creatorMinRSABits {
				return errShortRSAKey
			}
			return nil
		}
	case *gojwt.SigningMethodECDSA:
		if public, ok := key.(*ecdsa.PublicKey); ok {
			if public.Curve.Params().BitSize != method.CurveBits {
				return errCurve
			}
			return nil
		}
	case *gojwt.SigningMethodEd25519:
		if _, ok := key.(ed25519.PublicKey); ok {
			return nil
		}
	default:
		return errAlgorithm
	}
	return errKeyMismatch
}

// creatorAsymmetric checks that the algorithm of the name is a known
// public key signing algorithm, the only ones verified with a key set.
func creatorAsymmetric(
	name string,
) error {
	switch gojwt.GetSigningMethod(name).(type) {
	case *gojwt.SigningMethodRSA, *gojwt.SigningMethodRSAPSS, *gojwt.SigningMethodECDSA, *gojwt.SigningMethodEd25519:
		return nil
	case *gojwt.SigningMethodHMAC:
		return errKeyMismatch
	}
	return errAlgorithm
}

// creatorMinRSABits is the minimum size of an RSA key.
const creatorMinRSABits = 2048

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create jwt rest middleware"

// Causes of the invalid configurations, joined to ErrInvalidField.
var (
	errNotString       = errors.New("key field not a string")
	errNoKey           = errors.New("one of secret, public_key, public_key_file, jwks_url or issuer_url must be set")
	errKeyConflict     = errors.New("only one of secret, public_key, public_key_file, jwks_url or issuer_url may be set")
	errNotHTTPS        = errors.New("url not an absolute https url")
	errNoClientFactory = errors.New("no client factory, register the client bundle")
	errNoPEM           = errors.New("no PEM block found")
	errKeyType         = errors.New("public key not of RSA, ECDSA or Ed25519 type")
	errAlgorithm       = errors.New("unknown or unsigned algorithm")
	errKeyMismatch     = errors.New("algorithm not fitting the key")
	errShortSecret     = errors.New("secret shorter than the hash of the algorithm")
	errShortRSAKey     = errors.New("rsa key shorter than 2048 bits")
	errCurve           = errors.New("ecdsa key not of the curve of the algorithm")
)
