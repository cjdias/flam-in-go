package database

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.database"

// Configuration paths and container groups of the configurations, dialects
// and connections.
const (
	// ConfigConfigPath is the configuration path holding one gorm
	// configuration per configuration id, as read by the configuration
	// factory.
	ConfigConfigPath = "flam.database.configs"

	// ConfigCreatorGroup is the container group the configuration driver
	// bundles provide their [flam.IDatabaseConfigCreator] in, collected by
	// the configuration factory.
	ConfigCreatorGroup = "flam.database.configs.creator"

	// DialectConfigPath is the configuration path holding one dialect
	// configuration per dialect id, as read by the dialect factory.
	DialectConfigPath = "flam.database.dialects"

	// DialectCreatorGroup is the container group the dialect driver bundles
	// provide their [flam.IDatabaseDialectCreator] in, collected by the
	// dialect factory.
	DialectCreatorGroup = "flam.database.dialects.creator"

	// ConnectionConfigPath is the configuration path holding one connection
	// configuration per connection id, as read by the connection factory.
	ConnectionConfigPath = "flam.database.connections"

	// ConnectionCreatorGroup is the container group holding the
	// [flam.IDatabaseConnectionCreator] values collected by the connection
	// factory, among which the one of the bundle returned by [NewBundle].
	ConnectionCreatorGroup = "flam.database.connections.creator"
)

// Driver is the driver name of the connections created by the creator of
// the bundle returned by [NewBundle], held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration, which may also be left out.
const Driver = "flam.database.connections.driver.gorm"

// Fields of a connection configuration.
const (
	// ConfigFieldDialect is the field holding the id of the dialect of the
	// connection, in the dialect factory.
	ConfigFieldDialect = "dialect"

	// ConfigFieldConfig is the field holding the id of the gorm
	// configuration of the connection, in the configuration factory.
	ConfigFieldConfig = "config"

	// ConfigFieldReplicas is the field holding the ids of the dialects of
	// the read replicas of the connection, a list, in the dialect factory.
	// The queries of a connection with replicas, but those of a
	// transaction, run on one of them, chosen by the [ConfigFieldPolicy]
	// policy, while the statements writing run on the dialect of the
	// connection, or on its sources.
	ConfigFieldReplicas = "replicas"

	// ConfigFieldSources is the field holding the ids of the dialects of
	// the extra write sources of the connection, a list, in the dialect
	// factory. The statements writing run on one of the dialect of the
	// connection and its sources, chosen by the [ConfigFieldPolicy]
	// policy, as do the queries of a connection without replicas.
	ConfigFieldSources = "sources"

	// ConfigFieldPolicy is the field holding the policy choosing among the
	// replicas, or the sources, of a statement, [PolicyRandom] or
	// [PolicyRoundRobin].
	ConfigFieldPolicy = "policy"

	// ConfigFieldConnectRetries is the field holding the whole number of
	// times the opening of the connection, which pings the database unless
	// the gorm configuration disables it, is retried when it fails, 0, for
	// none, or more.
	ConfigFieldConnectRetries = "connect_retries"

	// ConfigFieldConnectRetryDelay is the field holding the duration
	// waited, with the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of
	// the container, before the first retry of the opening of the
	// connection, a [time.Duration] or a string such as "1s", doubled
	// before each next one, up to [ConnectRetryMaxDelay] or the duration
	// itself when longer.
	ConfigFieldConnectRetryDelay = "connect_retry_delay"
)

// Policies choosing among the replicas, or the sources, of a connection,
// held by its [ConfigFieldPolicy] field.
const (
	// PolicyRandom chooses at random.
	PolicyRandom = "random"

	// PolicyRoundRobin chooses each in turn.
	PolicyRoundRobin = "round_robin"
)

// ConnectRetryMaxDelay is the longest wait between two attempts to open a
// connection, unless its [ConfigFieldConnectRetryDelay] is longer.
const ConnectRetryMaxDelay = 30 * time.Second

// Configuration paths of the values taken by the fields missing from a
// connection configuration.
const (
	// DefaultDialectConfigPath is the path of the default of
	// [ConfigFieldDialect], unset unless configured.
	DefaultDialectConfigPath = "flam.database.defaults.connections.dialect"

	// DefaultConfigConfigPath is the path of the default of
	// [ConfigFieldConfig], set by the bundle of
	// [github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard] to
	// its configuration.
	DefaultConfigConfigPath = "flam.database.defaults.connections.config"

	// DefaultPolicyConfigPath is the path of the default of
	// [ConfigFieldPolicy], [PolicyRandom] unless configured.
	DefaultPolicyConfigPath = "flam.database.defaults.connections.policy"

	// DefaultConnectRetriesConfigPath is the path of the default of
	// [ConfigFieldConnectRetries], 0 unless configured.
	DefaultConnectRetriesConfigPath = "flam.database.defaults.connections.connect_retries"

	// DefaultConnectRetryDelayConfigPath is the path of the default of
	// [ConfigFieldConnectRetryDelay], one second unless configured.
	DefaultConnectRetryDelayConfigPath = "flam.database.defaults.connections.connect_retry_delay"
)

// Defaults of the options of [Retry], taken by their zero values.
const (
	// DefaultRetryAttempts is the default number of attempts.
	DefaultRetryAttempts = 3

	// DefaultRetryDelay is the default wait before the first retry.
	DefaultRetryDelay = 50 * time.Millisecond

	// DefaultRetryMaxDelay is the default longest wait between two
	// attempts.
	DefaultRetryMaxDelay = time.Second
)

// Context fields of the [*flam.Error] values returned by the database
// package.
const (
	// ErrorFieldID holds the id of a connection or a dialect.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldDialect holds the id of the dialect of a connection.
	ErrorFieldDialect = "dialect"

	// ErrorFieldConfig holds the id of the gorm configuration of a
	// connection.
	ErrorFieldConfig = "config"

	// ErrorFieldAttempts holds the number of attempts made by [Retry].
	ErrorFieldAttempts = "attempts"
)

// Defaults of the fields of a connection configuration.
const (
	defaultPolicy            = PolicyRandom
	defaultConnectRetries    = 0
	defaultConnectRetryDelay = time.Second
)
