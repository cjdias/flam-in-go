package redis

import (
	"time"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.broker.brokers.redis"

// Driver is the driver name of the redis brokers, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.broker.brokers.driver.redis"

// Modes of a redis broker, held by its [ConfigFieldMode] field.
const (
	// ModePubSub publishes with PUBLISH and subscribes with SUBSCRIBE,
	// delivering each message at most once to every subscription in place.
	ModePubSub = "pubsub"

	// ModeStreams publishes with XADD on the stream of the topic and
	// subscribes with XREADGROUP in a consumer group, delivering each
	// message at least once to one consumer of each group.
	ModeStreams = "streams"
)

// Starts of the consumer group of a stream, held by the [ConfigFieldStart]
// field.
const (
	// StartNew makes a new consumer group read the entries added after its
	// creation.
	StartNew = "new"

	// StartAll makes a new consumer group read every entry of its stream.
	StartAll = "all"
)

// Fields of a redis broker configuration.
const (
	// ConfigFieldConnection is the field holding the id of the redis
	// connection of the broker, in the
	// [github.com/cjdias/flam-in-go/pkg/flam.IRedisConnectionFactory].
	ConfigFieldConnection = "connection"

	// ConfigFieldMode is the field holding the mode of the broker,
	// [ModePubSub] or [ModeStreams].
	ConfigFieldMode = "mode"

	// ConfigFieldGroup is the field holding the consumer group of the
	// subscriptions of the streams mode, the broker id when empty.
	ConfigFieldGroup = "group"

	// ConfigFieldConsumer is the field holding the name of the consumer of
	// the subscriptions in their group, in the streams mode, unique to
	// each subscription when empty, such a consumer being deleted from its
	// group when its subscription ends, unless it holds pending entries.
	ConfigFieldConsumer = "consumer"

	// ConfigFieldStart is the field holding the start of a consumer group
	// created by a subscription, [StartNew] or [StartAll].
	ConfigFieldStart = "start"

	// ConfigFieldBatch is the field holding the maximum number of entries
	// a subscription reads at once, a whole number, 1 or more.
	ConfigFieldBatch = "batch"

	// ConfigFieldBlock is the field holding the time a read of a
	// subscription waits for new entries, a positive duration such as
	// "1s", bounding the time closing it takes.
	ConfigFieldBlock = "block"

	// ConfigFieldClaimIdle is the field holding the time after which an
	// entry delivered and not acknowledged, as its handler failed or its
	// consumer stopped, is claimed by a subscription of its group to be
	// delivered again, a duration such as "30s", 0 claiming none, so that
	// such entries are never delivered again by the broker.
	ConfigFieldClaimIdle = "claim_idle"

	// ConfigFieldMaxLen is the field holding the length the streams are
	// trimmed to, approximately, when publishing, a whole number, 0 for no
	// trimming.
	ConfigFieldMaxLen = "max_len"
)

// Configuration paths of the values taken by the fields missing from a
// broker configuration.
const (
	// DefaultConnectionConfigPath is the path of the default of
	// [ConfigFieldConnection], "default" unless set.
	DefaultConnectionConfigPath = "flam.broker.defaults.brokers.redis.connection"

	// DefaultModeConfigPath is the path of the default of
	// [ConfigFieldMode], [ModePubSub] unless set.
	DefaultModeConfigPath = "flam.broker.defaults.brokers.redis.mode"

	// DefaultStartConfigPath is the path of the default of
	// [ConfigFieldStart], [StartNew] unless set.
	DefaultStartConfigPath = "flam.broker.defaults.brokers.redis.start"

	// DefaultBatchConfigPath is the path of the default of
	// [ConfigFieldBatch], 10 unless set.
	DefaultBatchConfigPath = "flam.broker.defaults.brokers.redis.batch"

	// DefaultBlockConfigPath is the path of the default of
	// [ConfigFieldBlock], "1s" unless set.
	DefaultBlockConfigPath = "flam.broker.defaults.brokers.redis.block"

	// DefaultClaimIdleConfigPath is the path of the default of
	// [ConfigFieldClaimIdle], "30s" unless set.
	DefaultClaimIdleConfigPath = "flam.broker.defaults.brokers.redis.claim_idle"

	// DefaultMaxLenConfigPath is the path of the default of
	// [ConfigFieldMaxLen], 0 unless set.
	DefaultMaxLenConfigPath = "flam.broker.defaults.brokers.redis.max_len"
)

// Fields of the entries a broker adds to a stream.
const (
	// StreamFieldPayload is the field of an entry holding the payload.
	StreamFieldPayload = "payload"

	// StreamFieldHeaderPrefix is the prefix of the fields of an entry
	// holding the headers, followed by the header name.
	StreamFieldHeaderPrefix = "header."
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the broker.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldConnection holds the id of the redis connection.
	ErrorFieldConnection = "connection"
)

// Defaults of the fields of a broker configuration.
const (
	defaultConnection = "default"
	defaultMode       = ModePubSub
	defaultStart      = StartNew
	defaultBatch      = 10
	defaultBlock      = time.Second
	defaultClaimIdle  = 30 * time.Second
	defaultMaxLen     = 0
)
