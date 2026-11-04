package kafka

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.broker.brokers.kafka"

// Driver is the driver name of the Kafka brokers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.broker.brokers.driver.kafka"

// messagingSystem is the messaging system of the spans of the brokers,
// held by their "messaging.system" attribute.
const messagingSystem = "kafka"

// HeaderKey is the header holding the key of a Kafka message: given to
// Publish, it becomes the key of the message, which the hash balancer
// sends to the partition of the key, and not a header of it, and a
// received message with a key holds it.
const HeaderKey = "kafka.key"

// Balancers of the partitions of the messages, held by the
// [ConfigFieldBalancer] field.
const (
	// BalancerHash sends the messages of a key to the same partition, and
	// those without key to the partitions in turn.
	BalancerHash = "hash"

	// BalancerRoundRobin sends the messages to the partitions in turn.
	BalancerRoundRobin = "round_robin"

	// BalancerLeastBytes sends the messages to the partition that received
	// the fewest bytes.
	BalancerLeastBytes = "least_bytes"
)

// Acknowledgements a publication waits for, held by the
// [ConfigFieldRequiredAcks] field.
const (
	// AcksNone waits for no acknowledgement.
	AcksNone = "none"

	// AcksOne waits for the acknowledgement of the leader of the
	// partition.
	AcksOne = "one"

	// AcksAll waits for the acknowledgement of every in-sync replica of
	// the partition.
	AcksAll = "all"
)

// Offsets a new consumer group starts reading a partition from, held by
// the [ConfigFieldStartOffset] field.
const (
	// OffsetFirst reads the partitions from their first message.
	OffsetFirst = "first"

	// OffsetLast reads the messages published after the group joined.
	OffsetLast = "last"
)

// Fields of a Kafka broker configuration.
const (
	// ConfigFieldBrokers is the field holding the list of the addresses of
	// the bootstrap brokers of the cluster, such as ["127.0.0.1:9092"].
	ConfigFieldBrokers = "brokers"

	// ConfigFieldGroupID is the field holding the consumer group of the
	// subscriptions, each message going to one subscription of the group,
	// the broker id when empty.
	ConfigFieldGroupID = "group_id"

	// ConfigFieldBalancer is the field holding the balancer of the
	// partitions of the messages published, [BalancerHash],
	// [BalancerRoundRobin] or [BalancerLeastBytes].
	ConfigFieldBalancer = "balancer"

	// ConfigFieldRequiredAcks is the field holding the acknowledgements a
	// publication waits for, [AcksNone], [AcksOne] or [AcksAll].
	ConfigFieldRequiredAcks = "required_acks"

	// ConfigFieldAsync is the field holding whether Publish returns
	// without waiting for the batch of the message to be written, a bool,
	// its errors being lost.
	ConfigFieldAsync = "async"

	// ConfigFieldBatchSize is the field holding the maximum number of
	// messages of a batch, a whole number, 1 or more.
	ConfigFieldBatchSize = "batch_size"

	// ConfigFieldBatchBytes is the field holding the maximum size of a
	// batch, in bytes, a whole number, 1 or more.
	ConfigFieldBatchBytes = "batch_bytes"

	// ConfigFieldBatchTimeout is the field holding the time a batch waits
	// for more messages before being written, a duration such as "10ms".
	ConfigFieldBatchTimeout = "batch_timeout"

	// ConfigFieldWriteTimeout is the field holding the time allowed to
	// write a batch, a duration such as "10s".
	ConfigFieldWriteTimeout = "write_timeout"

	// ConfigFieldMaxAttempts is the field holding the maximum number of
	// attempts to write a batch, a whole number, 1 or more.
	ConfigFieldMaxAttempts = "max_attempts"

	// ConfigFieldAutoCreateTopic is the field holding whether publishing
	// on a missing topic creates it, a bool, as the cluster allows.
	ConfigFieldAutoCreateTopic = "auto_create_topic"

	// ConfigFieldMinBytes is the field holding the minimum size of a fetch
	// of the readers, in bytes, a whole number, 1 or more.
	ConfigFieldMinBytes = "min_bytes"

	// ConfigFieldMaxBytes is the field holding the maximum size of a fetch
	// of the readers, in bytes, a whole number, the minimum size or more.
	ConfigFieldMaxBytes = "max_bytes"

	// ConfigFieldMaxWait is the field holding the time a fetch waits for
	// the minimum size, a duration such as "1s".
	ConfigFieldMaxWait = "max_wait"

	// ConfigFieldStartOffset is the field holding the offset a new
	// consumer group starts from, [OffsetFirst] or [OffsetLast].
	ConfigFieldStartOffset = "start_offset"

	// ConfigFieldCommitInterval is the field holding the interval at which
	// the offsets of the messages handled are committed, a duration, 0
	// committing each one before handling the next.
	ConfigFieldCommitInterval = "commit_interval"

	// ConfigFieldRetryBackoff is the field holding the time waited before
	// handling again a message whose handler failed, a positive duration
	// such as "1s".
	ConfigFieldRetryBackoff = "retry_backoff"
)

