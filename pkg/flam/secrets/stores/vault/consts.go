package vault

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.secrets.stores.vault"

// Driver is the driver name of the vault stores, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.secrets.stores.driver.vault"

// EnvToken is the environment variable holding the token of the stores
// configured with neither [ConfigFieldToken] nor [ConfigFieldTokenFile].
const EnvToken = "VAULT_TOKEN"

// Headers of the requests sent to Vault.
const (
	// HeaderToken is the header holding the token of the store.
	HeaderToken = "X-Vault-Token"

	// HeaderNamespace is the header holding the namespace of the store,
	// when set.
	HeaderNamespace = "X-Vault-Namespace"
)

// Fields of a vault store configuration.
const (
	// ConfigFieldAddress is the field holding the URL of the server, an
	// absolute http or https URL, such as "https://vault:8200".
	ConfigFieldAddress = "address"

	// ConfigFieldMount is the field holding the non-empty mount path of the
	// KV version 2 secrets engine.
	ConfigFieldMount = "mount"

	// ConfigFieldNamespace is the field holding the namespace of the
	// requests, empty for none.
	ConfigFieldNamespace = "namespace"

	// ConfigFieldToken is the field holding the token of the requests.
	ConfigFieldToken = "token"

	// ConfigFieldTokenFile is the field holding the path of the file of
	// the token of the requests, read on each request, used when
	// [ConfigFieldToken] is empty.
	ConfigFieldTokenFile = "token_file"

	// ConfigFieldTimeout is the field holding the time.Duration, or the
	// string such as "10s", bounding each request, zero for no bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldCAFile is the field holding the path of the PEM file of
	// the certificate authorities trusted for the server, empty for those
	// of the system.
	ConfigFieldCAFile = "ca_file"

	// ConfigFieldSkipVerify is the field holding the bool telling whether
	// the certificate of the server is not verified.
	ConfigFieldSkipVerify = "skip_verify"

	// ConfigFieldCacheTTL is the field holding the time.Duration, or the
	// string such as "30s", the fields of a secret are kept for, zero
	// disabling the cache.
	ConfigFieldCacheTTL = "cache_ttl"
)

// Configuration paths of the values taken by the fields missing from a
// store configuration.
const (
	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], empty, so required, unless changed.
	DefaultAddressConfigPath = "flam.secrets.defaults.stores.vault.address"

	// DefaultMountConfigPath is the path of the default of
	// [ConfigFieldMount], "secret" unless changed.
	DefaultMountConfigPath = "flam.secrets.defaults.stores.vault.mount"

	// DefaultNamespaceConfigPath is the path of the default of
	// [ConfigFieldNamespace], empty unless changed.
	DefaultNamespaceConfigPath = "flam.secrets.defaults.stores.vault.namespace"

	// DefaultTokenConfigPath is the path of the default of
	// [ConfigFieldToken], empty unless changed.
	DefaultTokenConfigPath = "flam.secrets.defaults.stores.vault.token"

	// DefaultTokenFileConfigPath is the path of the default of
	// [ConfigFieldTokenFile], empty unless changed.
	DefaultTokenFileConfigPath = "flam.secrets.defaults.stores.vault.token_file"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 10 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.secrets.defaults.stores.vault.timeout"

	// DefaultCAFileConfigPath is the path of the default of
	// [ConfigFieldCAFile], empty unless changed.
	DefaultCAFileConfigPath = "flam.secrets.defaults.stores.vault.ca_file"

	// DefaultSkipVerifyConfigPath is the path of the default of
	// [ConfigFieldSkipVerify], false unless changed.
	DefaultSkipVerifyConfigPath = "flam.secrets.defaults.stores.vault.skip_verify"

	// DefaultCacheTTLConfigPath is the path of the default of
	// [ConfigFieldCacheTTL], 30 seconds unless changed.
	DefaultCacheTTLConfigPath = "flam.secrets.defaults.stores.vault.cache_ttl"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the stores, along with those of
// [github.com/cjdias/flam-in-go/pkg/flam/secrets].
const (
	// ErrorFieldID holds the id of the store.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds the key of a secret, or the path given to
	// GetMap.
	ErrorFieldKey = "key"

	// ErrorFieldStatus holds the HTTP status code of a response.
	ErrorFieldStatus = "status"
)

// Defaults of the fields of a store configuration, and the bound of the
// size of a response.
const (
	defaultAddress    = ""
	defaultMount      = "secret"
	defaultNamespace  = ""
	defaultToken      = ""
	defaultTokenFile  = ""
	defaultTimeout    = 10 * time.Second
	defaultCAFile     = ""
	defaultSkipVerify = false
	defaultCacheTTL   = 30 * time.Second
	maxResponseSize   = 1 << 20
)
