package gcpsm

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.stores.gcpsm"

// Driver is the driver name of the gcpsm stores, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.secrets.stores.driver.gcpsm"

// Fields of a gcpsm store configuration.
const (
	// ConfigFieldProject is the field holding the required id, or number,
	// of the Google Cloud project of the secrets.
	ConfigFieldProject = "project"

	// ConfigFieldVersion is the field holding the version of the secrets
	// read, a number or an alias, such as "latest".
	ConfigFieldVersion = "version"

	// ConfigFieldEndpoint is the field holding the host:port of the
	// service, replacing the global one, such as the one of a regional
	// endpoint or of an emulator, empty for the global one.
	ConfigFieldEndpoint = "endpoint"

	// ConfigFieldCredentialsFile is the field holding the path of the JSON
	// file of the credentials, empty for the Application Default
	// Credentials.
	ConfigFieldCredentialsFile = "credentials_file"

	// ConfigFieldCredentialsJSON is the field holding the JSON of the
	// credentials, used when [ConfigFieldCredentialsFile] is empty, empty
	// for the Application Default Credentials.
	ConfigFieldCredentialsJSON = "credentials_json"

	// ConfigFieldCredentialsType is the field holding the type of the
	// credentials of [ConfigFieldCredentialsFile] or
	// [ConfigFieldCredentialsJSON]: "service_account", "authorized_user",
	// "impersonated_service_account" or "external_account".
	ConfigFieldCredentialsType = "credentials_type"

	// ConfigFieldInsecure is the field holding the bool telling whether
	// the service of [ConfigFieldEndpoint] is reached without TLS nor
	// authentication, for an emulator or tests only.
	ConfigFieldInsecure = "insecure"

	// ConfigFieldTimeout is the field holding the time.Duration, or the
	// string such as "10s", bounding each read, retries included, zero for
	// no bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldCacheTTL is the field holding the time.Duration, or the
	// string such as "30s", the value of a secret is kept for, zero
	// disabling the cache.
	ConfigFieldCacheTTL = "cache_ttl"
)

// Configuration paths of the values taken by the fields missing from a
// store configuration.
const (
	// DefaultProjectConfigPath is the path of the default of
	// [ConfigFieldProject], empty, so required, unless changed.
	DefaultProjectConfigPath = "flam.secrets.defaults.stores.gcpsm.project"

	// DefaultVersionConfigPath is the path of the default of
	// [ConfigFieldVersion], "latest" unless changed.
	DefaultVersionConfigPath = "flam.secrets.defaults.stores.gcpsm.version"

	// DefaultEndpointConfigPath is the path of the default of
	// [ConfigFieldEndpoint], empty unless changed.
	DefaultEndpointConfigPath = "flam.secrets.defaults.stores.gcpsm.endpoint"

	// DefaultCredentialsFileConfigPath is the path of the default of
	// [ConfigFieldCredentialsFile], empty unless changed.
	DefaultCredentialsFileConfigPath = "flam.secrets.defaults.stores.gcpsm.credentials_file"

	// DefaultCredentialsJSONConfigPath is the path of the default of
	// [ConfigFieldCredentialsJSON], empty unless changed.
	DefaultCredentialsJSONConfigPath = "flam.secrets.defaults.stores.gcpsm.credentials_json"

	// DefaultCredentialsTypeConfigPath is the path of the default of
	// [ConfigFieldCredentialsType], "service_account" unless changed.
	DefaultCredentialsTypeConfigPath = "flam.secrets.defaults.stores.gcpsm.credentials_type"

	// DefaultInsecureConfigPath is the path of the default of
	// [ConfigFieldInsecure], false unless changed.
	DefaultInsecureConfigPath = "flam.secrets.defaults.stores.gcpsm.insecure"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 10 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.secrets.defaults.stores.gcpsm.timeout"

	// DefaultCacheTTLConfigPath is the path of the default of
	// [ConfigFieldCacheTTL], 30 seconds unless changed.
	DefaultCacheTTLConfigPath = "flam.secrets.defaults.stores.gcpsm.cache_ttl"
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

// Defaults of the fields of a store configuration.
const (
	defaultProject         = ""
	defaultVersion         = "latest"
	defaultEndpoint        = ""
	defaultCredentialsFile = ""
	defaultCredentialsJSON = ""
	defaultCredentialsType = "service_account"
	defaultInsecure        = false
	defaultTimeout         = 10 * time.Second
	defaultCacheTTL        = 30 * time.Second
)