// Configuration paths of the values taken by the fields missing from a
// broker configuration.
const (
	// DefaultBrokersConfigPath is the path of the default of
	// [ConfigFieldBrokers], ["127.0.0.1:9092"] unless set.
	DefaultBrokersConfigPath = "flam.broker.defaults.brokers.kafka.brokers"

	// DefaultBalancerConfigPath is the path of the default of
	// [ConfigFieldBalancer], [BalancerHash] unless set.
	DefaultBalancerConfigPath = "flam.broker.defaults.brokers.kafka.balancer"

	// DefaultRequiredAcksConfigPath is the path of the default of
	// [ConfigFieldRequiredAcks], [AcksAll] unless set.
	DefaultRequiredAcksConfigPath = "flam.broker.defaults.brokers.kafka.required_acks"

	// DefaultBatchSizeConfigPath is the path of the default of
	// [ConfigFieldBatchSize], 100 unless set.
	DefaultBatchSizeConfigPath = "flam.broker.defaults.brokers.kafka.batch_size"

	// DefaultBatchBytesConfigPath is the path of the default of
	// [ConfigFieldBatchBytes], 1048576 unless set.
	DefaultBatchBytesConfigPath = "flam.broker.defaults.brokers.kafka.batch_bytes"

	// DefaultBatchTimeoutConfigPath is the path of the default of
	// [ConfigFieldBatchTimeout], "10ms" unless set.
	DefaultBatchTimeoutConfigPath = "flam.broker.defaults.brokers.kafka.batch_timeout"

	// DefaultWriteTimeoutConfigPath is the path of the default of
	// [ConfigFieldWriteTimeout], "10s" unless set.
	DefaultWriteTimeoutConfigPath = "flam.broker.defaults.brokers.kafka.write_timeout"

	// DefaultMaxAttemptsConfigPath is the path of the default of
	// [ConfigFieldMaxAttempts], 10 unless set.
	DefaultMaxAttemptsConfigPath = "flam.broker.defaults.brokers.kafka.max_attempts"

	// DefaultMinBytesConfigPath is the path of the default of
	// [ConfigFieldMinBytes], 1 unless set.
	DefaultMinBytesConfigPath = "flam.broker.defaults.brokers.kafka.min_bytes"

	// DefaultMaxBytesConfigPath is the path of the default of
	// [ConfigFieldMaxBytes], 10485760 unless set.
	DefaultMaxBytesConfigPath = "flam.broker.defaults.brokers.kafka.max_bytes"

	// DefaultMaxWaitConfigPath is the path of the default of
	// [ConfigFieldMaxWait], "1s" unless set.
	DefaultMaxWaitConfigPath = "flam.broker.defaults.brokers.kafka.max_wait"

	// DefaultStartOffsetConfigPath is the path of the default of
	// [ConfigFieldStartOffset], [OffsetLast] unless set.
	DefaultStartOffsetConfigPath = "flam.broker.defaults.brokers.kafka.start_offset"

	// DefaultRetryBackoffConfigPath is the path of the default of
	// [ConfigFieldRetryBackoff], "1s" unless set.
	DefaultRetryBackoffConfigPath = "flam.broker.defaults.brokers.kafka.retry_backoff"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the broker.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"
)

// Defaults of the fields of a broker configuration.
const (
	defaultBroker       = "127.0.0.1:9092"
	defaultBalancer     = BalancerHash
	defaultRequiredAcks = AcksAll
	defaultBatchSize    = 100
	defaultBatchBytes   = 1 << 20
	defaultBatchTimeout = 10 * time.Millisecond
	defaultWriteTimeout = 10 * time.Second
	defaultMaxAttempts  = 10
	defaultMinBytes     = 1
	defaultMaxBytes     = 10 << 20
	defaultMaxWait      = time.Second
	defaultStartOffset  = OffsetLast
	defaultRetryBackoff = time.Second
)
