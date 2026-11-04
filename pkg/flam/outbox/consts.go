package outbox

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.outbox"

// RelayConfigPath is the configuration path holding one relay
// configuration per relay id, read by the bundle returned from
// [NewBundle] when it provides the relays.
const RelayConfigPath = "flam.outbox.relays"

// Fields of a relay configuration.
const (
	// ConfigFieldConnection is the field holding the id of the database
	// connection of the relay, in the connection factory.
	ConfigFieldConnection = "connection"

	// ConfigFieldBroker is the field holding the id of the broker the
	// relay publishes the messages with, in the broker factory.
	ConfigFieldBroker = "broker"

	// ConfigFieldProcess is the field holding the id of the relay process,
	// in the kennel, "outbox-" followed by the relay id unless set.
	ConfigFieldProcess = "process"

	// ConfigFieldPoll is the field holding the delay between two polls of
	// the relay once it found less than a batch of messages, a positive
	// duration such as "1s".
	ConfigFieldPoll = "poll"

	// ConfigFieldBatch is the field holding the number of messages the
	// relay claims at once, 1 or more.
	ConfigFieldBatch = "batch"

	// ConfigFieldLease is the field holding the time the relay holds the
	// messages it claimed, a positive duration: once elapsed, the
	// messages are taken as abandoned and claimed again, so that it must
	// exceed the time a batch takes to publish.
	ConfigFieldLease = "lease"

	// ConfigFieldBackoff is the field holding the delay before the first
	// retry of a message the broker refused, a duration doubled at each
	// following retry.
	ConfigFieldBackoff = "backoff"

	// ConfigFieldMaxBackoff is the field holding the maximum delay before
	// the retry of a message, a duration.
	ConfigFieldMaxBackoff = "max_backoff"

	// ConfigFieldDelete is the field holding whether the relay deletes the
	// messages it published, rather than marking them sent, a bool.
	ConfigFieldDelete = "delete"

	// ConfigFieldRetention is the field holding the time the sent messages
	// are kept for, a duration.
	ConfigFieldRetention = "retention"

	// ConfigFieldCleanup is the field holding the delay between two
	// deletions of the sent messages past their retention, a positive
	// duration.
	ConfigFieldCleanup = "cleanup"

	// ConfigFieldMigrate is the field holding whether the relay creates or
	// updates the [Table] table of its database when created, a bool.
	ConfigFieldMigrate = "migrate"
)

// Configuration paths of the values taken by the fields missing from a
// relay configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], unset unless configured.
	DefaultConnectionConfigPath = "flam.outbox.defaults.relays.connection"

	// DefaultBrokerConfigPath is the path of the default of
	// [ConfigFieldBroker], unset unless configured.
	DefaultBrokerConfigPath = "flam.outbox.defaults.relays.broker"

	// DefaultPollConfigPath is the path of the default of
	// [ConfigFieldPoll], 1s unless configured.
	DefaultPollConfigPath = "flam.outbox.defaults.relays.poll"

	// DefaultBatchConfigPath is the path of the default of
	// [ConfigFieldBatch], 100 unless configured.
	DefaultBatchConfigPath = "flam.outbox.defaults.relays.batch"

	// DefaultLeaseConfigPath is the path of the default of
	// [ConfigFieldLease], 1m unless configured.
	DefaultLeaseConfigPath = "flam.outbox.defaults.relays.lease"

	// DefaultBackoffConfigPath is the path of the default of
	// [ConfigFieldBackoff], 1s unless configured.
	DefaultBackoffConfigPath = "flam.outbox.defaults.relays.backoff"

	// DefaultMaxBackoffConfigPath is the path of the default of
	// [ConfigFieldMaxBackoff], 1m unless configured.
	DefaultMaxBackoffConfigPath = "flam.outbox.defaults.relays.max_backoff"

	// DefaultDeleteConfigPath is the path of the default of
	// [ConfigFieldDelete], false unless configured.
	DefaultDeleteConfigPath = "flam.outbox.defaults.relays.delete"

	// DefaultRetentionConfigPath is the path of the default of
	// [ConfigFieldRetention], 24h unless configured.
	DefaultRetentionConfigPath = "flam.outbox.defaults.relays.retention"

	// DefaultCleanupConfigPath is the path of the default of
	// [ConfigFieldCleanup], 1m unless configured.
	DefaultCleanupConfigPath = "flam.outbox.defaults.relays.cleanup"

	// DefaultMigrateConfigPath is the path of the default of
	// [ConfigFieldMigrate], true unless configured.
	DefaultMigrateConfigPath = "flam.outbox.defaults.relays.migrate"
)

// Table is the name of the table holding the messages of the outbox.
const Table = "flam_outbox"

// LogChannel is the channel of the entries the relays log, when the
// container provides a [flam.ILogger].
const LogChannel = "flam.outbox"

// Context fields of the [*flam.Error] values returned by the outbox
// package, and of the entries the relays log.
const (
	// ErrorFieldID holds the id of a relay.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field,
	// or the path of an invalid relay configuration.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the connection of a relay.
	ErrorFieldConnection = "connection"

	// ErrorFieldBroker holds the id of the broker of a relay.
	ErrorFieldBroker = "broker"

	// ErrorFieldTopic holds the topic of a message.
	ErrorFieldTopic = "topic"

	// ErrorFieldMessage holds the id of a message.
	ErrorFieldMessage = "message"
)

// Defaults of the fields of a relay configuration.
const (
	defaultPoll       = time.Second
	defaultBatch      = 100
	defaultLease      = time.Minute
	defaultBackoff    = time.Second
	defaultMaxBackoff = time.Minute
	defaultDelete     = false
	defaultRetention  = 24 * time.Hour
	defaultCleanup    = time.Minute
	defaultMigrate    = true

	// processPrefix prefixes the relay id in the default process id.
	processPrefix = "outbox-"
)
