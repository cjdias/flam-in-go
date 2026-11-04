package awssm

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.stores.awssm"

// Driver is the driver name of the awssm stores, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.secrets.stores.driver.awssm"

// Fields of an awssm store configuration.
const (
	// ConfigFieldRegion is the field holding the AWS region of the
	// service, such as "eu-central-1", empty for the one of the
	// environment or of the shared configuration.
	ConfigFieldRegion = "region"

	// ConfigFieldEndpoint is the field holding the URL of the service,
	// replacing the one of the region, such as the one of a LocalStack,
	// empty for the one of the region.
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldProfile is the field holding the profile of the shared
	// configuration, empty for the default one.
	ConfigFieldProfile = "profile"

	// ConfigFieldAccessKeyID is the field holding the access key id of the
	// static credentials, along with [ConfigFieldSecretAccessKey], empty
	// for the credentials of the default chain.
	ConfigFieldAccessKeyID = "access_key_id"

	// ConfigFieldSecretAccessKey is the field holding the secret access key
	// of the static credentials.
	ConfigFieldSecretAccessKey = "secret_access_key"

	// ConfigFieldSessionToken is the field holding the session token of the
	// static credentials, empty for none.
	ConfigFieldSessionToken = "session_token"

	// ConfigFieldVersionStage is the field holding the non-empty staging
	// label of the versions read, such as "AWSCURRENT".
	ConfigFieldVersionStage = "version_stage"

	// ConfigFieldTimeout is the field holding the time.Duration, or the
	// string such as "10s", bounding each read, retries included, zero for
	// no bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldMaxAttempts is the field holding the positive int maximum
	// number of attempts of a request, retries included.
	ConfigFieldMaxAttempts = "max_attempts"

	// ConfigFieldCacheTTL is the field holding the time.Duration, or the
	// string such as "30s", the value of a secret is kept for, zero
	// disabling the cache.
	ConfigFieldCacheTTL = "cache_ttl"
)

// Configuration paths of the values taken by the fields missing from a
// store configuration.
const (
	// DefaultRegionConfigPath is the path of the default of
	// [ConfigFieldRegion], empty unless changed.
	DefaultRegionConfigPath = "flam.secrets.defaults.stores.awssm.region"

	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], empty unless changed.
	DefaultEndpointConfigPath = "flam.secrets.defaults.stores.awssm.endpoint"

	// DefaultProfileConfigPath is the path of the default of
	// [ConfigFieldProfile], empty unless changed.
	DefaultProfileConfigPath = "flam.secrets.defaults.stores.awssm.profile"

	// DefaultAccessKeyIDConfigPath is the path of the default of
	// [ConfigFieldAccessKeyID], empty unless changed.
	DefaultAccessKeyIDConfigPath = "flam.secrets.defaults.stores.awssm.access_key_id"

	// DefaultSecretAccessKeyConfigPath is the path of the default of
	// [ConfigFieldSecretAccessKey], empty unless changed.
	DefaultSecretAccessKeyConfigPath = "flam.secrets.defaults.stores.awssm.secret_access_key"

	// DefaultSessionTokenConfigPath is the path of the default of
	// [ConfigFieldSessionToken], empty unless changed.
	DefaultSessionTokenConfigPath = "flam.secrets.defaults.stores.awssm.session_token"

	// DefaultVersionStageConfigPath is the path of the default of
	// [ConfigFieldVersionStage], "AWSCURRENT" unless changed.
	DefaultVersionStageConfigPath = "flam.secrets.defaults.stores.awssm.version_stage"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 10 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.secrets.defaults.stores.awssm.timeout"

	// DefaultMaxAttemptsConfigPath is the path of the default of
	// [ConfigFieldMaxAttempts], 3 unless changed.
	DefaultMaxAttemptsConfigPath = "flam.secrets.defaults.stores.awssm.max_attempts"

	// DefaultCacheTTLConfigPath is the path of the default of
	// [ConfigFieldCacheTTL], 30 seconds unless changed.
	DefaultCacheTTLConfigPath = "flam.secrets.defaults.stores.awssm.cache_ttl"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the stores, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets].
const (
	// ErrorFieldID holds the id of the store.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds the key of a secret, or the name given to
	// GetMap.
	ErrorFieldKey = "key"
)

// Defaults of the fields of a store configuration, and the version stage
// read when none is configured.
const (
	defaultRegion          = ""
	defaultEndpoint        = ""
	defaultProfile         = ""
	defaultAccessKeyID     = ""
	defaultSecretAccessKey = ""
	defaultSessionToken    = ""
	defaultVersionStage    = "AWSCURRENT"
	defaultTimeout         = 10 * time.Second
	defaultMaxAttempts     = 3
	defaultCacheTTL        = 30 * time.Second
)
