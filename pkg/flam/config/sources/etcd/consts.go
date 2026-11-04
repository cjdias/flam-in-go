package etcd

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.sources.etcd"

// Driver is the driver name of the etcd sources, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.config.sources.driver.etcd"

// Fields of an etcd source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldEndpoints is the field holding the non-empty list of the
	// string endpoints of the cluster, such as "127.0.0.1:2379" or
	// "https://etcd:2379".
	ConfigFieldEndpoints = "endpoints"

	// ConfigFieldUsername is the field holding the user name of the
	// requests, empty for none.
	ConfigFieldUsername = "username"

	// ConfigFieldPassword is the field holding the password of the user
	// name.
	ConfigFieldPassword = "password"

	// ConfigFieldKey is the required field holding the key of the value to
	// read or, with [ConfigFieldRecursive], the prefix of the keys to read.
	ConfigFieldKey = "key"

	// ConfigFieldRecursive is the field holding the bool telling whether
	// every key under the [ConfigFieldKey] prefix is read, as a tree,
	// instead of the key alone.
	ConfigFieldRecursive = "recursive"

	// ConfigFieldParser is the field holding the id of the parser of the
	// values. Empty, with [ConfigFieldRecursive], it stores each value as
	// its string.
	ConfigFieldParser = "parser"

	// ConfigFieldTimeout is the field holding the time.Duration, or the
	// string such as "10s", bounding each request, and the authentication
	// of the client, zero for no bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldCAFile is the field holding the path of the PEM file of
	// the certificate authorities trusted for the cluster, empty for those
	// of the system.
	ConfigFieldCAFile = "ca_file"

	// ConfigFieldCertFile is the field holding the path of the PEM file of
	// the client certificate, along with [ConfigFieldKeyFile], empty for
	// none.
	ConfigFieldCertFile = "cert_file"

	// ConfigFieldKeyFile is the field holding the path of the PEM file of
	// the private key of the client certificate, empty for none.
	ConfigFieldKeyFile = "key_file"

	// ConfigFieldSkipVerify is the field holding the bool telling whether
	// the certificates of the cluster are not verified.
	ConfigFieldSkipVerify = "skip_verify"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration.
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.config.defaults.sources.etcd.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], true unless changed.
	DefaultReloadConfigPath = "flam.config.defaults.sources.etcd.reload"

	// DefaultEndpointsConfigPath is the path of the default of
	// [ConfigFieldEndpoints], ["127.0.0.1:2379"] unless changed.
	DefaultEndpointsConfigPath = "flam.config.defaults.sources.etcd.endpoints"

	// DefaultUsernameConfigPath is the path of the default of
	// [ConfigFieldUsername], empty unless changed.
	DefaultUsernameConfigPath = "flam.config.defaults.sources.etcd.username"

	// DefaultPasswordConfigPath is the path of the default of
	// [ConfigFieldPassword], empty unless changed.
	DefaultPasswordConfigPath = "flam.config.defaults.sources.etcd.password"

	// DefaultParserConfigPath is the path of the default of
	// [ConfigFieldParser], "json" unless changed.
	DefaultParserConfigPath = "flam.config.defaults.sources.etcd.parser"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 10 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.config.defaults.sources.etcd.timeout"

	// DefaultCAFileConfigPath is the path of the default of
	// [ConfigFieldCAFile], empty unless changed.
	DefaultCAFileConfigPath = "flam.config.defaults.sources.etcd.ca_file"

	// DefaultCertFileConfigPath is the path of the default of
	// [ConfigFieldCertFile], empty unless changed.
	DefaultCertFileConfigPath = "flam.config.defaults.sources.etcd.cert_file"

	// DefaultKeyFileConfigPath is the path of the default of
	// [ConfigFieldKeyFile], empty unless changed.
	DefaultKeyFileConfigPath = "flam.config.defaults.sources.etcd.key_file"

	// DefaultSkipVerifyConfigPath is the path of the default of
	// [ConfigFieldSkipVerify], false unless changed.
	DefaultSkipVerifyConfigPath = "flam.config.defaults.sources.etcd.skip_verify"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds an etcd key: the configured key, or a key of the
	// tree read under it.
	ErrorFieldKey = "key"

	// ErrorFieldParser holds the id of the parser.
	ErrorFieldParser = "parser"
)

// Defaults of the fields of a source configuration.
const (
	defaultPriority   = 0
	defaultReload     = true
	defaultEndpoint   = "127.0.0.1:2379"
	defaultUsername   = ""
	defaultPassword   = ""
	defaultRecursive  = false
	defaultParser     = "json"
	defaultTimeout    = 10 * time.Second
	defaultCAFile     = ""
	defaultCertFile   = ""
	defaultKeyFile    = ""
	defaultSkipVerify = false
)
