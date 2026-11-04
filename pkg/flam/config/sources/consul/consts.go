package consul

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.config.sources.consul"

// Driver is the driver name of the consul sources, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.config.sources.driver.consul"

// Fields of a consul source configuration.
const (
	// ConfigFieldPriority is the field holding the int priority of the
	// source.
	ConfigFieldPriority = "priority"

	// ConfigFieldReload is the field holding the bool telling whether the
	// source is reloaded: when false, the source does not implement
	// [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
	// its values never change.
	ConfigFieldReload = "reload"

	// ConfigFieldAddress is the field holding the URL of the Consul agent,
	// an absolute http or https URL, such as "http://127.0.0.1:8500".
	ConfigFieldAddress = "address"

	// ConfigFieldToken is the field holding the ACL token of the requests,
	// empty for the one of the CONSUL_HTTP_TOKEN environment variable, or
	// none.
	ConfigFieldToken = "token"

	// ConfigFieldDatacenter is the field holding the datacenter of the
	// requests, empty for the one of the agent.
	ConfigFieldDatacenter = "datacenter"

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
	// string such as "10s", bounding each request, zero for no bound.
	ConfigFieldTimeout = "timeout"

	// ConfigFieldCAFile is the field holding the path of the PEM file of
	// the certificate authorities trusted for the agent, empty for those
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
	// the certificate of the agent is not verified.
	ConfigFieldSkipVerify = "skip_verify"
)

// Configuration paths of the values taken by the fields missing from a
// source configuration.
const (
	// DefaultPriorityConfigPath is the path of the default of
	// [ConfigFieldPriority], 0 unless changed.
	DefaultPriorityConfigPath = "flam.config.defaults.sources.consul.priority"

	// DefaultReloadConfigPath is the path of the default of
	// [ConfigFieldReload], true unless changed.
	DefaultReloadConfigPath = "flam.config.defaults.sources.consul.reload"

	// DefaultAddressConfigPath is the path of the default of
	// [ConfigFieldAddress], "http://127.0.0.1:8500" unless changed.
	DefaultAddressConfigPath = "flam.config.defaults.sources.consul.address"

	// DefaultTokenConfigPath is the path of the default of
	// [ConfigFieldToken], empty unless changed.
	DefaultTokenConfigPath = "flam.config.defaults.sources.consul.token"

	// DefaultDatacenterConfigPath is the path of the default of
	// [ConfigFieldDatacenter], empty unless changed.
	DefaultDatacenterConfigPath = "flam.config.defaults.sources.consul.datacenter"

	// DefaultParserConfigPath is the path of the default of
	// [ConfigFieldParser], "json" unless changed.
	DefaultParserConfigPath = "flam.config.defaults.sources.consul.parser"

	// DefaultTimeoutConfigPath is the path of the default of
	// [ConfigFieldTimeout], 10 seconds unless changed.
	DefaultTimeoutConfigPath = "flam.config.defaults.sources.consul.timeout"

	// DefaultCAFileConfigPath is the path of the default of
	// [ConfigFieldCAFile], empty unless changed.
	DefaultCAFileConfigPath = "flam.config.defaults.sources.consul.ca_file"

	// DefaultCertFileConfigPath is the path of the default of
	// [ConfigFieldCertFile], empty unless changed.
	DefaultCertFileConfigPath = "flam.config.defaults.sources.consul.cert_file"

	// DefaultKeyFileConfigPath is the path of the default of
	// [ConfigFieldKeyFile], empty unless changed.
	DefaultKeyFileConfigPath = "flam.config.defaults.sources.consul.key_file"

	// DefaultSkipVerifyConfigPath is the path of the default of
	// [ConfigFieldSkipVerify], false unless changed.
	DefaultSkipVerifyConfigPath = "flam.config.defaults.sources.consul.skip_verify"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the sources.
const (
	// ErrorFieldID holds the id of the source.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldKey holds a Consul key: the configured key, or a key of the
	// tree read under it.
	ErrorFieldKey = "key"

	// ErrorFieldParser holds the id of the parser.
	ErrorFieldParser = "parser"
)

// Defaults of the fields of a source configuration.
const (
	defaultPriority   = 0
	defaultReload     = true
	defaultAddress    = "http://127.0.0.1:8500"
	defaultToken      = ""
	defaultDatacenter = ""
	defaultRecursive  = false
	defaultParser     = "json"
	defaultTimeout    = 10 * time.Second
	defaultCAFile     = ""
	defaultCertFile   = ""
	defaultKeyFile    = ""
	defaultSkipVerify = false
)
